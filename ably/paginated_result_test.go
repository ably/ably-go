package ably_test

import (
	"testing"

	"github.com/ably/ably-go/ably"
)

func TestPaginatedResult(t *testing.T) {
	t.Parallel()
	result := &ably.PaginatedResult{}
	newPath := result.BuildPath("/path/to/resource?hello", "./newresource?world")
	expected := "/path/to/newresource?world"
	if newPath != expected {
		t.Errorf("expected %s got %s", expected, newPath)
	}
}
