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

func FuzzSearchPersonsEndToEnd(f *testing.F) {
	const fsMode, fsLast = "Teaser", "Sample"
	for _, s := range []struct {
		status int
		body   string
		mode   string
		last   string
	}{
		{200, string(testutil.Fixture(f, "provider/person-search-page.json")), "Person", fsLast},
		{200, string(testutil.Fixture(f, "synthetic/person-search-every-field.json")), fsMode, fsLast},
		{200, string(testutil.Fixture(f, "synthetic/person-search-no-persons.json")), "ReversePhonePerson", fsLast},
		{200, string(testutil.Fixture(f, "synthetic/person-search-is-error.json")), "ReversePhonePersonTeaser", fsLast},
		{200, string(testutil.Fixture(f, "synthetic/person-search-missing-persons.json")), fsMode, fsLast},
		{200, string(testutil.Fixture(f, "synthetic/person-search-unknown-keys-person.json")), fsMode, fsLast},
		{200, `{"isError":false,"persons":[{"age":41.5}]}`, fsMode, fsLast},
		{200, `not json`, fsMode, fsLast},
		{400, string(testutil.Fixture(f, "provider/person-search-rejected-400.json")), fsMode, fsLast},
		{429, `{}`, fsMode, fsLast},
		{302, ``, fsMode, fsLast},
		{503, `{"message":"synthetic"}`, "Person", fsLast},
		{200, emptySearchPage, "", fsLast},
		{200, emptySearchPage, "teaser", fsLast},
		{200, emptySearchPage, fsMode, ""},
		{200, emptySearchPage, fsMode, "Sam\tple"},
		{200, emptySearchPage, fsMode, "Sam\xffple"},
		{200, emptySearchPage, fsMode, strings.Repeat("a", 257)},
	} {
		f.Add(s.status, []byte(s.body), s.mode, s.last)
	}
	f.Fuzz(func(t *testing.T, status int, body []byte, mode, last string) {
		status = feStatus(status)
		var resp atomic.Pointer[testutil.Body]
		ct := &testutil.CountingTransport{Next: testutil.Transport(status, int64(len(body)), func() io.ReadCloser {
			b := testutil.NewBody(string(body))
			resp.Store(b)
			return b
		})}
		client := newTransportClient(t, ct)
		req := enformion.PersonSearchRequest{Mode: enformion.PersonSearchMode(mode), LastName: last}
		res, err := client.SearchPersons(t.Context(), req)

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
