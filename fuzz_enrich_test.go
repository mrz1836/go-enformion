package enformion_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion"
	"github.com/mrz1836/go-enformion/internal/testutil"
)

// feFirst is the first name of the fuzzed requests.
const feFirst = "Riley"

// feStatus bounds a fuzzed status to 100–999.
func feStatus(status int) int {
	if status < 0 {
		status = -(status + 1)
	}
	return 100 + status%900
}

// feKind is the documented status mapping, written out independently of the
// library's.
func feKind(status int) enformion.FailureKind {
	switch {
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		return enformion.FailureUnauthorized
	case status == http.StatusTooManyRequests:
		return enformion.FailureRateLimited
	case status >= 400 && status <= 499:
		return enformion.FailureRejected
	case status >= 500 && status <= 599:
		return enformion.FailureUpstream
	}
	return enformion.FailureUnexpectedResponse
}

// feStatusSeeds are statuses at and around every boundary of the mapping.
func feStatusSeeds() []int {
	return []int{
		100, 101, 199, 200, 201, 204, 299, 300, 301, 302, 303, 304, 307, 308, 399,
		400, 401, 402, 403, 404, 418, 428, 429, 430, 499, 500, 502, 503, 599, 600, 999,
	}
}

func FuzzEnrichContactEndToEnd(f *testing.F) {
	for _, s := range []struct {
		status int
		body   string
		first  string
	}{
		{200, string(testutil.Fixture(f, "provider/match.json")), feFirst},
		{200, string(testutil.Fixture(f, "synthetic/no-person.json")), feFirst},
		{200, string(testutil.Fixture(f, "synthetic/is-error.json")), feFirst},
		{200, string(testutil.Fixture(f, "synthetic/unknown-keys-person.json")), feFirst},
		{200, `{"isError":false,"person":{"age":41}}`, feFirst},
		{200, `not json`, feFirst},
		{200, "{\"isError\":false}\xff", feFirst},
		{400, string(testutil.Fixture(f, "provider/rejected-400.json")), feFirst},
		{429, `{}`, feFirst},
		{302, ``, feFirst},
		{500, `{"message":"synthetic"}`, feFirst},
		{200, `{}`, ""},
		{200, `{}`, "Av\tery"},
		{200, `{}`, "Av\xffery"},
		{200, `{}`, "   "},
		{200, `{}`, strings.Repeat("a", 257)},
	} {
		f.Add(s.status, []byte(s.body), s.first)
	}
	f.Fuzz(func(t *testing.T, status int, body []byte, first string) {
		status = feStatus(status)
		var resp atomic.Pointer[testutil.Body]
		ct := &testutil.CountingTransport{Next: testutil.Transport(status, int64(len(body)), func() io.ReadCloser {
			b := testutil.NewBody(string(body))
			resp.Store(b)
			return b
		})}
		client := newTransportClient(t, ct)
		req := validRequest()
		req.FirstName = first
		res, err := client.EnrichContact(t.Context(), req)

		if ve, ok := errors.AsType[*enformion.ValidationError](err); ok {
			assert.ErrorIs(t, ve, enformion.ErrInvalidRequest)
			assert.Zero(t, ct.Attempts(), "an invalid request is never sent")
			assert.Nil(t, res)
			feCheckText(t, err)
			return
		}
		require.Equal(t, 1, ct.Attempts(), "a valid request is sent exactly once")
		require.NotNil(t, resp.Load())
		assert.Equal(t, 1, resp.Load().Closes(), "the response body is closed exactly once")
		if err == nil {
			require.Equal(t, http.StatusOK, status)
			require.NotNil(t, res)
			return
		}
		assert.Nil(t, res)
		feCheckText(t, err)
		ce := callError(t, err)
		assert.Equal(t, status, ce.StatusCode)
		if status != http.StatusOK {
			assert.Equal(t, feKind(status), ce.Kind)
			return
		}
		assert.Contains(t, []enformion.FailureKind{
			enformion.FailureMalformedResponse, enformion.FailureUnexpectedResponse,
			enformion.FailureRejected, enformion.FailureResponseTooLarge,
		}, ce.Kind)
	})
}

// feCheckText checks an error's text: the package prefix and no credentials.
func feCheckText(t *testing.T, err error) {
	t.Helper()
	msg := err.Error()
	assert.True(t, strings.HasPrefix(msg, "enformion: "), "error text %q", msg)
	assert.NotContains(t, msg, testKeyName)
	assert.NotContains(t, msg, testKeyPassword)
}

func FuzzStatusMapping(f *testing.F) {
	for _, s := range feStatusSeeds() {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, status int) {
		status = feStatus(status)
		client := newTransportClient(t, testutil.StaticTransport(status, `{}`))
		res, err := client.EnrichContact(t.Context(), validRequest())
		require.Nil(t, res)
		ce := callError(t, err)
		assert.Equal(t, status, ce.StatusCode)
		want := feKind(status)
		if status == http.StatusOK {
			// {} has no isError flag, so even a 200 is not a success.
			want = enformion.FailureUnexpectedResponse
		}
		assert.Equal(t, want, ce.Kind)
		assert.ErrorIs(t, err, kindSentinels()[want])
	})
}
