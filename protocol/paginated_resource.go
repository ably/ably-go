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

const relLink = `<(?P<url>[^>]+)>; rel="(?P<rel>[^"]+)"`

var relLinkRegexp = regexp.MustCompile(relLink)

type ResourceReader interface {
	Get(path string, out interface{}) (*http.Response, error)
}

// PaginatedResource represents a single page coming back from the REST API.
type PaginatedResource struct {
	Response          *http.Response
	Path              string
	paginationHeaders map[string]string

	ResourceReader ResourceReader
	Body           []byte
}

func (p *PaginatedResource) PaginatedGet(path string, params *config.PaginateParams) (*PaginatedResource, error) {
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

	paginatedResource := &PaginatedResource{
		Response:       resp,
		Path:           builtPath,
		ResourceReader: p.ResourceReader,
		Body:           body,
	}

	return paginatedResource, nil
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

// NextPage returns the path to the next page.
// This path is contained in the response headers from the REST API as a
// relative link (Link: <./path>; rel="next")
func (p *PaginatedResource) NextPage() (string, error) {
	if p.GetPaginationHeaders()["next"] == "" {
		return "", fmt.Errorf("No next page after %v", p.Path)
	}

	nextPage, err := p.BuildPath(p.Path, p.GetPaginationHeaders()["next"])
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

func (p *PaginatedResource) GetPaginationHeaders() map[string]string {
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
