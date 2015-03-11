package protocol

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/ably/ably-go/config"
)

// relLink is the regexp that matches our pagination format
const relLink = `<(?P<url>[^>]+)>; rel="(?P<rel>[^"]+)"`

var relLinkRegexp = regexp.MustCompile(relLink)

// ResourceReader wraps around
type ResourceReader interface {
	Get(path string, out interface{}) (*http.Response, error)
}

// PaginatedResource represents a single page coming back from the REST API.
// This resource never changes (even if technically it has mutable fields). Any
// call to create a new page will generate a new instance.
type PaginatedResource struct {
	Response       *http.Response
	Path           string
	ResourceReader ResourceReader
	Body           []byte

	paginationHeaders map[string]string
}

// NewPaginatedResource returns a new instance of PaginatedResource
// It needs to be a struct implementing ResourceReader which in our case is rest.Client.
func NewPaginatedResource(rr ResourceReader, path string, params *config.PaginateParams) (*PaginatedResource, error) {
	p := &PaginatedResource{
		ResourceReader: rr,
	}

	builtPath, err := p.BuildPaginatedPath(path, params)
	if err != nil {
		return nil, err
	}

	resp, err := p.ResourceReader.Get(builtPath, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	p.Response = resp
	p.Path = builtPath
	p.Body = body

	return p, nil
}

func (c *PaginatedResource) BuildPaginatedPath(path string, params *config.PaginateParams) (string, error) {
	if params == nil {
		return path, nil
	}

	values := &url.Values{}
	err := params.EncodeValues(values)
	if err != nil {
		return "", err
	}

	queryString := values.Encode()
	if len(queryString) > 0 {
		path = path + "?" + queryString
	}

	return path, nil
}

// NextPagePath returns the path to the next page as found in the response headers.
// The response headers from the REST API contains a relative link to the next resource
// (Link: <./path>; rel="next").
func (p *PaginatedResource) NextPagePath() (string, error) {
	if p.getPaginationHeaders()["next"] == "" {
		return "", fmt.Errorf("No next page after %v", p.Path)
	}

	nextPage, err := p.BuildPath(p.Path, p.getPaginationHeaders()["next"])
	if err != nil {
		return "", err
	}

	return nextPage, nil
}

// BuildPath finds the absolute path based on the path parameter and the new relative path.
func (p *PaginatedResource) BuildPath(path string, newRelativePath string) (string, error) {
	oldURL, err := url.Parse("http://example.com" + path)
	if err != nil {
		return "", err
	}

	rootPath := strings.TrimRightFunc(oldURL.Path, func(r rune) bool {
		return r != '/'
	})

	newPath := rootPath + strings.TrimLeft(newRelativePath, "./")

	return newPath, nil
}

func (p *PaginatedResource) getPaginationHeaders() map[string]string {
	if len(p.paginationHeaders) > 0 {
		return p.paginationHeaders
	}

	links := p.Response.Header["Link"]
	p.paginationHeaders = map[string]string{}

	for i := range links {
		if result := relLinkRegexp.FindStringSubmatch(links[i]); result != nil {
			p.addMatch(result)
		}
	}

	return p.paginationHeaders
}

func (p *PaginatedResource) addMatch(matches []string) {
	matchingNames := relLinkRegexp.SubexpNames()
	matchMap := map[string]string{}
	for i, value := range matches {
		matchMap[matchingNames[i]] = value
	}

	p.paginationHeaders[matchMap["rel"]] = matchMap["url"]
}
