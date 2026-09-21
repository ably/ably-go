package ablyutil

import (
	"crypto/rand"
	"encoding/base64"
)

// BaseID returns a base64 encoded 9 random bytes to be used in idempotent rest
// publishing as part of message id.
//
// Spec RSL1k1
func BaseID() (string, error) {
	r := make([]byte, 9)
	_, err := rand.Read(r)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(r), nil
}
