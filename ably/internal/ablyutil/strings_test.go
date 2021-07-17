package ablyutil_test

import (
	"reflect"
	"testing"

	"github.com/ably/ably-go/ably/internal/ablyutil"
)

func Test_string(t *testing.T) {
	t.Run("String array Shuffle", func(t *testing.T) {
		t.Parallel()

		strList := []string{}
		shuffledList := ablyutil.Shuffle(strList)
		areEqual := reflect.DeepEqual(strList, shuffledList)
		if !areEqual {
			t.Errorf("%v should be equal to %v", strList, shuffledList)
		}

		strList = []string{"a"}
		shuffledList = ablyutil.Shuffle(strList)
		areEqual = reflect.DeepEqual(strList, shuffledList)
		if !areEqual {
			t.Errorf("%v should be equal to %v", strList, shuffledList)
		}

		strList = []string{"a", "b", "c"}
		shuffledList = ablyutil.Shuffle(strList)
		areEqual = reflect.DeepEqual(strList, shuffledList)
		if areEqual {
			t.Errorf("%v is equal to %v", strList, shuffledList)
		}

		areEqual = reflect.DeepEqual(ablyutil.Sort(strList), ablyutil.Sort(shuffledList))
		if !areEqual {
			t.Errorf("%v should be equal to %v", strList, shuffledList)
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

func TestHashSet(t *testing.T) {
	t.Run("Add should not duplicate entries", func(t *testing.T) {
		hashSet := ablyutil.NewHashSet()
		hashSet.Add("apple")
		hashSet.Add("apple")
		if len(hashSet) != 1 {
			t.Fatalf("hashset length should be equal to 1")
		}
		hashSet.Add("banana")
		if len(hashSet) != 2 {
			t.Fatalf("hashset length should be equal to 2")
		}
		hashSet.Add("orange")
		if len(hashSet) != 3 {
			t.Fatalf("hashset length should be equal to 3")
		}
		hashSet.Add("banana")
		hashSet.Add("apple")
		hashSet.Add("orange")
		hashSet.Add("orange")

		if len(hashSet) != 3 {
			t.Fatalf("hashset length should be equal to 3")
		}
	})

	t.Run("Should check if item is present", func(t *testing.T) {
		hashSet := ablyutil.NewHashSet()
		hashSet.Add("apple")
		hashSet.Add("orange")
		if !hashSet.Has("apple") {
			t.Fatalf("Set should contain apple")
		}
		if hashSet.Has("banana") {
			t.Fatalf("Set shouldm't contain banana")
		}
		if !hashSet.Has("orange") {
			t.Fatalf("Set should contain orange")
		}
	})

	t.Run("Should remove element", func(t *testing.T) {
		hashSet := ablyutil.NewHashSet()
		hashSet.Add("apple")
		if len(hashSet) != 1 {
			t.Fatalf("hashset length should be equal to 1")
		}
		hashSet.Add("orange")
		if len(hashSet) != 2 {
			t.Fatalf("hashset length should be equal to 2")
		}
		hashSet.Remove("apple")
		if len(hashSet) != 1 {
			t.Fatalf("hashset length should be equal to 1")
		}
		if hashSet.Has("apple") {
			t.Fatalf("Set shouldm't contain apple")
		}
		hashSet.Remove("orange")
		if len(hashSet) != 0 {
			t.Fatalf("hashset length should be equal to 0")
		}
	})
}
