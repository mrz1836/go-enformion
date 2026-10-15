package testutil_test

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion/internal/testutil"
)

// newRequest returns a POST to a loopback URL with body.
func newRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://127.0.0.1/p?q=1", strings.NewReader(body))
	require.NoError(t, err)
	return req
}

// readAll reads and closes resp's body.
func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return string(data)
}

func TestTransportAnswersInMemory(t *testing.T) {
	bodies := 0
	ct := &testutil.CountingTransport{Next: testutil.Transport(http.StatusTeapot, -1, func() io.ReadCloser {
		bodies++
		return io.NopCloser(strings.NewReader("tea"))
	})}
	for range 2 {
		req := newRequest(t, "payload")
		resp, err := ct.RoundTrip(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusTeapot, resp.StatusCode)
		assert.Equal(t, http.StatusText(http.StatusTeapot), resp.Status)
		assert.Equal(t, int64(-1), resp.ContentLength)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
		assert.Same(t, req, resp.Request)
		assert.Equal(t, "tea", readAll(t, resp))
		n, err := req.Body.Read(make([]byte, 1))
		assert.Zero(t, n, "the request body was drained")
		require.ErrorIs(t, err, io.EOF)
	}
	assert.Equal(t, 2, ct.Attempts())
	assert.Equal(t, 2, bodies, "each response gets a fresh body")
}

func TestStaticTransport(t *testing.T) {
	resp, err := testutil.StaticTransport(http.StatusOK, "static").RoundTrip(newRequest(t, ""))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int64(len("static")), resp.ContentLength)
	assert.Equal(t, "static", readAll(t, resp))
}

func TestDrainAndRecord(t *testing.T) {
	req := newRequest(t, "payload")
	req.Header.Set("X-Test", "v")
	rec := testutil.Record(req)
	assert.Equal(t, testutil.Request{
		Method:  http.MethodPost,
		URL:     "https://127.0.0.1/p?q=1",
		Header:  http.Header{"X-Test": {"v"}},
		Body:    []byte("payload"),
		GetBody: true,
	}, rec)
	req.Header.Set("X-Test", "changed")
	assert.Equal(t, "v", rec.Header.Get("X-Test"), "the header is a copy")
	assert.Zero(t, testutil.Drain(req), "Record consumed the body")

	assert.Equal(t, int64(len("payload")), testutil.Drain(newRequest(t, "payload")))

	bare := &http.Request{Method: http.MethodGet, URL: &url.URL{Path: "/bare"}, Header: http.Header{}}
	assert.Zero(t, testutil.Drain(bare))
	assert.Equal(t, testutil.Request{Method: http.MethodGet, URL: "/bare", Header: http.Header{}}, testutil.Record(bare))
}

func TestBlockUntilDone(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://127.0.0.1/", strings.NewReader("payload"))
	require.NoError(t, err)
	resp, err := testutil.BlockUntilDone(req) //nolint:bodyclose // it never returns a response
	assert.Nil(t, resp)
	require.ErrorIs(t, err, context.Canceled)
}

func TestServerRecordsRequests(t *testing.T) {
	srv := testutil.NewServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "pong")
	})
	assert.True(t, strings.HasPrefix(srv.URL(), "https://127.0.0.1:"), srv.URL())
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL()+"/p?q=1", strings.NewReader("ping"))
	require.NoError(t, err)
	req.Header.Set("X-Test", "v")
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	assert.Equal(t, "pong", readAll(t, resp))

	assert.Equal(t, 1, srv.Hits())
	reqs := srv.Requests()
	require.Len(t, reqs, 1)
	assert.Equal(t, http.MethodPost, reqs[0].Method)
	assert.Equal(t, "/p?q=1", reqs[0].URL)
	assert.Equal(t, "ping", string(reqs[0].Body))
	assert.Equal(t, "v", reqs[0].Header.Get("X-Test"))

	reqs[0] = testutil.Request{}
	assert.Equal(t, "ping", string(srv.Requests()[0].Body), "Requests returns a copy")
}

func TestBodies(t *testing.T) {
	b := testutil.NewBody("hello")
	data, err := io.ReadAll(b)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
	assert.Equal(t, int64(5), b.BytesRead())
	assert.Zero(t, b.Closes())
	require.NoError(t, b.Close())
	require.NoError(t, b.Close())
	assert.Equal(t, 2, b.Closes(), "every close is counted")

	e := &testutil.EndlessBody{}
	p := make([]byte, 8)
	for range 3 {
		n, err := e.Read(p)
		require.NoError(t, err)
		assert.Equal(t, len(p), n)
		assert.Equal(t, "xxxxxxxx", string(p))
	}
	assert.Equal(t, int64(24), e.BytesRead())
	require.NoError(t, e.Close())
	assert.Equal(t, 1, e.Closes())
}
