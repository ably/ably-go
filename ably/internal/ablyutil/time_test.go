//go:build !integration
// +build !integration

package ablyutil_test

import (
	"testing"
	"time"

	"github.com/ably/ably-go/ably/internal/ablyutil"
	"golang.org/x/net/context"
)

func TestAfterOK(t *testing.T) {

	const wait = 10 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), wait*2)
	defer cancel()

	now := time.Now()

	select {
	case firedAt := <-ablyutil.After(ctx, wait):
		if elapsed := firedAt.Sub(now); !isCloseTo(elapsed, wait) {
			t.Errorf("expected timer to fire after ~%v; got %v", wait, elapsed)
		}
	case <-ctx.Done():
		t.Error("expected timer to be done before the context is canceled")
	}
}

func TestAfterCanceled(t *testing.T) {

	const wait = 10 * time.Millisecond

	testCtx, cancel := context.WithTimeout(context.Background(), wait*2)
	defer cancel()

	ctx, cancel := context.WithCancel(testCtx)
	defer cancel()

	var canceledAt time.Time

	go func() {
		time.Sleep(wait / 2)
		canceledAt = time.Now()
		cancel()
	}()

	select {
	case _, ok := <-ablyutil.After(ctx, wait):
		if ok {
			t.Error("expected timer channel to be closed on cancel")
		}
		if sinceCancel := time.Since(canceledAt); !isCloseTo(sinceCancel, 0) {
			t.Errorf("expected timer to fire immediately after cancel; got %v", sinceCancel)
		}
	case <-testCtx.Done():
		t.Error("expected timer to be done before the context is canceled")
	}
}

func isCloseTo(d time.Duration, to time.Duration) bool {
	const leeway = 2 * time.Millisecond
	return d > to-leeway && d < to+leeway
}
