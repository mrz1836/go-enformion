package enformion_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion"
	"github.com/mrz1836/go-enformion/internal/testutil"
)

// Synthetic test credentials. They double as leak markers: no output may ever
// contain them. They are not real secrets, so gitleaks is told to allow them.
const (
	testKeyName     = "keyname-marker-7f3a2c"
	testKeyPassword = "keypass-marker-91c2e8" // gitleaks:allow (synthetic test marker)
)

// loopbackOrigin is a test base URL for in-memory transports, which never dial.
const loopbackOrigin = "https://127.0.0.1"

// emptyEnvelope is the smallest successful envelope: 17 bytes, no person.
const emptyEnvelope = `{"isError":false}`

// validRequest returns a request with two categories, name and email, built
// from synthetic values.
func validRequest() enformion.EnrichContactRequest {
	return enformion.EnrichContactRequest{
		FirstName: "Avery",
		LastName:  "Example",
		Email:     "avery.example@example.com",
	}
}

// newClient returns a client built with the test credentials and opts,
// failing tb when NewClient fails.
func newClient(tb testing.TB, opts ...enformion.Option) *enformion.Client {
	tb.Helper()
	c, err := enformion.NewClient(testKeyName, testKeyPassword, opts...)
	require.NoError(tb, err)
	return c
}

// newServerClient starts a recording TLS server running h and returns a client
// pointed at it, with opts applied after the server options.
func newServerClient(tb testing.TB, h http.HandlerFunc, opts ...enformion.Option) (*enformion.Client, *testutil.Server) {
	tb.Helper()
	srv := testutil.NewServer(tb, h)
	all := append([]enformion.Option{
		enformion.WithTestBaseURL(srv.URL()),
		enformion.WithHTTPClient(srv.Client()),
	}, opts...)
	return newClient(tb, all...), srv
}

// newTransportClient returns a client whose calls go through rt, in memory,
// with opts applied after the transport options.
func newTransportClient(tb testing.TB, rt http.RoundTripper, opts ...enformion.Option) *enformion.Client {
	tb.Helper()
	all := append([]enformion.Option{
		enformion.WithTestBaseURL(loopbackOrigin),
		enformion.WithHTTPClient(&http.Client{Transport: rt}),
	}, opts...)
	return newClient(tb, all...)
}

// redirectTransport answers in memory with a 302 to location, whose body is
// body, declared with contentLength (-1 for unknown).
func redirectTransport(location string, body io.ReadCloser, contentLength int64) testutil.RoundTripFunc {
	next := testutil.Transport(http.StatusFound, contentLength, func() io.ReadCloser { return body })
	return func(r *http.Request) (*http.Response, error) {
		resp, err := next(r)
		resp.Header.Set("Location", location)
		return resp, err
	}
}

// respond returns a handler that answers with status and body as JSON.
func respond(status int, body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}
}

// callError returns the *enformion.CallError in err's chain, failing tb when
// there is none.
func callError(tb testing.TB, err error) *enformion.CallError {
	tb.Helper()
	ce, ok := errors.AsType[*enformion.CallError](err)
	require.True(tb, ok, "want a *enformion.CallError, got %T", err)
	return ce
}

// validationError returns the *enformion.ValidationError in err's chain,
// failing tb when there is none.
func validationError(tb testing.TB, err error) *enformion.ValidationError {
	tb.Helper()
	var ve *enformion.ValidationError
	require.ErrorAs(tb, err, &ve)
	return ve
}

// assertErrorClean fails tb when a secret appears however a caller might print
// or walk err: any fmt verb on each error of its Unwrap chain, or on err
// wrapped or joined, or any string reflection reaches. The chain must also hold
// no *url.Error, whose text quotes the URL.
func assertErrorClean(tb testing.TB, err error, secrets ...string) {
	tb.Helper()
	for e := err; e != nil; e = errors.Unwrap(e) {
		testutil.AssertNeverPrints(tb, e, secrets...)
	}
	testutil.AssertNeverPrints(tb, fmt.Errorf("wrapped: %w", err), secrets...)
	testutil.AssertNeverPrints(tb, errors.Join(err, context.Canceled), secrets...)
	testutil.AssertRedacted(tb, strings.Join(testutil.ReflectStrings(err), "\n"), secrets, "reflection walk")
	_, isURLErr := errors.AsType[*url.Error](err)
	assert.False(tb, isURLErr, "the chain must never hold a *url.Error")
}

// kindSentinels is the documented mapping from each failure kind to the
// sentinel, or context error, that it matches with errors.Is, written out
// independently of the library's. FailureTimeout also matches
// context.DeadlineExceeded.
func kindSentinels() map[enformion.FailureKind]error {
	return map[enformion.FailureKind]error{
		enformion.FailureUnauthorized:       enformion.ErrUnauthorized,
		enformion.FailureRateLimited:        enformion.ErrRateLimited,
		enformion.FailureRejected:           enformion.ErrRejected,
		enformion.FailureUpstream:           enformion.ErrUpstream,
		enformion.FailureUnexpectedResponse: enformion.ErrUnexpectedResponse,
		enformion.FailureMalformedResponse:  enformion.ErrMalformedResponse,
		enformion.FailureResponseTooLarge:   enformion.ErrResponseTooLarge,
		enformion.FailureTransport:          enformion.ErrTransport,
		enformion.FailureTimeout:            enformion.ErrTimeout,
		enformion.FailureCanceled:           context.Canceled,
		enformion.FailureDeadlineExceeded:   context.DeadlineExceeded,
	}
}
