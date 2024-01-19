package ablyutil_test

import (
	"testing"

	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/stretchr/testify/assert"
)

func Test_string(t *testing.T) {
	t.Run("String array Shuffle", func(t *testing.T) {
		t.Parallel()

		strList := []string{}
		shuffledList := ablyutil.Shuffle(strList)
		assert.Equal(t, strList, shuffledList)

		strList = []string{"a"}
		shuffledList = ablyutil.Shuffle(strList)
		assert.Equal(t, strList, shuffledList)

		strList = []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p"}
		shuffledList = ablyutil.Shuffle(strList)
		assert.NotEqual(t, strList, shuffledList)
		assert.Equal(t, ablyutil.Sort(strList), ablyutil.Sort(shuffledList))
	})

	t.Run("String array contains", func(t *testing.T) {
		t.Parallel()
		strarr := []string{"apple", "banana", "dragonfruit"}

		if !ablyutil.SliceContains(strarr, "apple") {
			t.Error("String array should contain apple")
		}
		if ablyutil.SliceContains(strarr, "orange") {
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
		assert.Len(t, hashSet, 1)

		hashSet.Add("banana")
		assert.Len(t, hashSet, 2)

		hashSet.Add("orange")
		assert.Len(t, hashSet, 3)

		hashSet.Add("banana")
		hashSet.Add("apple")
		hashSet.Add("orange")
		hashSet.Add("orange")

		assert.Len(t, hashSet, 3)
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
		assert.Len(t, hashSet, 1)

		hashSet.Add("orange")
		assert.Len(t, hashSet, 2)

		hashSet.Remove("apple")
		assert.Len(t, hashSet, 1)

		if hashSet.Has("apple") {
			t.Fatalf("Set shouldm't contain apple")
		}
		hashSet.Remove("orange")
		assert.Len(t, hashSet, 0)

	})
}
