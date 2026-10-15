package enformion_test

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion"
	"github.com/mrz1836/go-enformion/internal/testutil"
)

// tpMarker is planted in every transport error; it must never surface.
const tpMarker = "tp-leak-marker-5c1e"

var (
	errTPMarker  = errors.New("dial refused " + tpMarker)
	errTPPlanted = errors.New("transport broke " + tpMarker)
)

// tpTimeout is a net.Error that timed out.
type tpTimeoutError struct{}

func (tpTimeoutError) Error() string   { return "synthetic network timeout " + tpMarker }
func (tpTimeoutError) Timeout() bool   { return true }
func (tpTimeoutError) Temporary() bool { return true }

// tpExpectedBody is the exact body validRequest encodes to.
const tpExpectedBody = `{"FirstName":"Avery","LastName":"Example","Email":"avery.example@example.com"}`

func TestTransportWireRequestOverTLS(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
	}{
		{name: "without a session ID"},
		{name: "with a session ID", sessionID: "batch-7f3a9c-0001"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, srv := newServerClient(t, respond(http.StatusOK, testutil.Fixture(t, "provider/match.json")))
			req := validRequest()
			req.SessionID = tt.sessionID
			_, err := c.EnrichContact(t.Context(), req)
			require.NoError(t, err)

			reqs := srv.Requests()
			require.Len(t, reqs, 1)
			got := reqs[0]
			assert.Equal(t, http.MethodPost, got.Method)
			assert.Equal(t, "/Contact/Enrich", got.URL, "exact path and an empty query")
			assert.Equal(t, tpExpectedBody, string(got.Body), "exact body bytes") //nolint:testifylint // byte-exact, not just equivalent JSON

			want := tpExpectedHeaders(len(tpExpectedBody), tt.sessionID)
			for k, v := range want {
				assert.Equal(t, []string{v}, got.Header.Values(k), "header %s", k)
			}
			// The transport may add Accept-Encoding; nothing else may appear.
			assert.Equal(t, tpSortedKeys(want), tpSortedKeys(got.Header, "Accept-Encoding"))
			for _, banned := range []string{"Authorization", "Cookie", "Idempotency-Key", "X-Idempotency-Key"} {
				assert.Empty(t, got.Header.Values(banned), "header %s must never be sent", banned)
			}
		})
	}
}

// tpExpectedHeaders returns every header a call must carry over the wire,
// with its single value.
func tpExpectedHeaders(bodyLen int, sessionID string) map[string]string {
	want := map[string]string{
		"Content-Type":       "application/json",
		"Accept":             "application/json",
		"User-Agent":         "go-enformion",
		"Galaxy-Ap-Name":     testKeyName,
		"Galaxy-Ap-Password": testKeyPassword,
		"Galaxy-Search-Type": "DevAPIContactEnrich",
		"Content-Length":     strconv.Itoa(bodyLen),
	}
	if sessionID != "" {
		want["Galaxy-Client-Session-Id"] = sessionID
	}
	return want
}

// tpSortedKeys returns the keys of m, sorted, without the skipped ones.
func tpSortedKeys[V any](m map[string]V, skip ...string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		if !slices.Contains(skip, k) {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}

// tpCapture is an in-memory transport that keeps every request it sees and the
// time left before each request's deadline, and answers with the match fixture.
type tpCapture struct {
	body     []byte
	reqs     []*http.Request
	bodies   []string
	timeLeft []time.Duration
	hasDL    []bool
}

func (c *tpCapture) RoundTrip(r *http.Request) (*http.Response, error) {
	deadline, ok := r.Context().Deadline()
	c.hasDL = append(c.hasDL, ok)
	c.timeLeft = append(c.timeLeft, time.Until(deadline))
	b, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()
	c.bodies = append(c.bodies, string(b))
	c.reqs = append(c.reqs, r)
	return testutil.Response(r, http.StatusOK, io.NopCloser(strings.NewReader(string(c.body))), int64(len(c.body))), nil
}

func TestTransportRequestInMemory(t *testing.T) {
	capt := &tpCapture{body: testutil.Fixture(t, "provider/match.json")}
	const timeout = 7 * time.Second
	c := newTransportClient(t, capt, enformion.WithTimeout(timeout))
	_, err := c.EnrichContact(t.Context(), validRequest())
	require.NoError(t, err)
	require.Len(t, capt.reqs, 1)

	r := capt.reqs[0]
	assert.Equal(t, http.MethodPost, r.Method)
	assert.Equal(t, "https://127.0.0.1/Contact/Enrich", r.URL.String())
	assert.Nil(t, r.GetBody, "a rewindable body would let the standard library resend the POST")
	assert.Equal(t, int64(len(tpExpectedBody)), r.ContentLength)
	assert.Equal(t, tpExpectedBody, capt.bodies[0], "exact body bytes") //nolint:testifylint // byte-exact, not just equivalent JSON

	require.True(t, capt.hasDL[0], "every call carries a deadline")
	assert.LessOrEqual(t, capt.timeLeft[0], timeout)
	assert.Greater(t, capt.timeLeft[0], timeout-2*time.Second)

	wantKeys := []string{"Accept", "Content-Type", "Galaxy-Ap-Name", "Galaxy-Ap-Password", "Galaxy-Search-Type", "User-Agent"}
	assert.Equal(t, wantKeys, tpSortedKeys(r.Header))
}

func TestTransportHeaderMapsAreIndependent(t *testing.T) {
	capt := &tpCapture{body: testutil.Fixture(t, "provider/match.json")}
	c := newTransportClient(t, capt)

	_, err := c.EnrichContact(t.Context(), validRequest())
	require.NoError(t, err)
	h1 := capt.reqs[0].Header
	_, err = c.EnrichContact(t.Context(), validRequest())
	require.NoError(t, err)
	h2 := capt.reqs[1].Header

	assert.NotEqual(t, reflect.ValueOf(h1).Pointer(), reflect.ValueOf(h2).Pointer(), "each call gets a fresh header map")
	for k, v := range h1 {
		require.Len(t, v, 1, k)
		assert.Equal(t, 1, cap(v), "a value must not leave room for an append to overwrite the next: %s", k)
		require.Len(t, h2[k], 1, k)
		assert.NotSame(t, &v[0], &h2[k][0], "header %s shares memory between calls", k)
	}

	// Appending to and rewriting the first call's headers changes nothing else.
	h1["Accept"] = append(h1["Accept"], "text/plain")
	h1.Set("Galaxy-Ap-Name", "changed")
	h1.Set("X-Planted", "1")
	assert.Equal(t, []string{"go-enformion"}, h1.Values("User-Agent"))
	assert.Equal(t, []string{"application/json"}, h2.Values("Accept"))
	assert.Equal(t, []string{testKeyName}, h2.Values("Galaxy-Ap-Name"))

	_, err = c.EnrichContact(t.Context(), validRequest())
	require.NoError(t, err)
	h3 := capt.reqs[2].Header
	assert.Equal(t, []string{testKeyName}, h3.Values("Galaxy-Ap-Name"))
	assert.Equal(t, []string{"application/json"}, h3.Values("Accept"))
	assert.Empty(t, h3.Values("X-Planted"))
}

func TestTransportErrorClassification(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		kind     enformion.FailureKind
		sentinel error
		cause    string
	}{
		{"dns", &net.DNSError{Err: "no such host " + tpMarker, Name: tpMarker + ".invalid", Server: tpMarker}, enformion.FailureTransport, enformion.ErrTransport, "DNS lookup failed"},
		{"dial", &net.OpError{Op: "dial", Net: "tcp", Err: errTPMarker}, enformion.FailureTransport, enformion.ErrTransport, "transport: connection failed"},
		{"proxy", &net.OpError{Op: "proxyconnect", Net: "tcp", Err: errTPMarker}, enformion.FailureTransport, enformion.ErrTransport, "proxy connection failed"},
		{"tls record header", tls.RecordHeaderError{Msg: "bad record " + tpMarker, RecordHeader: [5]byte{'X', 'Y', 'Z', 0, 0}}, enformion.FailureTransport, enformion.ErrTransport, "TLS failed"},
		{"tls alert", tls.AlertError(40), enformion.FailureTransport, enformion.ErrTransport, "TLS failed"},
		{"tls certificate", &tls.CertificateVerificationError{Err: errTPMarker}, enformion.FailureTransport, enformion.ErrTransport, "TLS failed"},
		{"connection reset", &net.OpError{Op: "read", Net: "tcp", Err: os.NewSyscallError("read", syscall.ECONNRESET)}, enformion.FailureTransport, enformion.ErrTransport, "connection reset"},
		{"eof", io.EOF, enformion.FailureTransport, enformion.ErrTransport, "connection closed early"},
		{"unexpected eof", io.ErrUnexpectedEOF, enformion.FailureTransport, enformion.ErrTransport, "connection closed early"},
		{"other", errTPPlanted, enformion.FailureTransport, enformion.ErrTransport, "transport: request failed"},
		{"canceled by the transport alone", context.Canceled, enformion.FailureTransport, enformion.ErrTransport, "transport: request failed"},
		{"network timeout", tpTimeoutError{}, enformion.FailureTimeout, enformion.ErrTimeout, "context deadline exceeded"},
		{"deadline from the transport", context.DeadlineExceeded, enformion.FailureTimeout, enformion.ErrTimeout, "context deadline exceeded"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct := &testutil.CountingTransport{Next: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
				testutil.Drain(r)
				return nil, tt.err
			})}
			c := newTransportClient(t, ct)
			res, err := c.EnrichContact(t.Context(), validRequest())
			assert.Nil(t, res)
			require.ErrorIs(t, err, tt.sentinel)
			ce := callError(t, err)
			assert.Equal(t, tt.kind, ce.Kind)
			assert.Zero(t, ce.StatusCode)
			assert.Equal(t, 1, ct.Attempts(), "exactly one attempt")
			cause := errors.Unwrap(err)
			require.Error(t, cause, "the sanitized cause")
			assert.Contains(t, cause.Error(), tt.cause, "the fixed class of the cause")
			if tt.kind == enformion.FailureTimeout {
				require.ErrorIs(t, err, context.DeadlineExceeded)
			}
			assertErrorClean(t, err, tpMarker, "127.0.0.1", "/Contact/Enrich", testKeyName, testKeyPassword)
		})
	}
}

func TestTransportRealDialFailure(t *testing.T) {
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	base, hc := srv.URL, srv.Client()
	srv.Close()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(base, "https://"))
	require.NoError(t, err)

	ct := &testutil.CountingTransport{Next: hc.Transport}
	c := newClient(t, enformion.WithTestBaseURL(base), enformion.WithHTTPClient(&http.Client{Transport: ct}))
	res, err := c.EnrichContact(t.Context(), validRequest())
	assert.Nil(t, res)
	require.ErrorIs(t, err, enformion.ErrTransport)
	ce := callError(t, err)
	assert.Equal(t, enformion.FailureTransport, ce.Kind)
	assert.Zero(t, ce.StatusCode)
	assert.Equal(t, 1, ct.Attempts())
	assertErrorClean(t, err, ":"+port, "127.0.0.1", testKeyName, testKeyPassword)
}

func TestTransportRealTLSFailure(t *testing.T) {
	srv := testutil.NewServer(t, respond(http.StatusOK, testutil.Fixture(t, "provider/match.json")))
	// A client with no transport uses http.DefaultTransport, which does not
	// trust the test server's certificate.
	c := newClient(t, enformion.WithTestBaseURL(srv.URL()), enformion.WithHTTPClient(&http.Client{}))
	res, err := c.EnrichContact(t.Context(), validRequest())
	assert.Nil(t, res)
	require.ErrorIs(t, err, enformion.ErrTransport)
	assert.Equal(t, enformion.FailureTransport, callError(t, err).Kind)
	assert.Zero(t, srv.Hits(), "the handshake fails before any request is sent")
	assertErrorClean(t, err, srv.URL(), testKeyName, testKeyPassword)
}

// TestTransportProductionOrigin proves that without WithTestBaseURL every call
// goes to the documented production endpoint. The in-memory transport never
// dials.
func TestTransportProductionOrigin(t *testing.T) {
	var got string
	rt := testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		got = r.URL.String()
		return testutil.StaticTransport(http.StatusOK, emptyEnvelope)(r)
	})
	c := newClient(t, enformion.WithHTTPClient(&http.Client{Transport: rt}))
	_, err := c.EnrichContact(t.Context(), validRequest())
	require.NoError(t, err)
	assert.Equal(t, "https://devapi.enformion.com/Contact/Enrich", got)
}
