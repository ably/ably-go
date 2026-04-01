package ably

import (
	"context"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"regexp"
)

type Direction string

const (
	Backwards Direction = "backwards"
	Forwards  Direction = "forwards"
)

type paginatedRequest struct {
	path    string
	rawPath string
	params  url.Values

	query queryFunc
}

func (r *REST) newPaginatedRequest(path, rawPath string, params url.Values) paginatedRequest {
	return paginatedRequest{
		path:    path,
		rawPath: rawPath,
		params:  params,

		query: query(r.get),
	}
}

// PaginatedResult is a generic iterator for PaginatedResult pagination.
// Items decoding is delegated to type-specific wrappers.
//
// See "Paginated results" section in the package-level documentation.
type PaginatedResult struct {
	basePath  string
	nextLink  string
	firstLink string
	res       *http.Response
	err       error

	query queryFunc
	first bool
}

// load - It loads first page of results. Must be called from the type-specific
// wrapper Pages method that creates the PaginatedResult object.
func (p *PaginatedResult) load(ctx context.Context, r paginatedRequest) error {
	p.basePath = path.Dir(r.path)
	p.firstLink = (&url.URL{
		Path:     r.path,
		RawPath:  r.rawPath,
		RawQuery: r.params.Encode(),
	}).String()
	p.query = r.query
	return p.First(ctx)
}

// loadItems loads the first page of results and returns a next function. Must
// be called from the type-specific wrapper Items method that creates the
// PaginatedItems object.
//
// The returned next function must be called from the wrapper's Next method, and
// returns the index of the object that should be returned by the Item method,
// previously loading the next page if necessary.
//
// pageDecoder will be called each time a new page is retrieved under the hood.
// It should return a destination object on which the page of results will be
// decoded, and a pageLength function that, when called after the page has been
// decoded, must return the length of the page.
func (p *PaginatedResult) loadItems(
	ctx context.Context,
	r paginatedRequest,
	pageDecoder func() (page interface{}, pageLength func() int),
) (
	next func(context.Context) (int, bool),
	err error,
) {
	err = p.load(ctx, r)
	if err != nil {
		return nil, err
	}

	var page interface{}
	var pageLen int
	nextItem := 0

	return func(ctx context.Context) (int, bool) {
		if nextItem == 0 {
			var getLen func() int
			page, getLen = pageDecoder()
			hasNext := p.next(ctx, page)
			if !hasNext {
				return 0, false
			}
			pageLen = getLen()
			if pageLen == 0 { // compatible with hasNext if first page is empty
				return 0, false
			}
		}

		idx := nextItem
		nextItem = (nextItem + 1) % pageLen
		return idx, true
	}, nil
}

func (p *PaginatedResult) goTo(ctx context.Context, link string) error {
	var err error
	p.res, err = p.query(ctx, link)
	if err != nil {
		return err
	}
	p.nextLink = ""
	for _, rawLink := range p.res.Header["Link"] {
		m := relLinkRegexp.FindStringSubmatch(rawLink)
		if len(m) == 0 {
			continue
		}
		relPath, rel := m[1], m[2]
		linkPath := path.Join(p.basePath, relPath)
		switch rel {
		case "first":
			p.firstLink = linkPath
		case "next":
			p.nextLink = linkPath
		}
	}
	return nil
}

// next loads the next page of items, if there is one. It returns whether a page
// was successfully loaded or not; after it returns false, Err should be
// called to check for any errors.
//
// Items can then be inspected with the type-specific Items method.
//
// For items iterators, use the next function returned by loadItems instead.
func (p *PaginatedResult) next(ctx context.Context, into interface{}) bool {
	if !p.first {
		if p.nextLink == "" {
			return false
		}
		p.err = p.goTo(ctx, p.nextLink)
		if p.err != nil {
			return false
		}
	}
	p.first = false

	p.err = decodeResp(p.res, into)
	return p.err == nil
}

// First loads the first page of items. Next should be called before inspecting
// the Items.
func (p *PaginatedResult) First(ctx context.Context) error {
	p.first = true
	return p.goTo(ctx, p.firstLink)
}

// Err returns the error that caused Next to fail, if there was one.
func (p *PaginatedResult) Err() error {
	return p.err
}

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
type queryFunc func(ctx context.Context, url string) (*http.Response, error)

func copyHeader(dest, src http.Header) {
	for k, v := range src {
		d := make([]string, len(v))
		copy(d, v)
		dest[k] = v
	}
}
