//go:build msgpack_artifact_gen

// A utility that reads testdata/msgpack_test_fixtures.json, updates the msgpack representation,
// and writes the output to testdata/msgpack_test_fixtures.json.new.

package ably

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/stretchr/testify/require"
	"os"
	"strings"
	"testing"
)

func TestGenMsgpackFixture(t *testing.T) {
	msgpackTestFixtures, err := loadMsgpackFixtures()
	require.NoError(t, err)
	out := make([]MsgpackTestFixture, 0, len(msgpackTestFixtures))
	for i, f := range msgpackTestFixtures {
		msg := new(Message)
		switch f.Type {
		case "string":
			msg.Data = strings.Repeat(f.Data.(string), f.NumRepeat)
		case "binary":
			msg.Data = bytes.Repeat([]byte(f.Data.(string)), f.NumRepeat)
		case "jsonObject", "jsonArray":
			msg.Data = f.Data
		default:
			panic("unkown message type: " + f.Type)
		}
		msg.Encoding = f.Encoding
		encodedMsg, err := msg.withEncodedData(nil)
		require.NoError(t, err)
		pm := ProtocolMessage{
			Messages: []*Message{&encodedMsg},
		}
		buf, err := ablyutil.MarshalMsgpack(pm)
		require.NoError(t, err)
		msgpackTestFixtures[i].MsgPack = base64.StdEncoding.EncodeToString(buf)
		msgpackTestFixtures[i].Encoding = msg.Encoding
		out = append(out, msgpackTestFixtures[i])
	}

	w, err := os.Create("testdata/msgpack_test_fixtures.json.new")
	require.NoError(t, err)
	defer w.Close()
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(out)
}
