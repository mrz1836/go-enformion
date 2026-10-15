package enformion_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion"
	"github.com/mrz1836/go-enformion/internal/testutil"
)

// HTTP/2 frame types, flags and error codes the raw server uses.
const (
	rpFrameData     = 0x0
	rpFrameHeaders  = 0x1
	rpFrameRST      = 0x3
	rpFrameSettings = 0x4
	rpFramePing     = 0x6
	rpFrameGoAway   = 0x7
	rpFrameWindow   = 0x8
	rpFlagEnd       = 0x1 // END_STREAM on HEADERS and DATA, ACK on SETTINGS and PING
	rpCodeNoError   = 0x0
	rpCodeProtocol  = 0x1
	rpCodeRefused   = 0x7
	rpPreface       = "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"
	rpMaxFrameBytes = 1 << 24
)

// rpMarker is GOAWAY debug data. The standard library quotes it in its error
// text, so it must never reach anything the SDK returns.
const rpMarker = "goaway-debug-marker-5d1e"

var errRPFrameTooLarge = errors.New("frame too large")

// rpAction answers a request whose stream has ended: it writes frames to w and
// reports whether the connection should then be closed.
type rpAction func(w io.Writer, stream uint32) (closeConn bool)

// rpServer is a minimal HTTP/2 server that reads each request completely, as a
// server that may already have acted on it would, and then answers with its
// action instead of a response.
type rpServer struct {
	ln       net.Listener
	action   rpAction
	requests atomic.Int64
	notH2    atomic.Bool // a client spoke something other than HTTP/2

	mu    sync.Mutex
	conns []net.Conn
	wg    sync.WaitGroup
}

// rpStart starts a raw HTTP/2 server and returns it with an HTTP/2 client
// transport that trusts its certificate.
func rpStart(t *testing.T, action rpAction) (*rpServer, *http.Transport) {
	t.Helper()
	ts := httptest.NewTLSServer(http.NotFoundHandler())
	cert := ts.TLS.Certificates[0]
	base, ok := ts.Client().Transport.(*http.Transport)
	require.True(t, ok)
	tr := base.Clone()
	ts.Close()
	tr.ForceAttemptHTTP2 = true

	lc := net.ListenConfig{}
	inner, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ln := tls.NewListener(inner, &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h2"},
		MinVersion:   tls.VersionTLS12,
	})
	s := &rpServer{ln: ln, action: action}
	s.wg.Go(s.accept)
	t.Cleanup(func() {
		_ = ln.Close()
		tr.CloseIdleConnections()
		s.mu.Lock()
		for _, c := range s.conns {
			_ = c.Close()
		}
		s.mu.Unlock()
		s.wg.Wait()
	})
	return s, tr
}

func (s *rpServer) origin() string { return "https://" + s.ln.Addr().String() }

// requireH2 skips the test when the client did not speak HTTP/2, so that the
// replay checks never pass or hang vacuously.
func (s *rpServer) requireH2(t *testing.T) {
	t.Helper()
	if s.notH2.Load() {
		t.Skip("HTTP/2 was not negotiated (is GODEBUG=http2client=0 set?)")
	}
}

func (s *rpServer) accept() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		s.conns = append(s.conns, c)
		s.mu.Unlock()
		s.wg.Go(func() { s.serve(c) })
	}
}

func (s *rpServer) serve(c net.Conn) {
	defer func() { _ = c.Close() }()
	r := bufio.NewReader(c)
	preface := make([]byte, len(rpPreface))
	if _, err := io.ReadFull(r, preface); err != nil || string(preface) != rpPreface {
		// Go lets an HTTP/1.1 client reach an h2-only server without ALPN, as
		// with GODEBUG=http2client=0. Close at once instead of waiting for frames.
		s.notH2.Store(true)
		return
	}
	if rpWriteFrame(c, rpFrameSettings, 0, 0, nil) != nil {
		return
	}
	for {
		typ, flags, stream, payload, err := rpReadFrame(r)
		if err != nil {
			return
		}
		if s.handle(c, typ, flags, stream, payload) {
			return
		}
	}
}

// handle answers one client frame: it acknowledges settings and pings, counts
// requests, returns flow-control credit, and runs the action once a request's
// stream has ended. It reports whether to close the connection.
func (s *rpServer) handle(w io.Writer, typ, flags byte, stream uint32, payload []byte) bool {
	end := flags&rpFlagEnd != 0
	switch typ {
	case rpFrameSettings:
		if !end {
			_ = rpWriteFrame(w, rpFrameSettings, rpFlagEnd, 0, nil)
		}
		return false
	case rpFramePing:
		if !end {
			_ = rpWriteFrame(w, rpFramePing, rpFlagEnd, 0, payload)
		}
		return false
	case rpFrameHeaders:
		s.requests.Add(1)
	case rpFrameData:
		if n := len(payload); n > 0 && n <= rpMaxFrameBytes {
			_ = rpWriteFrame(w, rpFrameWindow, 0, 0, binary.BigEndian.AppendUint32(nil, uint32(n)))
		}
	default:
		return false
	}
	if end {
		return s.action(w, stream)
	}
	return false
}

func rpReadFrame(r io.Reader) (typ, flags byte, stream uint32, payload []byte, err error) {
	var hdr [9]byte
	if _, err = io.ReadFull(r, hdr[:]); err != nil {
		return 0, 0, 0, nil, err
	}
	n := int(hdr[0])<<16 | int(hdr[1])<<8 | int(hdr[2])
	if n > rpMaxFrameBytes {
		return 0, 0, 0, nil, errRPFrameTooLarge
	}
	payload = make([]byte, n)
	if _, err = io.ReadFull(r, payload); err != nil {
		return 0, 0, 0, nil, err
	}
	return hdr[3], hdr[4], binary.BigEndian.Uint32(hdr[5:]) & 0x7fffffff, payload, nil
}

func rpWriteFrame(w io.Writer, typ, flags byte, stream uint32, payload []byte) error {
	n := len(payload)
	if n > rpMaxFrameBytes {
		return errRPFrameTooLarge
	}
	frame := make([]byte, 9, 9+n)
	binary.BigEndian.PutUint32(frame[0:4], uint32(n)<<8) // 24-bit length, then the type byte
	frame[3], frame[4] = typ, flags
	binary.BigEndian.PutUint32(frame[5:], stream)
	_, err := w.Write(append(frame, payload...))
	return err
}

// rpReset resets the request's stream with code and keeps the connection, so
// that a replay could reuse it.
func rpReset(code uint32) rpAction {
	return func(w io.Writer, stream uint32) bool {
		_ = rpWriteFrame(w, rpFrameRST, 0, stream, binary.BigEndian.AppendUint32(nil, code))
		return false
	}
}

// rpGoAway sends GOAWAY with the marker as debug data and closes the
// connection. With processed set, the last stream ID covers the request, so
// the client fails it with an error quoting the debug data; otherwise it is 0,
// which tells the client the request was not processed and may be retried.
func rpGoAway(code uint32, processed bool) rpAction {
	return func(w io.Writer, stream uint32) bool {
		last := uint32(0)
		if processed {
			last = stream
		}
		payload := binary.BigEndian.AppendUint32(nil, last)
		payload = binary.BigEndian.AppendUint32(payload, code)
		payload = append(payload, rpMarker...)
		_ = rpWriteFrame(w, rpFrameGoAway, 0, 0, payload)
		return true
	}
}

// TestReplayNeverOverHTTP2 guards the one-attempt contract against the standard
// HTTP/2 client, which resends a request whose body it can rewind after a
// refused or reset stream or a GOAWAY, even once the server has read the whole
// body.
func TestReplayNeverOverHTTP2(t *testing.T) {
	tests := []struct {
		name   string
		action rpAction
	}{
		{name: "refused stream", action: rpReset(rpCodeRefused)},
		{name: "protocol error", action: rpReset(rpCodeProtocol)},
		{name: "GOAWAY before the stream", action: rpGoAway(rpCodeNoError, false)},
		{name: "GOAWAY with error before the stream", action: rpGoAway(rpCodeProtocol, false)},
		{name: "GOAWAY after the stream", action: rpGoAway(rpCodeProtocol, true)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, tr := rpStart(t, tt.action)
			const timeout = 10 * time.Second
			c := newClient(t,
				enformion.WithTestBaseURL(s.origin()),
				enformion.WithHTTPClient(&http.Client{Transport: tr}),
				enformion.WithTimeout(timeout),
			)
			start := time.Now()
			res, err := c.EnrichContact(t.Context(), validRequest())
			elapsed := time.Since(start)
			s.requireH2(t)

			require.Error(t, err)
			assert.Nil(t, res)
			require.ErrorIs(t, err, enformion.ErrTransport)
			ce := callError(t, err)
			assert.Equal(t, enformion.FailureTransport, ce.Kind)
			assert.Zero(t, ce.StatusCode)
			assert.Equal(t, int64(1), s.requests.Load(), "the POST must reach the server exactly once")
			assert.Less(t, elapsed, timeout/2, "the call must fail at once, not retry until the timeout")
			assertErrorClean(t, err, rpMarker)
		})
	}
}

// TestReplayControlStdlibResends shows the server detects a replay: the same
// client, sending a rewindable body directly, is resent after a refused
// stream. The SDK's one attempt above is therefore its own doing.
func TestReplayControlStdlibResends(t *testing.T) {
	s, tr := rpStart(t, rpReset(rpCodeRefused))
	// The first resend is immediate, so one second is ample.
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.origin()+"/Contact/Enrich",
		bytes.NewReader([]byte(`{"FirstName":"Avery"}`)))
	require.NoError(t, err)
	require.NotNil(t, req.GetBody, "a bytes.Reader body is rewindable")
	resp, err := (&http.Client{Transport: tr}).Do(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	s.requireH2(t)
	require.Error(t, err)
	assert.Greater(t, s.requests.Load(), int64(1), "the standard library resends a rewindable POST")
}

// TestReplayControlStdlibQuotesGoAwayDebug shows the standard library's own
// error quotes GOAWAY debug data, which is why the SDK discards it.
func TestReplayControlStdlibQuotesGoAwayDebug(t *testing.T) {
	s, tr := rpStart(t, rpGoAway(rpCodeProtocol, true))
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.origin()+"/Contact/Enrich",
		bytes.NewReader([]byte(`{}`)))
	require.NoError(t, err)
	req.GetBody = nil
	resp, err := (&http.Client{Transport: tr}).Do(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	s.requireH2(t)
	require.Error(t, err)
	assert.Contains(t, err.Error(), rpMarker)
}

// rpH1Server starts an HTTP/1.1 TLS server that, once the recording server has
// read the whole request, closes the connection without answering from request
// number closeFrom on. Earlier requests get the empty envelope.
func rpH1Server(t *testing.T, closeFrom int) *testutil.Server {
	t.Helper()
	var n atomic.Int64
	return testutil.NewServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if n.Add(1) < int64(closeFrom) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, emptyEnvelope)
			return
		}
		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, _, err := hj.Hijack()
		if err == nil {
			_ = conn.Close()
		}
	})
}

// TestReplayNeverOverHTTP1 checks that a server closing the connection after
// reading the whole request gets it exactly once, on a new connection and on
// a reused one, where the standard library retries replayable requests.
func TestReplayNeverOverHTTP1(t *testing.T) {
	t.Run("new connection", func(t *testing.T) {
		srv := rpH1Server(t, 1)
		c := newClient(t, enformion.WithTestBaseURL(srv.URL()), enformion.WithHTTPClient(srv.Client()))
		_, err := c.EnrichContact(t.Context(), validRequest())
		require.ErrorIs(t, err, enformion.ErrTransport)
		assert.Equal(t, enformion.FailureTransport, callError(t, err).Kind)
		assert.Equal(t, 1, srv.Hits())
	})
	t.Run("reused connection", func(t *testing.T) {
		srv := rpH1Server(t, 2)
		c := newClient(t, enformion.WithTestBaseURL(srv.URL()), enformion.WithHTTPClient(srv.Client()))
		res, err := c.EnrichContact(t.Context(), validRequest())
		require.NoError(t, err)
		assert.Nil(t, res.Person)

		var reused atomic.Bool
		ctx := httptrace.WithClientTrace(t.Context(), &httptrace.ClientTrace{
			GotConn: func(info httptrace.GotConnInfo) { reused.Store(info.Reused) },
		})
		_, err = c.EnrichContact(ctx, validRequest())
		require.ErrorIs(t, err, enformion.ErrTransport)
		assert.Equal(t, enformion.FailureTransport, callError(t, err).Kind)
		assert.True(t, reused.Load(), "the second call must reuse the kept-alive connection")
		assert.Equal(t, 2, srv.Hits(), "the failed POST must reach the server exactly once")
	})
}

// rpArmedConn fails its next write, once armed, with nothing written: what a
// kept-alive connection that the server has meanwhile closed looks like.
type rpArmedConn struct {
	net.Conn

	armed atomic.Bool
}

func (c *rpArmedConn) Write(p []byte) (int, error) {
	if c.armed.Load() {
		return 0, &net.OpError{Op: "write", Net: "tcp", Err: errRPBrokenPipe}
	}
	return c.Conn.Write(p)
}

var errRPBrokenPipe = errors.New("broken pipe")

// rpArmedTransport returns a transport to srv whose connections are kept in
// conns, plus a count of its dials.
func rpArmedTransport(t *testing.T, srv *testutil.Server) (*http.Transport, *[]*rpArmedConn, *atomic.Int64) {
	t.Helper()
	base, ok := srv.Client().Transport.(*http.Transport)
	require.True(t, ok)
	tr := base.Clone()
	var (
		mu    sync.Mutex
		conns []*rpArmedConn
		dials atomic.Int64
	)
	dialer := &net.Dialer{}
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		dials.Add(1)
		c, err := dialer.DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		ac := &rpArmedConn{Conn: c}
		mu.Lock()
		conns = append(conns, ac)
		mu.Unlock()
		return ac, nil
	}
	t.Cleanup(tr.CloseIdleConnections)
	return tr, &conns, &dials
}

// TestReplayNeverOverHTTP1NothingWritten guards the HTTP/1 resend path: on a
// reused connection whose write fails before anything is written, the standard
// library resends a request it can rewind. The SDK's request cannot be
// rewound, so it fails once; the control shows the resend is real.
func TestReplayNeverOverHTTP1NothingWritten(t *testing.T) {
	srv := rpH1Server(t, math.MaxInt)
	t.Run("sdk", func(t *testing.T) {
		tr, conns, dials := rpArmedTransport(t, srv)
		c := newClient(t, enformion.WithTestBaseURL(srv.URL()), enformion.WithHTTPClient(&http.Client{Transport: tr}))
		_, err := c.EnrichContact(t.Context(), validRequest())
		require.NoError(t, err)
		require.Len(t, *conns, 1)
		before := srv.Hits()
		(*conns)[0].armed.Store(true)

		_, err = c.EnrichContact(t.Context(), validRequest())
		require.ErrorIs(t, err, enformion.ErrTransport)
		assert.Equal(t, int64(1), dials.Load(), "the SDK must not open a second connection to resend")
		assert.Equal(t, before, srv.Hits(), "nothing reached the server")
	})
	t.Run("control: a rewindable POST is resent", func(t *testing.T) {
		tr, conns, dials := rpArmedTransport(t, srv)
		hc := &http.Client{Transport: tr}
		post := func() error {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL()+"/Contact/Enrich",
				bytes.NewReader([]byte(`{}`)))
			require.NoError(t, err)
			require.NotNil(t, req.GetBody)
			resp, err := hc.Do(req)
			if err != nil {
				return err
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			return resp.Body.Close()
		}
		require.NoError(t, post())
		require.Len(t, *conns, 1)
		(*conns)[0].armed.Store(true)
		require.NoError(t, post(), "the standard library resends on a new connection")
		assert.Equal(t, int64(2), dials.Load())
	})
}
