package ablyutil

import (
	"math/rand"
	"sort"
	"strings"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

func GenerateRandomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

type HashSet map[string]struct{} // struct {} has zero space complexity

func NewHashSet() HashSet {
	return make(HashSet)
}

func (s HashSet) Add(item string) {
	s[item] = struct{}{}
}

func (s HashSet) Remove(item string) {
	delete(s, item)
}

func (s HashSet) Has(item string) bool {
	_, ok := s[item]
	return ok
}

func Copy(list []string) []string {
	copiedList := make([]string, len(list))
	copy(copiedList, list)
	return copiedList
}

func Sort(list []string) []string {
	copiedList := Copy(list)
	sort.Strings(copiedList)
	return copiedList
}

func Shuffle(list []string) []string {
	copiedList := Copy(list)
	if len(copiedList) <= 1 {
		return copiedList
	}
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(copiedList), func(i, j int) { copiedList[i], copiedList[j] = copiedList[j], copiedList[i] })
	return copiedList
}

func SliceContains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

func Empty(s string) bool {
	return len(strings.TrimSpace(s)) == 0
}
