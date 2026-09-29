//go:build !integration
// +build !integration

package ably_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/ably/ably-go/ably"

	"github.com/stretchr/testify/assert"
)

type recordingLogger struct {
	mtx  sync.Mutex
	logs []string
}

func (l *recordingLogger) Printf(level ably.LogLevel, format string, v ...interface{}) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.logs = append(l.logs, fmt.Sprintf(format, v...))
}

func (l *recordingLogger) lines() []string {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	return append([]string(nil), l.logs...)
}

func TestRESTPresence_HistoryLogsDecodeFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		rw.Write([]byte(`[{"action":2,"clientId":"a","data":"x","encoding":"nonsense"}]`))
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	assert.NoError(t, err)
	port, err := strconv.Atoi(serverURL.Port())
	assert.NoError(t, err)

	logger := &recordingLogger{}
	client, err := ably.NewREST(
		ably.WithTLS(false),
		ably.WithToken("token"),
		ably.WithUseBinaryProtocol(false),
		ably.WithEndpoint(serverURL.Hostname()),
		ably.WithPort(port),
		ably.WithLogLevel(ably.LogError),
		ably.WithLogHandler(logger),
	)
	assert.NoError(t, err)

	pages, err := client.Channels.Get("test").Presence.History().Pages(context.Background())
	assert.NoError(t, err)
	assert.True(t, pages.Next(context.Background()))
	assert.Len(t, pages.Items(), 1)

	var found string
	for _, line := range logger.lines() {
		if strings.Contains(line, "Couldn't fully decode presence message data") {
			found = line
		}
	}
	if assert.NotEmpty(t, found, "expected a decode failure to be logged") {
		assert.NotContains(t, found, "%!", "log line has a formatting error: %s", found)
		assert.Contains(t, found, "unknown encoding nonsense")
	}
}
