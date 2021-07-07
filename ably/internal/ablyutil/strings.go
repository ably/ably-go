package ablyutil

import (
	"math/rand"
	"strings"
	"time"
)

func Shuffle(list []string) []string {
	strListLen := len(list)
	var shuffledList []string
	shuffledList = append(shuffledList, list...)
	if strListLen == 0 || strListLen == 1 {
		return list
	}
	src := rand.NewSource(time.Now().UnixNano())
	r := rand.New(src)
	for i := range shuffledList {
		n := r.Intn(strListLen - 1)
		shuffledList[i], shuffledList[n] = shuffledList[n], shuffledList[i]
	}
	return shuffledList
}

func Contains(s []string, str string) bool {
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
