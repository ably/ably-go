package ablytest

import (
	"context"
	"fmt"
	"reflect"
)

func TestPagination(expected, request interface{}, perPage int) error {
	var items []interface{}
	rexpected := reflect.ValueOf(expected)
	for i := 0; i < reflect.ValueOf(expected).Len(); i++ {
		items = append(items, rexpected.Index(i).Interface())
	}
	return testPagination(reflect.ValueOf(request), items, perPage)
}

func testPagination(request reflect.Value, expectedItems []interface{}, perPage int) error {
	getPages, getItems := generalizePagination(request)

	var expectedPages [][]interface{}
	var page []interface{}
	for _, item := range expectedItems {
		page = append(page, item)
		if len(page) == perPage {
			expectedPages = append(expectedPages, page)
			page = nil
		}
	}
	if len(page) > 0 {
		expectedPages = append(expectedPages, page)
	}

	for i := 0; i < 2; i++ {
		pages, err := getPages()
		if err != nil {
			return fmt.Errorf("calling Pages: %w", err)
		}
		var gotPages [][]interface{}
		for pages.next() {
			gotPages = append(gotPages, pages.items())
		}
		if err := pages.err(); err != nil {
			return fmt.Errorf("iterating pages: %w", err)
		}

		if !reflect.DeepEqual(expectedPages, gotPages) {
			return fmt.Errorf("expected pages: %+v, got: %+v", expectedPages, gotPages)
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

		if !reflect.DeepEqual(expectedItems, gotItems) {
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
