package ablytest

import (
	"context"
	"fmt"
	"reflect"
)

// AllPages appends all items from all pages resulting from a paginated
// request into the slice pointed to by dst, which must be a pointer to a slice
// of the same type as the paginated response's items.
func AllPages(dst, paginatedRequest interface{}) error {
	_, getItems := generalizePagination(reflect.ValueOf(paginatedRequest))
	items, err := getItems()
	if err != nil {
		return err
	}
	all := reflect.ValueOf(dst).Elem()
	for items.next() {
		all.Set(reflect.Append(all, reflect.ValueOf(items.item())))
	}
	return items.err()
}

type paginationOptions struct {
	equal      func(x, y interface{}) bool
	sortResult func([]interface{})
}

type PaginationOption func(*paginationOptions)

func PaginationWithEqual(equal func(x, y interface{}) bool) PaginationOption {
	return func(o *paginationOptions) {
		o.equal = equal
	}
}

func PaginationWithSortResult(sort func([]interface{})) PaginationOption {
	return func(o *paginationOptions) {
		o.sortResult = sort
	}
}

func TestPagination(expected, request interface{}, perPage int, options ...PaginationOption) error {
	opts := paginationOptions{
		equal:      reflect.DeepEqual,
		sortResult: func(items []interface{}) {},
	}
	for _, o := range options {
		o(&opts)
	}

	var items []interface{}
	rexpected := reflect.ValueOf(expected)
	for i := 0; i < reflect.ValueOf(expected).Len(); i++ {
		items = append(items, rexpected.Index(i).Interface())
	}
	return testPagination(reflect.ValueOf(request), items, perPage, opts)
}

func testPagination(request reflect.Value, expectedItems []interface{}, perPage int, opts paginationOptions) error {
	getPages, getItems := generalizePagination(request)

	for i := 0; i < 2; i++ {
		pages, err := getPages()
		if err != nil {
			return fmt.Errorf("calling Pages: %w", err)
		}
		var gotItems []interface{}
		pageNum := 1
		expectedFullPages := len(expectedItems) / perPage
		for pages.next() {
			if (pageNum <= expectedFullPages && len(pages.items()) != perPage) ||
				(pageNum > expectedFullPages && len(pages.items()) >= perPage) {
				return fmt.Errorf("page #%d got %d items, expected at most %d", pageNum, len(pages.items()), perPage)
			}
			gotItems = append(gotItems, pages.items()...)
			pageNum++
		}
		if err := pages.err(); err != nil {
			return fmt.Errorf("iterating pages: %w", err)
		}

		opts.sortResult(gotItems)

		if !ItemsEqual(expectedItems, gotItems, opts.equal) {
			return fmt.Errorf("expected items: %+v, got: %+v", expectedItems, gotItems)
		}

		if err := pages.first(); err != nil {
			return fmt.Errorf("going back to first page: %w", err)
		}
	}

	for i := 0; i < 2; i++ {
		items, err := getItems()
		if err != nil {
			return fmt.Errorf("calling Items: %w", err)
		}
		var gotItems []interface{}
		for items.next() {
			gotItems = append(gotItems, items.item())
		}
		if err := items.err(); err != nil {
			return fmt.Errorf("iterating items: %w", err)
		}

		opts.sortResult(gotItems)

		if !ItemsEqual(expectedItems, gotItems, opts.equal) {
			return fmt.Errorf("expected items: %+v, got: %+v", expectedItems, gotItems)
		}

		if err := items.first(); err != nil {
			return fmt.Errorf("going back to first page: %w", err)
		}
	}

	return nil
}

type paginated struct {
	next  func() bool
	first func() error
	err   func() error
}

type paginatedResult struct {
	paginated
	items func() []interface{}
}

type paginatedItems struct {
	paginated
	item func() interface{}
}

func generalizePagination(request reflect.Value) (func() (paginatedResult, error), func() (paginatedItems, error)) {
	ctx := reflect.ValueOf(context.Background())

	generalizeCommon := func(r reflect.Value) paginated {
		return paginated{
			next: func() bool {
				return r.MethodByName("Next").Call([]reflect.Value{ctx})[0].Bool()
			},
			first: func() error {
				err, _ := r.MethodByName("First").Call([]reflect.Value{ctx})[0].Interface().(error)
				return err
			},
			err: func() error {
				err, _ := r.MethodByName("Err").Call(nil)[0].Interface().(error)
				return err
			},
		}
	}

	pages := func() (paginatedResult, error) {
		ret := request.MethodByName("Pages").Call([]reflect.Value{ctx})
		if err, ok := ret[1].Interface().(error); ok && err != nil {
			return paginatedResult{}, err
		}
		r := ret[0]
		return paginatedResult{
			paginated: generalizeCommon(r),
			items: func() []interface{} {
				ritems := r.MethodByName("Items").Call(nil)[0]
				var items []interface{}
				for i := 0; i < ritems.Len(); i++ {
					items = append(items, ritems.Index(i).Interface())
				}
				return items
			},
		}, nil
	}

	items := func() (paginatedItems, error) {
		ret := request.MethodByName("Items").Call([]reflect.Value{ctx})
		if err, ok := ret[1].Interface().(error); ok && err != nil {
			return paginatedItems{}, err
		}
		r := ret[0]
		return paginatedItems{
			paginated: generalizeCommon(r),
			item: func() interface{} {
				return r.MethodByName("Item").Call(nil)[0].Interface()
			},
		}, nil
	}

	return pages, items
}

func ItemsEqual(expected, got []interface{}, equal func(x, y interface{}) bool) bool {
	if len(expected) != len(got) {
		return false
	}
	for i := range expected {
		if !equal(expected[i], got[i]) {
			return false
		}
	}
	return true
}
