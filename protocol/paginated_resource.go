package protocol

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type PaginatedResource struct {
	Response          *http.Response
	Path              string
	paginationHeaders map[string]string
}

const relLink = `<(?P<url>[^>]+)>; rel="(?P<rel>[^"]+)"`

var relLinkRegexp = regexp.MustCompile(relLink)

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
