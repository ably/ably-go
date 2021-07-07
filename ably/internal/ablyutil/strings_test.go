package ablyutil_test

import (
	"reflect"
	"testing"

	"github.com/ably/ably-go/ably/internal/ablyutil"
)

func Test_string(t *testing.T) {
	t.Run("String array Shuffle", func(t *testing.T) {
		t.Parallel()
		strList := []string{"a", "b", "c"}
		shuffledList := ablyutil.Shuffle(strList)
		areEqual := reflect.DeepEqual(strList, shuffledList)
		if areEqual {
			t.Errorf("%v is equal to %v", strList, shuffledList)
		}
	})

	t.Run("String array contains", func(t *testing.T) {
		t.Parallel()
		strarr := []string{"apple", "banana", "dragonfruit"}
		if !ablyutil.Contains(strarr, "apple") {
			t.Error("String array should contain apple")
		}
		if ablyutil.Contains(strarr, "orange") {
			t.Error("String array should not contain orange")
		}
	})

	t.Run("Empty String", func(t *testing.T) {
		t.Parallel()
		str := ""
		if !ablyutil.Empty(str) {
			t.Error("String should be empty")
		}
		str = " "
		if !ablyutil.Empty(str) {
			t.Error("String should be empty")
		}
		str = "ab"
		if ablyutil.Empty(str) {
			t.Error("String should not be empty")
		}
	})
}
