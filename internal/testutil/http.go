package testutil

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// RoundTripFunc adapts a function to http.RoundTripper.
type RoundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip returns f(r).
func (f RoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// CountingTransport counts the round trips it passes on to Next.
type CountingTransport struct {
	Next     http.RoundTripper
	attempts atomic.Int64
}

// RoundTrip counts the attempt and delegates to Next.
func (c *CountingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.attempts.Add(1)
	return c.Next.RoundTrip(r)
}

// Attempts returns the number of round trips so far.
func (c *CountingTransport) Attempts() int { return int(c.attempts.Load()) }

// Drain reads and closes the request body, as a real transport would, and
// returns its length.
func Drain(r *http.Request) int64 {
	if r.Body == nil {
		return 0
	}
	n, _ := io.Copy(io.Discard, r.Body)
	_ = r.Body.Close()
	return n
}

// BlockUntilDone is an in-memory round trip that never answers: it drains the
// request, waits for the request's context to end and returns its error.
func BlockUntilDone(r *http.Request) (*http.Response, error) {
	Drain(r)
	<-r.Context().Done()
	return nil, r.Context().Err()
}

// Request is one request as a test saw it.
type Request struct {
	Method string
	// URL is the request's URL as received: the path and query on a server,
	// the full URL in a transport.
	URL     string
	Header  http.Header
	Body    []byte
	GetBody bool
}

// Record reads and closes r's body and returns a copy of the request.
func Record(r *http.Request) Request {
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
		_ = r.Body.Close()
	}
	return Request{
		Method:  r.Method,
		URL:     r.URL.String(),
		Header:  r.Header.Clone(),
		Body:    body,
		GetBody: r.GetBody != nil,
	}
}

// Response returns a response to r with the given status, body and declared
// length (-1 for unknown), as an in-memory transport would.
func Response(r *http.Request, status int, body io.ReadCloser, contentLength int64) *http.Response {
	return &http.Response{
		Status:        http.StatusText(status),
		StatusCode:    status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{"Content-Type": {"application/json"}},
		Body:          body,
		ContentLength: contentLength,
		Request:       r,
	}
}

// Transport answers every request in memory with status and the body newBody
// returns, declared with contentLength (-1 for unknown), after draining the
// request body.
func Transport(status int, contentLength int64, newBody func() io.ReadCloser) RoundTripFunc {
	return func(r *http.Request) (*http.Response, error) {
		Drain(r)
		return Response(r, status, newBody(), contentLength), nil
	}
}

// StaticTransport answers every request in memory with the same status and
// body, after draining the request body.
func StaticTransport(status int, body string) RoundTripFunc {
	return Transport(status, int64(len(body)), func() io.ReadCloser { return io.NopCloser(strings.NewReader(body)) })
}

// Server is a local TLS server that records every request before handing it
// to its handler. Handlers run on server goroutines, so they should only
// answer; tests assert on Requests afterward.
type Server struct {
	srv  *httptest.Server
	mu   sync.Mutex
	reqs []Request
}

// NewServer starts a Server that records each request and then calls h. It is
// closed when the test ends. Its error log is discarded: a server closing while
// a client finishes a spare TLS handshake logs a harmless error.
func NewServer(tb testing.TB, h http.HandlerFunc) *Server {
	tb.Helper()
	s := &Server{}
	s.srv = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := Record(r)
		s.mu.Lock()
		s.reqs = append(s.reqs, rec)
		s.mu.Unlock()
		h(w, r)
	}))
	s.srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	s.srv.StartTLS()
	tb.Cleanup(s.srv.Close)
	return s
}

// URL returns the server's base URL, such as https://127.0.0.1:port.
func (s *Server) URL() string { return s.srv.URL }

// Client returns an HTTP client that trusts the server's certificate.
func (s *Server) Client() *http.Client { return s.srv.Client() }

// Requests returns a copy of the recorded requests, in arrival order.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.reqs)
}

// Hits returns the number of requests received.
func (s *Server) Hits() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.reqs)
}

// bodyCounters counts the bytes read from a response body and its closes.
type bodyCounters struct {
	read   atomic.Int64
	closes atomic.Int32
}

// Close counts the close.
func (c *bodyCounters) Close() error {
	c.closes.Add(1)
	return nil
}

// Closes returns the number of Close calls.
func (c *bodyCounters) Closes() int { return int(c.closes.Load()) }

// BytesRead returns the number of bytes read.
func (c *bodyCounters) BytesRead() int64 { return c.read.Load() }

// Body is an io.ReadCloser over fixed content that counts its closes and the
// bytes read from it.
type Body struct {
	bodyCounters

	r io.Reader
}

// NewBody returns a Body over content.
func NewBody(content string) *Body { return &Body{r: strings.NewReader(content)} }

// Read reads from the content.
func (b *Body) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	b.read.Add(int64(n))
	return n, err
}

// EndlessBody streams 'x' forever, counting the bytes read and the closes.
type EndlessBody struct {
	bodyCounters
}

// Read fills p with 'x'.
func (b *EndlessBody) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	b.read.Add(int64(len(p)))
	return len(p), nil
}
