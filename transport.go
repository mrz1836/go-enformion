package enformion

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync"
	"syscall"
	"time"
)

// userAgent is the User-Agent header of every call.
const userAgent = "go-enformion"

// call is one provider operation: its path, its galaxy-search-type value, the
// optional session ID, the encoded body and its response size limit, where 0
// means the client's.
type call struct {
	path       string
	searchType string
	sessionID  string
	body       []byte
	maxBytes   int64
}

// exchange is a received HTTP response. For status 200, body is the whole body,
// within the size limit. For any other status, body is the body if it could be
// read within the limit, else nil: it only feeds best-effort diagnostics.
type exchange struct {
	status int
	body   []byte
}

// sender makes one attempt of a call. Everything it needs, credentials
// included, is captured inside the function: fmt prints a func value only as an
// address, and reflection-based dumpers cannot see what a closure captures. Its
// errors are *CallError values.
type sender func(ctx context.Context, c *call) (*exchange, error)

// newSender returns the sender for a validated configuration. maxBytes is the
// response size limit of every call that does not set its own.
func newSender(hc *http.Client, origin, keyName, keyPassword string, timeout time.Duration, maxBytes int64) sender {
	return func(ctx context.Context, c *call) (*exchange, error) {
		h := callHeader(keyName, keyPassword, c.searchType, c.sessionID)
		limit := maxBytes
		if c.maxBytes > 0 {
			limit = c.maxBytes
		}
		return roundTrip(ctx, hc, origin+c.path, h, c.body, timeout, limit)
	}
}

// callHeader returns a fresh header map for one call. The values share one
// backing array, allocated per call so that no two requests share header
// memory; each is a full slice expression, so an append to one cannot overwrite
// the next.
//
// It never sets Authorization, Cookie or an idempotency key: with an
// Idempotency-Key header, the standard library may replay a POST.
func callHeader(keyName, keyPassword, searchType, sessionID string) http.Header {
	v := append(make([]string, 0, 7),
		"application/json", "application/json", userAgent, keyName, keyPassword, searchType, sessionID)
	h := make(http.Header, 7)
	h["Content-Type"] = v[0:1:1]
	h["Accept"] = v[1:2:2]
	h["User-Agent"] = v[2:3:3]
	h["Galaxy-Ap-Name"] = v[3:4:4]
	h["Galaxy-Ap-Password"] = v[4:5:5]
	h["Galaxy-Search-Type"] = v[5:6:6]
	if sessionID != "" {
		h["Galaxy-Client-Session-Id"] = v[6:7:7]
	}
	return h
}

// roundTrip makes exactly one POST attempt and reads the response body within
// the call's timeout. The body is closed exactly once on every path.
func roundTrip(ctx context.Context, hc *http.Client, url string, h http.Header, body []byte,
	timeout time.Duration, maxBytes int64,
) (*exchange, error) {
	callCtx, cancel := context.WithTimeout(withoutCredentialTrace(ctx), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		// The standard library's error quotes the URL.
		return nil, newCallError(FailureTransport, 0, errBuildRequest)
	}
	// A rewindable body lets the standard library resend the request on its own:
	// HTTP/1 when a reused connection fails before anything is written, and
	// HTTP/2 after a refused stream, a GOAWAY or a protocol error, even once the
	// server has read the whole body. Without GetBody every call is one attempt,
	// and a 307 or 308 cannot be followed either.
	req.GetBody = nil
	req.Header = h

	resp, err := hc.Do(req) //nolint:bodyclose // closed exactly once by closeBody, below
	if err != nil {
		return nil, classify(ctx, callCtx, err, 0)
	}
	// When the call's context ends, close the body, so that a read that does not
	// watch the context still stops; otherwise close it on return. The Once makes
	// it exactly one close, finished before roundTrip returns. A close error
	// after a complete read changes nothing.
	var closeOnce sync.Once
	closeBody := func() { closeOnce.Do(func() { _ = resp.Body.Close() }) }
	stop := context.AfterFunc(callCtx, closeBody)
	defer func() {
		stop()
		closeBody()
	}()

	if resp.StatusCode < http.StatusOK {
		// A 1xx that reaches here, such as 101 Switching Protocols, has the raw
		// connection as its body, which no deadline watches: never read it.
		return &exchange{status: resp.StatusCode}, nil
	}
	data, tooLarge, err := readLimited(resp, maxBytes)
	if resp.StatusCode != http.StatusOK {
		// The status decides the outcome; the body only feeds diagnostics, so an
		// oversized or unreadable one is skipped.
		return &exchange{status: resp.StatusCode, body: data}, nil
	}
	if tooLarge {
		return nil, newCallError(FailureResponseTooLarge, resp.StatusCode, errResponseTooLarge)
	}
	if err != nil {
		return nil, classify(ctx, callCtx, err, resp.StatusCode)
	}
	if resp.ContentLength >= 0 && int64(len(data)) != resp.ContentLength {
		// The standard transport reports a short body as io.ErrUnexpectedEOF; an
		// injected one might end it cleanly instead. Either way it is incomplete.
		return nil, newCallError(FailureTransport, resp.StatusCode, errTransportClosed)
	}
	return &exchange{status: resp.StatusCode, body: data}, nil
}

// readLimited reads resp's body within maxBytes. tooLarge reports a body that
// is declared or found to be over the limit, and err a failed read; either way
// data is nil. A declared length over the limit is refused before any read.
func readLimited(resp *http.Response, maxBytes int64) (data []byte, tooLarge bool, err error) {
	if resp.ContentLength > maxBytes {
		return nil, true, nil
	}
	data, err = readBody(resp.Body, resp.ContentLength, maxBytes)
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) > maxBytes {
		return nil, true, nil
	}
	return data, false, nil
}

// readBody reads r to EOF, but never more than maxBytes+1 bytes, so a result
// longer than maxBytes means the body is too large. A declared length, already
// known to be within the limit, sizes the buffer exactly, with one spare byte to
// see EOF without growing.
func readBody(r io.Reader, contentLength, maxBytes int64) ([]byte, error) {
	limited := io.LimitReader(r, maxBytes+1)
	if contentLength < 0 {
		return io.ReadAll(limited)
	}
	buf := make([]byte, 0, contentLength+1)
	for len(buf) < cap(buf) {
		n, err := limited.Read(buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+n]
		if errors.Is(err, io.EOF) {
			return buf, nil
		}
		if err != nil {
			return buf, err
		}
	}
	// The body is longer than declared: read on, still within the limit.
	rest, err := io.ReadAll(limited)
	return append(buf, rest...), err
}

// classify maps a failed exchange onto a CallError: the caller's context first,
// then a timeout (this call's deadline, an http.Client Timeout, or any network
// timeout), else a transport failure of a fixed class. The original error is
// never kept.
func classify(caller, callCtx context.Context, err error, status int) *CallError {
	if cerr := caller.Err(); cerr != nil {
		return contextFailure(cerr, status)
	}
	if callCtx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || isNetTimeout(err) {
		return newCallError(FailureTimeout, status, context.DeadlineExceeded)
	}
	return newCallError(FailureTransport, status, transportClass(err))
}

// isNetTimeout reports whether err is a network error that timed out, such as a
// dial or TLS handshake timeout.
func isNetTimeout(err error) bool {
	ne, ok := errors.AsType[net.Error](err)
	return ok && ne.Timeout()
}

// transportClass reduces a transport error to one of a few fixed classes.
func transportClass(err error) error {
	if _, ok := errors.AsType[*net.DNSError](err); ok {
		return errTransportDNS
	}
	var op string
	if oe, ok := errors.AsType[*net.OpError](err); ok {
		op = oe.Op
	}
	if op == "proxyconnect" {
		return errTransportProxy
	}
	if isTLSError(err) {
		return errTransportTLS
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNABORTED) || errors.Is(err, syscall.EPIPE) {
		return errTransportReset
	}
	if op == "dial" {
		return errTransportDial
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) {
		return errTransportClosed
	}
	return errTransportOther
}

// isTLSError reports whether err comes from the TLS handshake or record layer.
func isTLSError(err error) bool {
	if _, ok := errors.AsType[*tls.CertificateVerificationError](err); ok {
		return true
	}
	if _, ok := errors.AsType[tls.RecordHeaderError](err); ok {
		return true
	}
	_, ok := errors.AsType[tls.AlertError](err)
	return ok
}

// withoutCredentialTrace returns ctx with any httptrace.ClientTrace replaced by
// a copy whose WroteHeaderField hook never receives the credential headers. The
// standard library passes every request header and its values to that hook,
// and tracing libraries record them, so without this the API key name and
// password would reach the caller's tracing backend. Every other hook, and the
// context's cancellation and values, are unchanged.
func withoutCredentialTrace(ctx context.Context) context.Context {
	trace := httptrace.ContextClientTrace(ctx)
	if trace == nil || trace.WroteHeaderField == nil {
		return ctx
	}
	safe := *trace
	wrote := trace.WroteHeaderField
	safe.WroteHeaderField = func(key string, value []string) {
		if strings.EqualFold(key, "Galaxy-Ap-Name") || strings.EqualFold(key, "Galaxy-Ap-Password") {
			return
		}
		wrote(key, value)
	}
	return traceContext{Context: ctx, trace: &safe}
}

// traceContext is a context whose ClientTrace is replaced.
type traceContext struct {
	context.Context //nolint:containedctx // a context wrapper embeds its parent

	trace *httptrace.ClientTrace
}

// Value returns the replacement trace in place of the parent's, and every
// other value unchanged.
func (c traceContext) Value(key any) any {
	v := c.Context.Value(key)
	if _, ok := v.(*httptrace.ClientTrace); ok {
		return c.trace
	}
	return v
}
