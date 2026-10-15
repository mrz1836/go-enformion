package enformion_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion"
	"github.com/mrz1836/go-enformion/internal/testutil"
)

// toBody is a response body whose Read blocks until the request's context is
// done and then returns that context's error. It counts its closes; tests read
// the count only after the call returned, which orders the accesses.
type toBody struct {
	wait   func() error
	closes int
}

func (b *toBody) Read([]byte) (int, error) { return 0, b.wait() }

func (b *toBody) Close() error {
	b.closes++
	return nil
}

// toStallingBody returns a transport that answers with status and a body that
// stalls until the request's context ends. A status of 0 stalls before any
// response instead.
func toStallingBody(status int, body *toBody) testutil.RoundTripFunc {
	if status == 0 {
		return testutil.BlockUntilDone
	}
	return func(r *http.Request) (*http.Response, error) {
		testutil.Drain(r)
		ctx := r.Context()
		body.wait = func() error {
			<-ctx.Done()
			return ctx.Err()
		}
		return testutil.Response(r, status, body, -1), nil
	}
}

// toCall makes one call and returns the time it took, which inside a bubble is
// the fake clock's exact elapsed time.
func toCall(ctx context.Context, c *enformion.Client) (time.Duration, error) {
	start := time.Now()
	_, err := c.EnrichContact(ctx, validRequest())
	return time.Since(start), err
}

// TestTimeoutBoundsTheWholeCall runs each case in a synctest bubble with an
// in-memory transport, so the fake clock measures exactly when the call ends.
func TestTimeoutBoundsTheWholeCall(t *testing.T) {
	tests := []struct {
		name           string
		status         int           // 0 stalls before the response headers
		sdkTimeout     time.Duration // 0 keeps the default of 30s
		clientTimeout  time.Duration // an injected http.Client's Timeout
		callerTimeout  time.Duration // the caller's context deadline
		wantKind       enformion.FailureKind
		wantStatus     int
		wantElapsed    time.Duration
		wantErrTimeout bool
	}{
		{
			name: "default timeout awaiting headers", wantKind: enformion.FailureTimeout,
			wantElapsed: 30 * time.Second, wantErrTimeout: true,
		},
		{
			name: "custom timeout awaiting headers", sdkTimeout: 7 * time.Second,
			wantKind: enformion.FailureTimeout, wantElapsed: 7 * time.Second, wantErrTimeout: true,
		},
		{
			name: "default timeout mid-body", status: http.StatusOK, wantKind: enformion.FailureTimeout,
			wantStatus: http.StatusOK, wantElapsed: 30 * time.Second, wantErrTimeout: true,
		},
		{
			name: "custom timeout mid-body", status: http.StatusOK, sdkTimeout: 11 * time.Second,
			wantKind: enformion.FailureTimeout, wantStatus: http.StatusOK, wantElapsed: 11 * time.Second,
			wantErrTimeout: true,
		},
		{
			name: "caller deadline awaiting headers", callerTimeout: 5 * time.Second,
			wantKind: enformion.FailureDeadlineExceeded, wantElapsed: 5 * time.Second,
		},
		{
			name: "caller deadline mid-body", status: http.StatusOK, callerTimeout: 5 * time.Second,
			wantKind: enformion.FailureDeadlineExceeded, wantStatus: http.StatusOK, wantElapsed: 5 * time.Second,
		},
		{
			name: "SDK timeout shorter than the caller deadline", sdkTimeout: 4 * time.Second,
			callerTimeout: 6 * time.Second, wantKind: enformion.FailureTimeout,
			wantElapsed: 4 * time.Second, wantErrTimeout: true,
		},
		{
			name: "injected client timeout awaiting headers", clientTimeout: 3 * time.Second,
			wantKind: enformion.FailureTimeout, wantElapsed: 3 * time.Second, wantErrTimeout: true,
		},
		{
			name: "injected client timeout mid-body", status: http.StatusOK, clientTimeout: 3 * time.Second,
			wantKind: enformion.FailureTimeout, wantStatus: http.StatusOK, wantElapsed: 3 * time.Second,
			wantErrTimeout: true,
		},
		{
			name: "non-200 body read stalls until the timeout", status: http.StatusServiceUnavailable,
			sdkTimeout: 9 * time.Second, wantKind: enformion.FailureUpstream,
			wantStatus: http.StatusServiceUnavailable, wantElapsed: 9 * time.Second,
		},
		{
			name: "non-200 body read stalls until the caller deadline", status: http.StatusTooManyRequests,
			callerTimeout: 2 * time.Second, wantKind: enformion.FailureRateLimited,
			wantStatus: http.StatusTooManyRequests, wantElapsed: 2 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				body := &toBody{}
				ct := &testutil.CountingTransport{Next: toStallingBody(tt.status, body)}
				opts := []enformion.Option{
					enformion.WithTestBaseURL(loopbackOrigin),
					enformion.WithHTTPClient(&http.Client{Transport: ct, Timeout: tt.clientTimeout}),
				}
				if tt.sdkTimeout > 0 {
					opts = append(opts, enformion.WithTimeout(tt.sdkTimeout))
				}
				c := newClient(t, opts...)

				ctx := t.Context()
				if tt.callerTimeout > 0 {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, tt.callerTimeout)
					defer cancel()
				}
				elapsed, err := toCall(ctx, c)
				require.Error(t, err)

				ce := callError(t, err)
				assert.Equal(t, tt.wantKind, ce.Kind)
				assert.Equal(t, tt.wantStatus, ce.StatusCode)
				assert.Equal(t, tt.wantElapsed, elapsed, "the call must end exactly at the deadline")
				assert.Equal(t, 1, ct.Attempts(), "exactly one attempt")
				assert.Equal(t, tt.wantErrTimeout, errors.Is(err, enformion.ErrTimeout), "errors.Is(err, ErrTimeout)")
				if tt.wantKind == enformion.FailureTimeout || tt.wantKind == enformion.FailureDeadlineExceeded {
					require.ErrorIs(t, err, context.DeadlineExceeded)
				}
				if tt.status != 0 {
					assert.Equal(t, 1, body.closes, "the body is closed exactly once")
				}
			})
		})
	}
}

// TestTimeoutCallerCancel cancels the caller's context while the call waits,
// from another goroutine, once everything in the bubble is blocked.
func TestTimeoutCallerCancel(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		wantStatus int
	}{
		{name: "awaiting headers"},
		{name: "mid-body", status: http.StatusOK, wantStatus: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				body := &toBody{}
				ct := &testutil.CountingTransport{Next: toStallingBody(tt.status, body)}
				c := newTransportClient(t, ct)
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				type outcome struct {
					elapsed time.Duration
					err     error
				}
				done := make(chan outcome, 1)
				go func() {
					elapsed, err := toCall(ctx, c)
					done <- outcome{elapsed: elapsed, err: err}
				}()
				synctest.Wait()
				require.Empty(t, done, "the call must still be waiting")
				cancel()
				out := <-done

				require.ErrorIs(t, out.err, context.Canceled)
				require.NotErrorIs(t, out.err, context.DeadlineExceeded)
				require.NotErrorIs(t, out.err, enformion.ErrTimeout)
				ce := callError(t, out.err)
				assert.Equal(t, enformion.FailureCanceled, ce.Kind)
				assert.Equal(t, tt.wantStatus, ce.StatusCode)
				assert.Zero(t, out.elapsed, "no fake time passes before the cancel")
				assert.Equal(t, 1, ct.Attempts())
				if tt.status != 0 {
					assert.Equal(t, 1, body.closes)
				}
			})
		})
	}
}
