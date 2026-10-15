package enformion_test

import (
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion"
	"github.com/mrz1836/go-enformion/internal/testutil"
)

// hsStuckBody ignores every context: its Read blocks until Close.
type hsStuckBody struct {
	once   sync.Once
	done   chan struct{}
	closes atomic.Int32
}

func newHSStuckBody() *hsStuckBody { return &hsStuckBody{done: make(chan struct{})} }

func (b *hsStuckBody) Read([]byte) (int, error) {
	<-b.done
	return 0, net.ErrClosed
}

func (b *hsStuckBody) Close() error {
	b.closes.Add(1)
	b.once.Do(func() { close(b.done) })
	return nil
}

// TestHostileBodyIgnoringContextStillTimesOut proves that a response body whose
// reads ignore the context, as a raw connection's do, is still stopped at the
// call's timeout and closed exactly once.
func TestHostileBodyIgnoringContextStillTimesOut(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				body := newHSStuckBody()
				ct := &testutil.CountingTransport{Next: testutil.Transport(status, -1, func() io.ReadCloser { return body })}
				c := newTransportClient(t, ct, enformion.WithTimeout(4*time.Second))
				start := time.Now()
				_, err := c.EnrichContact(t.Context(), validRequest())
				require.Error(t, err)
				assert.Equal(t, 4*time.Second, time.Since(start))
				ce := callError(t, err)
				if status == http.StatusOK {
					assert.Equal(t, enformion.FailureTimeout, ce.Kind)
				} else {
					assert.Equal(t, enformion.FailureUpstream, ce.Kind, "the status decides a non-200 outcome")
				}
				assert.Equal(t, status, ce.StatusCode)
				assert.Equal(t, int32(1), body.closes.Load())
				assert.Equal(t, 1, ct.Attempts())
			})
		})
	}
}

// TestHostileInformationalStatus proves that a 1xx response that reaches the
// SDK, such as 101 Switching Protocols with the raw connection as its body, is
// classified at once without reading that body.
func TestHostileInformationalStatus(t *testing.T) {
	for _, status := range []int{http.StatusContinue, http.StatusSwitchingProtocols} {
		t.Run("in memory "+strconv.Itoa(status), func(t *testing.T) {
			body := newHSStuckBody()
			ct := &testutil.CountingTransport{Next: testutil.Transport(status, -1, func() io.ReadCloser { return body })}
			c := newTransportClient(t, ct)
			_, err := c.EnrichContact(t.Context(), validRequest())
			require.ErrorIs(t, err, enformion.ErrUnexpectedResponse)
			assert.Equal(t, status, callError(t, err).StatusCode)
			assert.Equal(t, int32(1), body.closes.Load())
			assert.Equal(t, 1, ct.Attempts())
		})
	}

	t.Run("over the wire", func(t *testing.T) {
		release := make(chan struct{})
		c, _ := newServerClient(t, func(w http.ResponseWriter, _ *http.Request) {
			conn, buf, err := w.(http.Hijacker).Hijack()
			if err != nil {
				return
			}
			defer func() { _ = conn.Close() }()
			_, _ = buf.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: synthetic\r\nConnection: Upgrade\r\n\r\n")
			_ = buf.Flush()
			// Hold the upgraded connection open, as a server would.
			<-release
		}, enformion.WithTimeout(time.Minute))
		// Cleanups run last first, so the handler is released before the
		// server closes.
		t.Cleanup(func() { close(release) })

		start := time.Now()
		_, err := c.EnrichContact(t.Context(), validRequest())
		require.ErrorIs(t, err, enformion.ErrUnexpectedResponse)
		assert.Equal(t, http.StatusSwitchingProtocols, callError(t, err).StatusCode)
		assert.Less(t, time.Since(start), 10*time.Second, "the call must not wait for the timeout")
	})
}
