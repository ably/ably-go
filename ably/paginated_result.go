package ably

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strings"

	"github.com/ably/ably-go/ably/proto"

	"github.com/ably/ably-go/Godeps/_workspace/src/gopkg.in/vmihailenco/msgpack.v2"
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
}

func newPaginatedResult(typ reflect.Type, path string, params *PaginateParams,
	query QueryFunc) (*PaginatedResult, error) {
	p := &PaginatedResult{
		typ:   typ,
		query: query,
	}
	builtPath, err := p.buildPaginatedPath(path, params)
	if err != nil {
		return nil, err
	}
	resp, err := p.query(builtPath)
	if err != nil {
		return nil, err
	}
	if err = checkValidHTTPResponse(resp); err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	p.path = builtPath
	p.links = resp.Header["Link"]
	v := reflect.New(p.typ)
	switch typ := strip(resp.Header.Get("Content-Type"), ';'); typ {
	case "", "application/x-msgpack":
		err = msgpack.NewDecoder(resp.Body).Decode(v.Interface())
	case "application/json":
		err = json.NewDecoder(resp.Body).Decode(v.Interface())
	default:
		err = newError(40000, errors.New("unrecognized Content-Type: "+typ))
	}
	if err != nil {
		return nil, err
	}
	p.typItems = v.Elem().Interface()
	return p, nil
}

// Next returns the path to the next page as found in the response headers.
// The response headers from the REST API contains a relative link to the next result.
// (Link: <./path>; rel="next").
func (p *PaginatedResult) Next() (*PaginatedResult, error) {
	nextPath, ok := p.paginationHeaders()["next"]
	if !ok {
		return nil, newErrorf(ErrCodeNotFound, "no next page after %q", p.path)
	}
	nextPage, err := p.buildPath(p.path, nextPath)
	if err != nil {
		return nil, err
	}
	return newPaginatedResult(p.typ, nextPage, nil, p.query)
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
func (p *PaginatedResult) Stats() []*proto.Stat {
	items, ok := p.typItems.([]*proto.Stat)
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
func (p *PaginatedResult) buildPath(path string, newRelativePath string) (string, error) {
	oldURL, err := url.Parse("http://example.com" + path)
	if err != nil {
		return "", newError(50000, err)
	}
	rootPath := strings.TrimRightFunc(oldURL.Path, func(r rune) bool { return r != '/' })
	return rootPath + strings.TrimLeft(newRelativePath, "./"), nil
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
