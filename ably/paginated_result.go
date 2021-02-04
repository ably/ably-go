package ably

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/ably/ably-go/ably/proto"
)

// relLinkRegexp is the regexp that matches our pagination format
var relLinkRegexp = regexp.MustCompile(`<(?P<url>[^>]+)>; rel="(?P<rel>[^"]+)"`)

type errInvalidType struct {
	typ reflect.Type
}

func (err errInvalidType) Error() string {
	return "requested value of incompatible type: " + err.typ.String()
}

// queryFunc queries the given URL and gives non-nil HTTP response if no error
// occurred.
type queryFunc func(url string) (*http.Response, error)

// PaginatedResult represents a single page coming back from the REST API.
// Any call to create a new page will generate a new instance.
type PaginatedResult struct {
	path     string
	headers  map[string]string
	links    []string
	items    []interface{}
	typItems interface{}
	opts     *proto.ChannelOptions
	req      paginatedRequest

	statusCode   int
	success      bool
	errorCode    int
	errorMessage string
	respHeaders  http.Header
}

type paginatedRequest struct {
	typ       reflect.Type
	path      string
	params    *PaginateParams
	query     queryFunc
	logger    *LoggerOptions
	respCheck func(*http.Response) error
	decoder   func(*proto.ChannelOptions, reflect.Type, *http.Response) (interface{}, error)
}

func decodePaginatedResult(opts *proto.ChannelOptions, typ reflect.Type, resp *http.Response) (interface{}, error) {
	switch typ {
	case msgType:
		var o []map[string]interface{}
		err := decodeResp(resp, &o)
		if err != nil {
			return nil, err
		}
		rs := make([]*proto.Message, len(o))
		for k, v := range o {
			m := &proto.Message{
				ChannelOptions: opts,
			}
			err = m.FromMap(v)
			if err != nil {
				return nil, err
			}
			rs[k] = m
		}
		return rs, nil
	case presMsgType:
		var o []map[string]interface{}
		err := decodeResp(resp, &o)
		if err != nil {
			return nil, err
		}
		rs := make([]*proto.PresenceMessage, len(o))
		for k, v := range o {
			m := &proto.PresenceMessage{
				Message: proto.Message{
					ChannelOptions: opts,
				},
			}
			err = m.FromMap(v)
			if err != nil {
				return nil, err
			}
			rs[k] = m
		}
		return rs, nil
	default:
		v := reflect.New(typ)
		if err := decodeResp(resp, v.Interface()); err != nil {
			return nil, err
		}
		return v.Elem().Interface(), nil
	}
}
func newPaginatedResult(opts *proto.ChannelOptions, req paginatedRequest) (*PaginatedResult, error) {
	if req.decoder == nil {
		req.decoder = decodePaginatedResult
	}
	p := &PaginatedResult{
		opts: opts,
		req:  req,
	}
	builtPath, err := p.buildPaginatedPath(req.path, req.params)
	if err != nil {
		return nil, err
	}
	resp, err := p.req.query(builtPath)
	if err != nil {
		return nil, err
	}
	if err = req.respCheck(resp); err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if p.respHeaders == nil {
		p.respHeaders = make(http.Header)
	}
	p.statusCode = resp.StatusCode
	p.success = 200 <= p.statusCode && p.statusCode < 300
	copyHeader(p.respHeaders, resp.Header)
	if h := p.respHeaders.Get(proto.AblyErrorCodeHeader); h != "" {
		i, err := strconv.Atoi(h)
		if err != nil {
			return nil, err
		}
		p.errorCode = i
	} else if !p.success {
		return nil, malformedPaginatedResponseError(resp)
	}
	if h := p.respHeaders.Get(proto.AblyErrormessageHeader); h != "" {
		p.errorMessage = h
	} else if !p.success {
		return nil, malformedPaginatedResponseError(resp)
	}
	p.path = builtPath
	p.links = resp.Header["Link"]
	v, err := p.req.decoder(opts, p.req.typ, resp)
	if err != nil {
		return nil, err
	}
	p.typItems = v
	return p, nil
}

func malformedPaginatedResponseError(resp *http.Response) error {
	body := make([]byte, 200)
	n, err := io.ReadFull(resp.Body, body)
	body = body[:n]
	msg := fmt.Sprintf("invalid PaginatedResult HTTP response; status: %d; body (first %d bytes): %q", resp.StatusCode, len(body), body)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s; body read error: %w", msg, err)
	}
	return errors.New(msg)
}

func copyHeader(dest, src http.Header) {
	for k, v := range src {
		d := make([]string, len(v))
		copy(d, v)
		dest[k] = v
	}
}

// Next returns the path to the next page as found in the response headers.
// The response headers from the REST API contains a relative link to the next result.
// (Link: <./path>; rel="next").
//
// If there is no next link, both return values are nil.
func (p *PaginatedResult) Next() (*PaginatedResult, error) {
	nextPath, ok := p.paginationHeaders()["next"]
	if !ok {
		return nil, nil
	}
	nextPage := p.buildPath(p.path, nextPath)
	req := p.req
	req.path = nextPage
	req.params = nil
	return newPaginatedResult(p.opts, req)
}

// Items gives a slice of results of the current page.
func (p *PaginatedResult) Items() []interface{} {
	if p.items == nil {
		v := reflect.ValueOf(p.typItems)
		if v.Kind() == reflect.Slice {
			p.items = make([]interface{}, v.Len())
			for i := range p.items {
				p.items[i] = v.Index(i).Interface()
			}
		} else {
			p.items = []interface{}{p.typItems}
		}
	}
	return p.items
}

// Messages gives a slice of messages for the current page. The method panics if
// the underlying paginated result is not a message.
func (p *PaginatedResult) Messages() []*Message {
	items, ok := p.typItems.([]*proto.Message)
	if !ok {
		panic(errInvalidType{typ: p.req.typ})
	}
	return items
}

// PresenceMessages gives a slice of presence messages for the current path.
// The method panics if the underlying paginated result is not a presence message.
func (p *PaginatedResult) PresenceMessages() []*PresenceMessage {
	items, ok := p.typItems.([]*proto.PresenceMessage)
	if !ok {
		panic(errInvalidType{typ: p.req.typ})
	}
	return items
}

// Stats gives a slice of statistics for the current page. The method panics if
// the underlying paginated result is not statistics.
func (p *PaginatedResult) Stats() []*Stats {
	items, ok := p.typItems.([]*proto.Stats)
	if !ok {
		panic(errInvalidType{typ: p.req.typ})
	}
	return items
}

type Stats = proto.Stats
type StatsMessageTypes = proto.MessageTypes
type StatsMessageTraffic = proto.MessageTraffic
type StatsConnectionTypes = proto.ConnectionTypes
type StatsResourceCount = proto.ResourceCount
type StatsRequestCount = proto.RequestCount
type StatsPushStats = proto.PushStats
type StatsXchgMessages = proto.XchgMessages
type PushStats = proto.PushStats

func (c *PaginatedResult) buildPaginatedPath(path string, params *PaginateParams) (string, error) {
	if params == nil {
		return path, nil
	}
	values := &url.Values{}
	err := params.EncodeValues(values)
	if err != nil {
		return "", newError(50000, err)
	}
	queryString := values.Encode()
	if len(queryString) > 0 {
		return path + "?" + queryString, nil
	}
	return path, nil
}

// buildPath finds the absolute path based on the path parameter and the new relative path.
func (p *PaginatedResult) buildPath(origPath string, newRelativePath string) string {
	if i := strings.IndexRune(origPath, '?'); i != -1 {
		origPath = origPath[:i]
	}
	return path.Join(path.Dir(origPath), newRelativePath)
}

func (p *PaginatedResult) paginationHeaders() map[string]string {
	if p.headers == nil {
		p.headers = make(map[string]string)
		for _, link := range p.links {
			if result := relLinkRegexp.FindStringSubmatch(link); result != nil {
				p.addMatch(result)
			}
		}
	}
	return p.headers
}

func (p *PaginatedResult) addMatch(matches []string) {
	matchingNames := relLinkRegexp.SubexpNames()
	matchMap := map[string]string{}
	for i, value := range matches {
		matchMap[matchingNames[i]] = value
	}
	p.headers[matchMap["rel"]] = matchMap["url"]
}
