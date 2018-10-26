package ably

import (
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

// QueryFunc queries the given URL and gives non-nil HTTP response if no error
// occurred.
type QueryFunc func(url string) (*http.Response, error)

// PaginatedResult represents a single page coming back from the REST API.
// Any call to create a new page will generate a new instance.
type PaginatedResult struct {
	path     string
	headers  map[string]string
	links    []string
	items    []interface{}
	typItems interface{}
	typ      reflect.Type
	query    QueryFunc
	logger   *LoggerOptions
	opts     *proto.ChannelOptions

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
	query     QueryFunc
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
		typ:    req.typ,
		query:  req.query,
		logger: req.logger,
		opts:   opts,
	}
	builtPath, err := p.buildPaginatedPath(req.path, req.params)
	if err != nil {
		return nil, err
	}
	resp, err := p.query(builtPath)
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
	if h := p.respHeaders.Get(AblyErrorCodeHeader); h != "" {
		i, err := strconv.Atoi(h)
		if err != nil {
			return nil, err
		}
		p.errorCode = i
	}
	if h := p.respHeaders.Get(AblyErrormessageHeader); h != "" {
		p.errorMessage = h
	}
	p.path = builtPath
	p.links = resp.Header["Link"]
	if p.errorCode != 0 {
		var o map[string]interface{}
		err = decodeResp(resp, &o)
		if err != nil {
			return nil, err
		}
		p.typItems = []interface{}{o}
		return p, nil
	}
	v, err := req.decoder(opts, p.typ, resp)
	if err != nil {
		return nil, err
	}
	p.typItems = v
	return p, nil
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
func (p *PaginatedResult) Next() (*PaginatedResult, error) {
	nextPath, ok := p.paginationHeaders()["next"]
	if !ok {
		return nil, newErrorf(ErrProtocolError, "no next page after %q", p.path)
	}
	nextPage := p.buildPath(p.path, nextPath)
	return newPaginatedResult(p.opts, paginatedRequest{typ: p.typ, path: nextPage,
		query: p.query, logger: p.logger, respCheck: checkValidHTTPResponse})
}

// Items gives a slice of results of the current page.
func (p *PaginatedResult) Items() []interface{} {
	if p.items == nil {
		v := reflect.ValueOf(p.typItems)
		p.items = make([]interface{}, v.Len())
		for i := range p.items {
			p.items[i] = v.Index(i).Interface()
		}
	}
	return p.items
}

// Messages gives a slice of messages for the current page. The method panics if
// the underlying paginated result is not a message.
func (p *PaginatedResult) Messages() []*proto.Message {
	items, ok := p.typItems.([]*proto.Message)
	if !ok {
		panic(errInvalidType{typ: p.typ})
	}
	return items
}

// PresenceMessages gives a slice of presence messages for the current path.
// The method panics if the underlying paginated result is not a presence message.
func (p *PaginatedResult) PresenceMessages() []*proto.PresenceMessage {
	items, ok := p.typItems.([]*proto.PresenceMessage)
	if !ok {
		panic(errInvalidType{typ: p.typ})
	}
	return items
}

// Stats gives a slice of statistics for the current page. The method panics if
// the underlying paginated result is not statistics.
func (p *PaginatedResult) Stats() []*proto.Stats {
	items, ok := p.typItems.([]*proto.Stats)
	if !ok {
		panic(errInvalidType{typ: p.typ})
	}
	return items
}

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
