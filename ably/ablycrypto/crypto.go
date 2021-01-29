package ablycrypto

import (
	"github.com/ably/ably-go/ably/proto"
)

// GenerateRandomKey returns a random key. keyLength is optional; if non-zero,
// it should be in bits.
func GenerateRandomKey(keyLength int) ([]byte, error) {
	var lens []int
	if keyLength != 0 {
		lens = append(lens, keyLength)
	}
	return proto.GenerateRandomKey(lens...)
}
