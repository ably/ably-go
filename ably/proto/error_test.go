package proto_test

import (
	"strings"
	"testing"

	"github.com/ably/ably-go/ably/proto"
)

func TestErrorInfo(t *testing.T) {
	t.Run("without an error code", func(ts *testing.T) {
		e := &proto.ErrorInfo{
			StatusCode: 401,
		}
		h := "help.ably.io"
		if strings.Contains(e.Error(), h) {
			ts.Errorf("expected error message not to contain %s", h)
		}
	})
	t.Run("with an error code", func(ts *testing.T) {
		e := &proto.ErrorInfo{
			Code: 44444,
		}
		h := "https://help.ably.io/error/44444"
		if !strings.Contains(e.Error(), h) {
			ts.Errorf("expected error message %s  to contain %s", e.Error(), h)
		}
	})
	t.Run("with an error code and an href attribute", func(ts *testing.T) {
		href := "http://foo.bar.com/"
		e := &proto.ErrorInfo{
			Code: 44444,
			HRef: href,
		}
		h := "https://help.ably.io/error/44444"
		if strings.Contains(e.Error(), h) {
			ts.Errorf("expected error message %s not to contain %s", e.Error(), h)
		}
		if !strings.Contains(e.Error(), href) {
			ts.Errorf("expected error message %s  to contain %s", e.Error(), href)
		}
	})

	t.Run("with an error code and a message with the same error URL", func(ts *testing.T) {
		e := &proto.ErrorInfo{
			Code:    44444,
			Message: "error https://help.ably.io/error/44444",
		}
		h := "https://help.ably.io/error/44444"
		if !strings.Contains(e.Error(), h) {
			ts.Errorf("expected error message %s  to contain %s", e.Error(), h)
		}
		n := strings.Count(e.Error(), h)
		if n != 1 {
			ts.Errorf("expected 1 occupance of %s got %d", h, n)
		}
	})
	t.Run("with an error code and a message with a different error URL", func(ts *testing.T) {
		e := &proto.ErrorInfo{
			Code:    44444,
			Message: "error https://help.ably.io/error/123123",
		}
		h := "https://help.ably.io/error/44444"
		if !strings.Contains(e.Error(), h) {
			ts.Errorf("expected error message %s  to contain %s", e.Error(), h)
		}
		n := strings.Count(e.Error(), "help.ably.io")
		if n != 2 {
			ts.Errorf("expected 2 got %d", n)
		}
		n = strings.Count(e.Error(), "/123123")
		if n != 1 {
			ts.Errorf("expected 1 got %d", n)
		}
		n = strings.Count(e.Error(), "/44444")
		if n != 1 {
			ts.Errorf("expected 1 got %d", n)
		}
	})
}
