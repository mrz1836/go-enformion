package enformion_test

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion"
	"github.com/mrz1836/go-enformion/internal/testutil"
)

var errMMReadFailed = errors.New("synthetic read failure")

// mmPadded returns the empty envelope followed by whitespace, n bytes in all:
// valid JSON of an exact size.
func mmPadded(n int) string {
	return emptyEnvelope + strings.Repeat(" ", n-len(emptyEnvelope))
}

// mmFailingBody is a testutil.Body that fails with err, when set, instead of
// returning io.EOF.
type mmFailingBody struct {
	*testutil.Body

	err error
}

func (b *mmFailingBody) Read(p []byte) (int, error) {
	n, err := b.Body.Read(p)
	if b.err != nil && errors.Is(err, io.EOF) {
		return n, b.err
	}
	return n, err
}

// TestMemoryResponseLimit checks the size limit at its boundary, for declared
// and unknown lengths, and that a non-200 body declared over it is never read.
func TestMemoryResponseLimit(t *testing.T) {
	const limit = 4096
	tests := []struct {
		name     string
		status   int
		body     string
		declared bool
		kind     enformion.FailureKind
	}{
		{name: "exactly the limit, declared", status: http.StatusOK, body: mmPadded(limit), declared: true},
		{name: "exactly the limit, unknown length", status: http.StatusOK, body: mmPadded(limit)},
		{
			name: "one byte over, declared", status: http.StatusOK, body: mmPadded(limit + 1), declared: true,
			kind: enformion.FailureResponseTooLarge,
		},
		{name: "one byte over, unknown length", status: http.StatusOK, body: mmPadded(limit + 1), kind: enformion.FailureResponseTooLarge},
		{
			name: "429 one byte over, declared", status: http.StatusTooManyRequests, body: strings.Repeat(" ", limit+1), declared: true,
			kind: enformion.FailureRateLimited,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body *testutil.Body
			length := int64(-1)
			if tt.declared {
				length = int64(len(tt.body))
			}
			rt := testutil.Transport(tt.status, length, func() io.ReadCloser {
				body = testutil.NewBody(tt.body)
				return body
			})
			c := newTransportClient(t, rt, enformion.WithMaxResponseBytes(limit))
			res, err := c.EnrichContact(t.Context(), validRequest())
			require.NotNil(t, body)
			assert.Equal(t, 1, body.Closes(), "the body is closed exactly once")
			if tt.kind == "" {
				require.NoError(t, err)
				assert.Nil(t, res.Person)
				return
			}
			require.ErrorIs(t, err, kindSentinels()[tt.kind])
			ce := callError(t, err)
			assert.Equal(t, tt.kind, ce.Kind)
			assert.Equal(t, tt.status, ce.StatusCode)
			if tt.declared {
				assert.Zero(t, body.BytesRead(), "a declared length over the limit is not read")
			} else {
				assert.Equal(t, int64(limit+1), body.BytesRead(), "at most one byte past the limit is read")
			}
		})
	}
}

// TestMemoryHugeAndEndlessBodies checks that bodies far over the limit cost at
// most the limit, and that a non-200 status keeps its kind.
func TestMemoryHugeAndEndlessBodies(t *testing.T) {
	const limit = 1 << 20
	tests := []struct {
		name       string
		status     int
		length     int64
		wantKind   enformion.FailureKind
		wantMaxLen int64
	}{
		{name: "1 TiB declared", status: http.StatusOK, length: 1 << 40, wantKind: enformion.FailureResponseTooLarge},
		{
			name: "endless, unknown length", status: http.StatusOK, length: -1,
			wantKind: enformion.FailureResponseTooLarge, wantMaxLen: limit + 1,
		},
		{name: "503 with 1 TiB declared", status: http.StatusServiceUnavailable, length: 1 << 40, wantKind: enformion.FailureUpstream},
		{
			name: "503 endless", status: http.StatusServiceUnavailable, length: -1,
			wantKind: enformion.FailureUpstream, wantMaxLen: limit + 1,
		},
		{
			name: "429 endless", status: http.StatusTooManyRequests, length: -1,
			wantKind: enformion.FailureRateLimited, wantMaxLen: limit + 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body *testutil.EndlessBody
			rt := testutil.Transport(tt.status, tt.length, func() io.ReadCloser {
				body = &testutil.EndlessBody{}
				return body
			})
			c := newTransportClient(t, rt, enformion.WithMaxResponseBytes(limit))
			_, err := c.EnrichContact(t.Context(), validRequest())
			ce := callError(t, err)
			assert.Equal(t, tt.wantKind, ce.Kind)
			assert.Equal(t, tt.status, ce.StatusCode)
			require.NotNil(t, body)
			assert.Equal(t, tt.wantMaxLen, body.BytesRead(), "bytes read")
			assert.Equal(t, 1, body.Closes())
		})
	}
}

// TestMemoryGzipBomb serves a small gzip body that decompresses far past the
// limit. The client did not ask for an encoding, so the standard library
// decompresses it transparently; the limit counts decompressed bytes.
func TestMemoryGzipBomb(t *testing.T) {
	const (
		limit    = 1 << 20
		inflated = 64 << 20
	)
	var compressed bytes.Buffer
	zw, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	require.NoError(t, err)
	chunk := bytes.Repeat([]byte{' '}, 1<<20)
	for range inflated / len(chunk) {
		_, err = zw.Write(chunk)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	bomb := compressed.Bytes()
	t.Logf("%d compressed bytes inflate to %d", len(bomb), inflated)
	require.Less(t, len(bomb), 256<<10, "the bomb is small on the wire")

	c, _ := newServerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(bomb)
	}, enformion.WithMaxResponseBytes(limit))
	call := func() error {
		_, callErr := c.EnrichContact(t.Context(), validRequest())
		return callErr
	}
	err = call()
	require.ErrorIs(t, err, enformion.ErrResponseTooLarge)
	assert.Equal(t, http.StatusOK, callError(t, err).StatusCode)

	testutil.SkipUnderRace(t)
	perOp := testutil.BytesPerOp(func() { _ = call() })
	t.Logf("%d bytes allocated to reject a gzip bomb at a %d-byte limit", perOp, limit)
	assert.Less(t, perOp, int64(4*limit), "far below the inflated size")
}

// TestMemoryShortBody checks a body whose length differs from the declared one.
func TestMemoryShortBody(t *testing.T) {
	const content = `{"isError":false,"person":null}`
	// An injected transport may end a body cleanly at a length other than the
	// one it declared; a complete-looking JSON body must not hide that.
	tests := []struct {
		name     string
		content  string
		declared int64
		readErr  error
	}{
		// What the standard library's transport reports for a body cut short.
		{name: "cut short with an unexpected EOF", content: `{"isError"`, declared: 100, readErr: io.ErrUnexpectedEOF},
		{name: "clean EOF short of the declared length", content: content, declared: 1000},
		{name: "longer than the declared length", content: content, declared: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body *mmFailingBody
			rt := testutil.Transport(http.StatusOK, tt.declared, func() io.ReadCloser {
				body = &mmFailingBody{Body: testutil.NewBody(tt.content), err: tt.readErr}
				return body
			})
			c := newTransportClient(t, rt)
			_, err := c.EnrichContact(t.Context(), validRequest())
			require.ErrorIs(t, err, enformion.ErrTransport)
			assert.Equal(t, http.StatusOK, callError(t, err).StatusCode)
			require.NotNil(t, body)
			assert.Equal(t, 1, body.Closes())
		})
	}
	t.Run("over the wire", func(t *testing.T) {
		c, _ := newServerClient(t, func(w http.ResponseWriter, _ *http.Request) {
			hj, ok := w.(http.Hijacker)
			if !ok {
				return
			}
			conn, buf, err := hj.Hijack()
			if err != nil {
				return
			}
			_, _ = buf.WriteString("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 100\r\n\r\n{\"isError\"")
			_ = buf.Flush()
			_ = conn.Close()
		})
		_, err := c.EnrichContact(t.Context(), validRequest())
		require.ErrorIs(t, err, enformion.ErrTransport)
		assert.Equal(t, http.StatusOK, callError(t, err).StatusCode)
	})
}

// TestMemoryBodyClosedOnce checks that every path makes one attempt, closes the
// response body exactly once and reads it within the limit, and that a read
// error never reaches the error text.
func TestMemoryBodyClosedOnce(t *testing.T) {
	const limit = 4096
	match := string(testutil.Fixture(t, "provider/match.json"))
	tests := []struct {
		name     string
		status   int
		content  string
		length   int64
		readErr  error
		wantKind enformion.FailureKind
		maxRead  int64
	}{
		{name: "match", status: http.StatusOK, content: match, length: int64(len(match)), maxRead: int64(len(match))},
		{
			name: "malformed", status: http.StatusOK, content: `{"isError":false`, length: -1,
			wantKind: enformion.FailureMalformedResponse, maxRead: 16,
		},
		{name: "unexpected", status: http.StatusOK, content: `[]`, length: -1, wantKind: enformion.FailureUnexpectedResponse, maxRead: 2},
		{
			name: "too large declared", status: http.StatusOK, content: mmPadded(8192), length: 8192,
			wantKind: enformion.FailureResponseTooLarge,
		},
		{
			name: "too large streamed", status: http.StatusOK, content: mmPadded(8192), length: -1,
			wantKind: enformion.FailureResponseTooLarge, maxRead: limit + 1,
		},
		{
			name: "read error", status: http.StatusOK, content: `{"isError":`, length: -1,
			readErr: errMMReadFailed, wantKind: enformion.FailureTransport, maxRead: 11,
		},
		{name: "rejected", status: http.StatusBadRequest, content: `{}`, length: 2, wantKind: enformion.FailureRejected, maxRead: 2},
		{name: "unauthorized", status: http.StatusUnauthorized, content: ``, length: 0, wantKind: enformion.FailureUnauthorized},
		{
			name: "rate limited oversized streamed", status: http.StatusTooManyRequests, content: mmPadded(8192), length: -1,
			wantKind: enformion.FailureRateLimited, maxRead: limit + 1,
		},
		{
			name: "upstream oversized declared", status: http.StatusBadGateway, content: mmPadded(8192), length: 8192,
			wantKind: enformion.FailureUpstream,
		},
		{
			name: "read error on a refusal, where the status wins", status: http.StatusForbidden, content: `{`, length: -1,
			readErr: errMMReadFailed, wantKind: enformion.FailureUnauthorized, maxRead: 1,
		},
		{name: "redirect", status: http.StatusFound, content: ``, length: 0, wantKind: enformion.FailureUnexpectedResponse},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body *mmFailingBody
			ct := &testutil.CountingTransport{Next: testutil.Transport(tt.status, tt.length, func() io.ReadCloser {
				body = &mmFailingBody{Body: testutil.NewBody(tt.content), err: tt.readErr}
				return body
			})}
			c := newTransportClient(t, ct, enformion.WithMaxResponseBytes(limit))
			res, err := c.EnrichContact(t.Context(), validRequest())
			if tt.wantKind == "" {
				require.NoError(t, err)
				require.NotNil(t, res)
			} else {
				ce := callError(t, err)
				assert.Equal(t, tt.wantKind, ce.Kind)
				assert.Equal(t, tt.status, ce.StatusCode)
			}
			if tt.readErr != nil {
				assertErrorClean(t, err, tt.readErr.Error())
			}
			require.NotNil(t, body)
			assert.Equal(t, 1, body.Closes(), "the body is closed exactly once")
			assert.LessOrEqual(t, body.BytesRead(), tt.maxRead, "the read stays within the limit")
			assert.Equal(t, 1, ct.Attempts())
		})
	}
}

// TestMemoryAllocations guards allocation counts and bytes on the common
// paths. The ceilings sit about 25% above the measured values; the counts
// include the in-memory transport's own response.
func TestMemoryAllocations(t *testing.T) {
	testutil.SkipUnderRace(t)
	match := string(testutil.Fixture(t, "provider/match.json"))
	matchClient := newTransportClient(t, testutil.StaticTransport(http.StatusOK, match))
	emptyClient := newTransportClient(t, testutil.StaticTransport(http.StatusOK, emptyEnvelope))
	call := func(c *enformion.Client) func() {
		return func() { _, _ = c.EnrichContact(t.Context(), validRequest()) }
	}
	testutil.CheckCeilings(t, []testutil.Ceiling{
		{Name: "provider-shaped match", Max: 110, Run: call(matchClient)},
		{Name: "no person", Max: 54, Run: call(emptyClient)},
	})

	const limit = 1 << 20
	endless := newTransportClient(t,
		testutil.Transport(http.StatusOK, -1, func() io.ReadCloser { return &testutil.EndlessBody{} }),
		enformion.WithMaxResponseBytes(limit))
	perOp := testutil.BytesPerOp(call(endless))
	t.Logf("%d bytes allocated to reject an endless body at a %d-byte limit", perOp, limit)
	// Growing chunks up to the limit plus one right-sized copy: about twice it.
	assert.Less(t, perOp, int64(5*limit/2))

	huge := newTransportClient(t,
		testutil.Transport(http.StatusOK, 1<<40, func() io.ReadCloser { return &testutil.EndlessBody{} }),
		enformion.WithMaxResponseBytes(limit))
	perOp = testutil.BytesPerOp(call(huge))
	t.Logf("%d bytes allocated to reject a declared length of 1 TiB", perOp)
	assert.Less(t, perOp, int64(16<<10))
}
