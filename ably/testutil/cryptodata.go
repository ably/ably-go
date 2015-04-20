package testutil

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/ably/ably-go/ably/proto"
)

type CryptoData struct {
	Algorithm string `json:"algorithm"`
	Mode      string `json:"mode"`
	KeyLen    int    `json:"keylength"`
	Key       string `json:"key"`
	IV        string `json:"iv"`
	Items     []struct {
		Encoded   proto.Message `json:"encoded"`
		Encrypted proto.Message `json:"encrypted"`
	} `json:"items"`
}

func LoadCryptoData(rel string) (*CryptoData, []byte, []byte, error) {
	data := &CryptoData{}
	f, err := os.Open(filepath.Join("..", "..", "common", filepath.FromSlash(rel)))
	if err != nil {
		return nil, nil, nil, errors.New("missing common subrepo - ensure git submodules are initialized")
	}
	err = json.NewDecoder(f).Decode(data)
	f.Close()
	if err != nil {
		return nil, nil, nil, errors.New("unable to unmarshal test cases: " + err.Error())
	}
	key, err := base64.StdEncoding.DecodeString(data.Key)
	if err != nil {
		return nil, nil, nil, errors.New("unable to unbase64 key" + err.Error())
	}
	iv, err := base64.StdEncoding.DecodeString(data.IV)
	if err != nil {
		return nil, nil, nil, errors.New("unable to unbase64 IV" + err.Error())
	}
	return data, key, iv, nil
}
