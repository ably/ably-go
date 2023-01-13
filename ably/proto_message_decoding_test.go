package ably

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixture struct {
	Data          string `json:"data"`
	Encoding      string `json:"encoding"`
	ExpectedType  string `json:"expectedType"`
	ExpectedValue any    `json:"expectedValue"`
}

func loadFixtures() ([]fixture, error) {
	var dec struct {
		Messages []fixture
	}

	// We can't embed the fixtures as go:embed forbids embedding of resources above the current directory.
	text, err := os.ReadFile("../common/test-resources/messages-encoding.json")
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(text, &dec)
	return dec.Messages, err
}

func Test_decodeMessage(t *testing.T) {
	fixtures, err := loadFixtures()
	require.NoError(t, err, "failed to load test fixtures")
	for _, f := range fixtures {
		t.Run(f.Data, func(t *testing.T) {
			msg := Message{
				Data:     f.Data,
				Encoding: f.Encoding,
			}
			decodedMsg, err := msg.withDecodedData(nil)
			assert.NoError(t, err)
			switch f.ExpectedType {
			case "string":
				assert.IsType(t, "string", decodedMsg.Data)
				assert.Equal(t, f.ExpectedValue, decodedMsg.Data)
			case "jsonObject":
				assert.IsType(t, map[string]any{}, decodedMsg.Data)
				assert.Equal(t, f.ExpectedValue, decodedMsg.Data)

			case "jsonArray":
				assert.IsType(t, []any{}, decodedMsg.Data)
				assert.Equal(t, f.ExpectedValue, decodedMsg.Data)

				t.Log(decodedMsg)
			case "binary":
				assert.IsType(t, []byte{}, decodedMsg.Data)
				if f.ExpectedValue == nil {
					break
				}
				assert.Equal(t, []byte(f.ExpectedValue.(string)), decodedMsg.Data)
			}
		})
	}
}
