package enformion_test

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion"
	"github.com/mrz1836/go-enformion/internal/testutil"
)

// rdMarker is planted in every Location; it must never surface.
const rdMarker = "rd-location-marker-3b9d"

// rdStatuses are the redirect statuses, plus 304, which carries no redirect but
// may still hold a Location.
func rdStatuses() []int {
	return []int{
		http.StatusMultipleChoices, http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusNotModified, http.StatusTemporaryRedirect, http.StatusPermanentRedirect,
	}
}

// rdServers starts a redirect target and an origin that answers every request
// with status, the Location that location builds from the target's URL, and a
// cookie.
func rdServers(t *testing.T, status int, location func(target string) string) (origin, target *testutil.Server) {
	t.Helper()
	target = testutil.NewServer(t, respond(http.StatusOK, testutil.Fixture(t, "provider/match.json")))
	loc := location(target.URL())
	origin = testutil.NewServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", loc)
		w.Header().Set("Set-Cookie", "rd="+rdMarker+"; Path=/")
		w.WriteHeader(status)
	})
	return origin, target
}

// rdClient returns the HTTP client a caller injects: the origin's own, or, when
// permissive, one that would follow every redirect and keep every cookie. The
// returned check asserts afterward that the caller's client was neither
// consulted nor changed.
func rdClient(t *testing.T, origin *testutil.Server, permissive bool) (*http.Client, func(*testing.T)) {
	t.Helper()
	if !permissive {
		return origin.Client(), func(*testing.T) {}
	}
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	var followed atomic.Int32
	// The test servers share one certificate, so this transport trusts the
	// target too: only the SDK stands in the way.
	transport := origin.Client().Transport
	hc := &http.Client{
		Transport: transport,
		Jar:       jar,
		Timeout:   7 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			followed.Add(1)
			return nil
		},
	}
	checkPtr := reflect.ValueOf(hc.CheckRedirect).Pointer()
	return hc, func(t *testing.T) {
		t.Helper()
		assert.Zero(t, followed.Load(), "the caller's CheckRedirect is never consulted")
		u, err := url.Parse(origin.URL())
		require.NoError(t, err)
		assert.Empty(t, jar.Cookies(u), "the caller's jar receives no cookies")
		assert.Same(t, transport, hc.Transport, "the caller's client is left as it was")
		assert.Same(t, jar, hc.Jar)
		assert.Equal(t, 7*time.Second, hc.Timeout)
		assert.Equal(t, checkPtr, reflect.ValueOf(hc.CheckRedirect).Pointer())
	}
}

// rdCheck asserts the outcome every redirect must have: an unexpected response
// with the 3xx status, one request to the origin, none to the target, and no
// trace of the Location anywhere in the error.
func rdCheck(t *testing.T, res *enformion.EnrichContactResult, err error, status int, origin, target *testutil.Server) {
	t.Helper()
	assert.Nil(t, res)
	require.ErrorIs(t, err, enformion.ErrUnexpectedResponse)
	ce := callError(t, err)
	assert.Equal(t, enformion.FailureUnexpectedResponse, ce.Kind)
	assert.Equal(t, status, ce.StatusCode)
	assert.Equal(t, 1, origin.Hits())
	assert.Zero(t, target.Hits(), "the redirect target must never be reached, so credentials never reach it")
	assertErrorClean(t, err, rdMarker, target.URL(), testKeyName, testKeyPassword)
}

// TestRedirectNeverFollowed proves that no redirect is ever followed, whether
// the injected client is the test server's own or one that would follow it,
// and whether the Location points at a live target or cannot be parsed.
func TestRedirectNeverFollowed(t *testing.T) {
	locations := []struct {
		name string
		at   func(target string) string
	}{
		{name: "to the target", at: func(target string) string { return target + "/" + rdMarker + "?next=" + rdMarker }},
		{name: "unparsable", at: func(string) string { return "https://[::1" + rdMarker }},
	}
	for _, permissive := range []bool{false, true} {
		for _, loc := range locations {
			for _, status := range rdStatuses() {
				t.Run(fmt.Sprintf("permissive=%t/%s/%d", permissive, loc.name, status), func(t *testing.T) {
					origin, target := rdServers(t, status, loc.at)
					hc, check := rdClient(t, origin, permissive)
					c := newClient(t, enformion.WithTestBaseURL(origin.URL()), enformion.WithHTTPClient(hc))
					res, err := c.EnrichContact(t.Context(), validRequest())
					rdCheck(t, res, err, status, origin, target)
					check(t)
				})
			}
		}
	}
}

func TestRedirectUnparsableLocation(t *testing.T) {
	tests := []struct {
		name     string
		location string
	}{
		{"unclosed bracket", "https://[::1" + rdMarker},
		{"control byte", "https://127.0.0.1/\x01" + rdMarker},
		{"missing scheme", "://" + rdMarker},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := testutil.NewBody("moved")
			ct := &testutil.CountingTransport{Next: redirectTransport(tt.location, body, 5)}
			c := newTransportClient(t, ct)
			res, err := c.EnrichContact(t.Context(), validRequest())
			assert.Nil(t, res)
			require.ErrorIs(t, err, enformion.ErrUnexpectedResponse)
			assert.Equal(t, http.StatusFound, callError(t, err).StatusCode)
			assert.Equal(t, 1, ct.Attempts())
			assert.Equal(t, 1, body.Closes(), "the redirect body is closed exactly once")
			assertErrorClean(t, err, rdMarker)
		})
	}
}

func TestRedirectInjectedClientWithoutTransport(t *testing.T) {
	// A client with no transport means http.DefaultTransport, which the guard
	// wraps without modifying. It cannot trust the test certificate, so this
	// only checks construction; TestRedirectUnparsableLocation covers the
	// guard in memory.
	hc := &http.Client{}
	_, err := enformion.NewClient(testKeyName, testKeyPassword, enformion.WithHTTPClient(hc))
	require.NoError(t, err)
	assert.Nil(t, hc.Transport, "the caller's client is never modified")
	assert.Nil(t, hc.CheckRedirect)
	assert.Nil(t, hc.Jar)
}
