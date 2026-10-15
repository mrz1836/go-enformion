package enformion_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion"
	"github.com/mrz1836/go-enformion/internal/testutil"
)

// trRecorder collects what a caller's httptrace hooks see.
type trRecorder struct {
	mu     sync.Mutex
	keys   []string
	values []string
	conns  int
}

func (r *trRecorder) trace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		GotConn: func(httptrace.GotConnInfo) {
			r.mu.Lock()
			r.conns++
			r.mu.Unlock()
		},
		WroteHeaderField: func(key string, value []string) {
			r.mu.Lock()
			r.keys = append(r.keys, strings.ToLower(key))
			r.values = append(r.values, value...)
			r.mu.Unlock()
		},
	}
}

// TestTraceNeverSeesCredentials proves that a caller's httptrace hooks, which
// tracing libraries use to record request headers, never receive the API key
// name or password, over HTTP/1.1 and HTTP/2, while every other header and hook
// still reaches them.
func TestTraceNeverSeesCredentials(t *testing.T) {
	for _, http2 := range []bool{false, true} {
		name := "HTTP/1.1"
		if http2 {
			name = "HTTP/2"
		}
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewUnstartedServer(respond(http.StatusOK, testutil.Fixture(t, "synthetic/no-person.json")))
			srv.EnableHTTP2 = http2
			srv.StartTLS()
			t.Cleanup(srv.Close)
			c := newClient(t, enformion.WithTestBaseURL(srv.URL), enformion.WithHTTPClient(srv.Client()))

			rec := &trRecorder{}
			ctx := httptrace.WithClientTrace(t.Context(), rec.trace())
			req := validRequest()
			req.SessionID = "trace-session-0001"
			_, err := c.EnrichContact(ctx, req)
			require.NoError(t, err)

			rec.mu.Lock()
			defer rec.mu.Unlock()
			assert.Equal(t, 1, rec.conns, "other hooks still run")
			assert.Contains(t, rec.keys, "content-type")
			assert.Contains(t, rec.keys, "galaxy-search-type")
			assert.Contains(t, rec.keys, "galaxy-client-session-id")
			assert.NotContains(t, rec.keys, "galaxy-ap-name")
			assert.NotContains(t, rec.keys, "galaxy-ap-password")
			testutil.AssertRedacted(t, strings.Join(rec.values, "\n"), []string{testKeyName, testKeyPassword})
		})
	}
}

// TestTraceCallerContextUnchanged proves that filtering the trace keeps the
// caller's cancellation and context values.
func TestTraceCallerContextUnchanged(t *testing.T) {
	type trKey struct{}
	var seen any
	ctx, cancel := context.WithCancel(context.WithValue(t.Context(), trKey{}, "value"))
	defer cancel()
	ctx = httptrace.WithClientTrace(ctx, (&trRecorder{}).trace())
	rt := testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen = r.Context().Value(trKey{})
		// Cancel only once the request is in flight, so the caller's
		// cancellation must travel through the filtered context.
		cancel()
		return testutil.BlockUntilDone(r)
	})
	c := newTransportClient(t, rt)
	_, err := c.EnrichContact(ctx, validRequest())
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, enformion.FailureCanceled, callError(t, err).Kind)
	assert.Equal(t, "value", seen)
}
