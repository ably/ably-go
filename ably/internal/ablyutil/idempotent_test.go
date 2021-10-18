package ablyutil

import (
	"encoding/base64"
	"testing"
)

func Test_Idempotent_Publishing_9_bytes_Id_RSL1K1(t *testing.T) {
	baseId, err := BaseID()
	if err != nil {
		t.Fatalf("error processing idempotent base ID %v", err)
	}
	if len(baseId) != 12 { // 9 bytes becomes 12 after base64 encoded
		t.Fatalf("expected 12; actual %v", len(baseId))
	}
	decodedMsgIdempotentId, err := base64.StdEncoding.DecodeString(baseId)
	if err != nil {
		t.Fatalf("error decoding idempotent base ID %v", err)
	}
	if len(decodedMsgIdempotentId) != 9 {
		t.Fatalf("expected 9; actual %v", len(decodedMsgIdempotentId))
	}
}
