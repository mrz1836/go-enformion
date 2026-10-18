package enformion

import (
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Defaults and limits.
const (
	// productionOrigin is the only server the provider documents. Calls to it
	// are live and billable; there is no documented sandbox.
	productionOrigin = "https://devapi.enformion.com"

	defaultTimeout          = 30 * time.Second
	maxTimeout              = 5 * time.Minute
	defaultMaxResponseBytes = 1 << 20
	maxResponseBytesLimit   = 16 << 20
	maxCredentialBytes      = 1024
)

// Client calls the EnformionGO Contact Enrichment API. Create it with
// NewClient; the zero Client and a nil *Client return ErrInvalidConfig without
// any I/O. A Client is immutable and safe for concurrent use.
//
// Its fields are unexported and its credentials are held inside a function
// value, so formatting a Client, or a struct holding one, with any fmt verb or a
// reflection-based dumper never prints them.
type Client struct {
	send           sender
	searchMaxBytes int64
	diagnostics    bool
}

// Option configures NewClient. Options only record their argument: NewClient
// validates the result. A nil Option is skipped, and when an option is given
// more than once the last one wins.
type Option func(*options)

// options is the configuration options record.
type options struct {
	httpClient     *http.Client
	httpClientSet  bool
	timeout        time.Duration
	maxBytes       int64
	maxBytesSet    bool
	testBaseURL    string
	testBaseURLSet bool
	diagnostics    bool
}

// NewClient returns a Client that authenticates with an EnformionGO API key.
//
// Parameters:
//   - keyName: the dashboard's API key name, sent as the galaxy-ap-name header
//   - keyPassword: the dashboard's API key password, sent as galaxy-ap-password
//   - opts: optional settings; see the With functions
//
// Each credential must be 1 to 1024 bytes of printable ASCII (0x20 to 0x7E)
// without leading or trailing spaces. The application decides where they come
// from: NewClient reads no environment variables or files and performs no I/O.
//
// Errors are *ValidationError values matching ErrInvalidConfig. They name the
// argument or option, never its value.
func NewClient(keyName, keyPassword string, opts ...Option) (*Client, error) {
	if err := checkCredential("keyName", keyName); err != nil {
		return nil, err
	}
	if err := checkCredential("keyPassword", keyPassword); err != nil {
		return nil, err
	}
	o := options{timeout: defaultTimeout, maxBytes: defaultMaxResponseBytes}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	origin, err := o.origin()
	if err != nil {
		return nil, err
	}
	searchMaxBytes := int64(maxResponseBytesLimit)
	if o.maxBytesSet {
		searchMaxBytes = o.maxBytes
	}
	return &Client{
		send:           newSender(o.client(), origin, keyName, keyPassword, o.timeout, o.maxBytes),
		searchMaxBytes: searchMaxBytes,
		diagnostics:    o.diagnostics,
	}, nil
}

// WithHTTPClient makes calls through a copy of hc. The copy refuses redirects
// (its CheckRedirect returns http.ErrUseLastResponse), has no cookie jar, keeps
// hc.Timeout, and wraps hc's transport, without modifying it, so that every
// redirect comes back as a plain response and is never followed. A nil
// transport means http.DefaultTransport. hc itself is never modified.
//
// The transport is trusted: it sees the credential headers and the personal
// data in each request, and it may retry on its own, which the SDK cannot
// prevent. A nil hc is rejected.
func WithHTTPClient(hc *http.Client) Option {
	return func(o *options) {
		o.httpClient = hc
		o.httpClientSet = true
	}
}

// WithTimeout bounds each call, including reading the whole response body. The
// default is 30 seconds; d must be greater than 0 and at most 5 minutes. The
// caller's context deadline applies as well.
func WithTimeout(d time.Duration) Option {
	return func(o *options) { o.timeout = d }
}

// WithMaxResponseBytes limits the response body, counted after any transport
// decompression; n must be from 1 byte to 16 MiB. Without it, the limit is
// 1 MiB for EnrichContact and 16 MiB for SearchPersons, whose pages can hold
// many people; with it, n applies to every operation. A larger HTTP 200 body
// fails with ErrResponseTooLarge, after the provider may already have billed
// the call.
func WithMaxResponseBytes(n int64) Option {
	return func(o *options) {
		o.maxBytes = n
		o.maxBytesSet = true
	}
}

// WithTestBaseURL sends calls to a local test server instead of the provider.
// It exists for tests built on httptest.NewTLSServer, together with
// WithHTTPClient(srv.Client()).
//
// The URL must use https and have a host that is a canonical IPv4 address in
// 127.0.0.0/8 or exactly [::1], with an optional port and no userinfo, query,
// fragment or path (a lone trailing "/" is allowed). Names such as "localhost"
// are rejected, because they resolve through the system resolver. Errors never
// quote the URL. There is no option for any other base URL, so credentials
// reach only the provider or loopback.
func WithTestBaseURL(rawURL string) Option {
	return func(o *options) {
		o.testBaseURL = rawURL
		o.testBaseURLSet = true
	}
}

// WithProviderDiagnostics keeps the provider's message, inputErrors and
// warnings on errors, reachable only through (*CallError).Diagnostics. They are
// off by default, because errors are routinely logged and passed on, and
// provider text may echo the request's personal data. Results always carry
// their diagnostics.
func WithProviderDiagnostics() Option {
	return func(o *options) { o.diagnostics = true }
}

// origin validates the options in a fixed order and returns the API origin.
func (o *options) origin() (string, error) {
	switch {
	case o.httpClientSet && o.httpClient == nil:
		return "", invalidConfig("HTTPClient", reasonNil)
	case o.timeout <= 0 || o.timeout > maxTimeout:
		return "", invalidConfig("Timeout", reasonOutOfRange)
	case o.maxBytes < 1 || o.maxBytes > maxResponseBytesLimit:
		return "", invalidConfig("MaxResponseBytes", reasonOutOfRange)
	case !o.testBaseURLSet:
		return productionOrigin, nil
	}
	origin, ok := testOrigin(o.testBaseURL)
	if !ok {
		return "", invalidConfig("TestBaseURL", reasonInvalidURL)
	}
	return origin, nil
}

// client returns the HTTP client calls use: a copy of the injected client, or a
// new one on a fresh transport, that never follows a redirect and keeps no
// cookies.
func (o *options) client() *http.Client {
	var c http.Client
	if o.httpClientSet {
		c = *o.httpClient
	} else {
		c.Transport = newTransport()
	}
	next := c.Transport
	if next == nil {
		next = http.DefaultTransport
	}
	c.Transport = redirectGuard{next: next}
	c.CheckRedirect = refuseRedirect
	c.Jar = nil
	return &c
}

// newTransport returns a fresh transport with the standard library's default
// settings, tighter header limits, and its own connection pool. It uses
// http.ProxyFromEnvironment, as http.DefaultTransport does.
func newTransport() *http.Transport {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Transport{
		Proxy:                  http.ProxyFromEnvironment,
		DialContext:            dialer.DialContext,
		ForceAttemptHTTP2:      true,
		TLSHandshakeTimeout:    10 * time.Second,
		IdleConnTimeout:        90 * time.Second,
		MaxIdleConns:           100,
		MaxIdleConnsPerHost:    16,
		ExpectContinueTimeout:  1 * time.Second,
		MaxResponseHeaderBytes: 64 << 10,
	}
}

// refuseRedirect stops the client at the first response, so credentials are
// never sent to another URL.
func refuseRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// redirectGuard removes the Location header from every 3xx response before the
// http.Client sees it. The client then returns the response as it is: it neither
// follows the redirect, which would resend every custom header (credentials
// included) to the new URL, nor fails on an unparsable Location with an error
// that quotes it.
type redirectGuard struct {
	next http.RoundTripper
}

// RoundTrip passes r to the wrapped transport and strips Location from a 3xx.
func (g redirectGuard) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := g.next.RoundTrip(r)
	if resp != nil && resp.StatusCode >= http.StatusMultipleChoices && resp.StatusCode < http.StatusBadRequest {
		resp.Header.Del("Location")
	}
	return resp, err
}

// CloseIdleConnections closes the wrapped transport's idle connections, if it
// has any, so that (*http.Client).CloseIdleConnections still reaches it.
func (g redirectGuard) CloseIdleConnections() {
	if c, ok := g.next.(interface{ CloseIdleConnections() }); ok {
		c.CloseIdleConnections()
	}
}

// checkCredential validates one credential header value. The error names the
// argument, never its value.
func checkCredential(field, s string) error {
	if s == "" {
		return invalidConfig(field, reasonRequired)
	}
	if len(s) > maxCredentialBytes || s[0] == ' ' || s[len(s)-1] == ' ' || !asciiRange(s, 0x20, 0x7e) {
		return invalidConfig(field, reasonInvalidHeaderValue)
	}
	return nil
}

// testOrigin validates a loopback test URL and returns it as
// "https://host[:port]".
func testOrigin(raw string) (string, bool) {
	if raw == "" || strings.ContainsAny(raw, "?#@\\") {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Opaque != "" || u.User != nil ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" ||
		(u.Path != "" && u.Path != "/") {
		return "", false
	}
	if !loopbackHostPort(u.Host) {
		return "", false
	}
	return "https://" + u.Host, true
}

// loopbackHostPort accepts "ip[:port]" for a canonical IPv4 address in
// 127.0.0.0/8, or "[::1][:port]", with a canonical port from 1 to 65535.
func loopbackHostPort(hostport string) bool {
	host, port := hostport, ""
	if end := strings.LastIndexByte(hostport, ']'); end >= 0 {
		host, port = hostport[:end+1], hostport[end+1:]
	} else if i := strings.LastIndexByte(hostport, ':'); i >= 0 {
		host, port = hostport[:i], hostport[i:]
	}
	if host != "[::1]" {
		ip, err := netip.ParseAddr(host)
		if err != nil || !ip.Is4() || !ip.IsLoopback() || ip.String() != host {
			return false
		}
	}
	if port == "" {
		return true
	}
	digits, ok := strings.CutPrefix(port, ":")
	if !ok {
		return false
	}
	n, err := strconv.ParseUint(digits, 10, 16)
	return err == nil && n != 0 && strconv.FormatUint(n, 10) == digits
}
