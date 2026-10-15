package enformion

import (
	"net/http"
	"net/http/cookiejar"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion/internal/testutil"
)

// ciIdleTransport records CloseIdleConnections calls.
type ciIdleTransport struct {
	http.RoundTripper

	closed int
}

func (t *ciIdleTransport) CloseIdleConnections() { t.closed++ }

// TestClientRefuseRedirect covers the CheckRedirect backstop. The redirect
// guard strips Location first, so the http.Client never reaches it in practice.
func TestClientRefuseRedirect(t *testing.T) {
	require.ErrorIs(t, refuseRedirect(nil, nil), http.ErrUseLastResponse)
}

// TestClientGuardForwardsCloseIdleConnections proves that wrapping a transport
// keeps (*http.Client).CloseIdleConnections working, and that a transport
// without the method is tolerated.
func TestClientGuardForwardsCloseIdleConnections(t *testing.T) {
	idle := &ciIdleTransport{}
	o := options{httpClient: &http.Client{Transport: idle}, httpClientSet: true}
	o.client().CloseIdleConnections()
	assert.Equal(t, 1, idle.closed)

	assert.NotPanics(t, func() {
		redirectGuard{next: struct{ http.RoundTripper }{}}.CloseIdleConnections()
	})
}

// TestClientRedirectBackstopInstalled proves that every client the SDK builds,
// default or injected (even one that would follow redirects and keep
// cookies), refuses redirects on its own, has no cookie jar, and wraps its
// transport in the Location-stripping guard. Either layer alone keeps a
// redirect from being followed.
func TestClientRedirectBackstopInstalled(t *testing.T) {
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	permissive := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return nil },
		Jar:           jar,
		Transport:     &ciIdleTransport{},
	}
	for name, o := range map[string]options{
		"default":  {},
		"injected": {httpClient: permissive, httpClientSet: true},
	} {
		t.Run(name, func(t *testing.T) {
			c := o.client()
			require.NotNil(t, c.CheckRedirect)
			require.ErrorIs(t, c.CheckRedirect(nil, nil), http.ErrUseLastResponse)
			assert.Nil(t, c.Jar)
			_, ok := c.Transport.(redirectGuard)
			assert.True(t, ok, "the transport is wrapped in the redirect guard")
		})
	}
	assert.NotNil(t, permissive.Jar, "the caller's client is never modified")
}

// TestClientGuardStripsLocationOnlyOn3xx proves the guard alone removes the
// Location header of every 3xx response, and of no other.
func TestClientGuardStripsLocationOnlyOn3xx(t *testing.T) {
	const location = "https://elsewhere.example/"
	for _, status := range []int{100, 200, 204, 299, 300, 301, 302, 303, 304, 305, 307, 308, 399, 400, 404, 500} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			g := redirectGuard{next: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: status,
					Header:     http.Header{"Location": {location}, "X-Other": {"kept"}},
					Body:       http.NoBody,
					Request:    r,
				}, nil
			})}
			req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://127.0.0.1/", http.NoBody)
			require.NoError(t, err)
			resp, err := g.RoundTrip(req)
			require.NoError(t, err)
			_ = resp.Body.Close()
			if status >= 300 && status < 400 {
				assert.Empty(t, resp.Header.Values("Location"), "a 3xx loses its Location")
			} else {
				assert.Equal(t, location, resp.Header.Get("Location"), "any other status keeps it")
			}
			assert.Equal(t, "kept", resp.Header.Get("X-Other"))
		})
	}
}

// TestClientDefaultTransport pins the default transport's settings: its own
// connection pool (never the shared http.DefaultTransport), the standard proxy
// behavior, tighter header and handshake limits, HTTP/2, and no client-level
// timeout (the per-call context governs).
func TestClientDefaultTransport(t *testing.T) {
	c := (&options{}).client()
	assert.Zero(t, c.Timeout)
	g, ok := c.Transport.(redirectGuard)
	require.True(t, ok)
	tr, ok := g.next.(*http.Transport)
	require.True(t, ok)
	assert.NotSame(t, http.DefaultTransport, tr)
	require.NotNil(t, tr.Proxy)
	assert.Equal(t, reflect.ValueOf(http.ProxyFromEnvironment).Pointer(), reflect.ValueOf(tr.Proxy).Pointer())
	assert.Equal(t, int64(64<<10), tr.MaxResponseHeaderBytes)
	assert.Equal(t, 10*time.Second, tr.TLSHandshakeTimeout)
	assert.True(t, tr.ForceAttemptHTTP2)
	assert.NotNil(t, tr.DialContext)
	assert.Nil(t, tr.TLSClientConfig, "the standard library's TLS defaults apply")

	other := (&options{}).client()
	og, ok := other.Transport.(redirectGuard)
	require.True(t, ok)
	assert.NotSame(t, tr, og.next, "each client has its own pool")
}
