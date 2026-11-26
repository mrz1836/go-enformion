package enformion_test

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion"
	"github.com/mrz1836/go-enformion/internal/testutil"
)

// storedLimit is the largest body any call reads, and so the largest a
// decoder accepts.
const storedLimit = 16 << 20

// storedBody is a response body a caller kept, named for its subtest.
type storedBody struct {
	name string
	body []byte
}

// storedOperation is one operation as the tests drive it: a call through a
// client, and a decode of the bytes it read.
type storedOperation[R any] struct {
	call        func(*testing.T, *enformion.Client) (*R, error)
	decode      func([]byte) (*R, error)
	diagnostics func(*R) enformion.ProviderDiagnostics
}

// storedSearch is Person Search, called with validSearch.
func storedSearch() storedOperation[enformion.PersonSearchResult] {
	return storedOperation[enformion.PersonSearchResult]{
		call: func(t *testing.T, c *enformion.Client) (*enformion.PersonSearchResult, error) {
			return c.SearchPersons(t.Context(), validSearch())
		},
		decode:      enformion.DecodeSearchPersons,
		diagnostics: func(r *enformion.PersonSearchResult) enformion.ProviderDiagnostics { return r.Diagnostics },
	}
}

// storedEnrich is Contact Enrichment, called with validRequest.
func storedEnrich() storedOperation[enformion.EnrichContactResult] {
	return storedOperation[enformion.EnrichContactResult]{
		call: func(t *testing.T, c *enformion.Client) (*enformion.EnrichContactResult, error) {
			return c.EnrichContact(t.Context(), validRequest())
		},
		decode:      enformion.DecodeEnrichContact,
		diagnostics: func(r *enformion.EnrichContactResult) enformion.ProviderDiagnostics { return r.Diagnostics },
	}
}

// storedFixtures returns every JSON fixture under testdata/provider and
// testdata/synthetic, each named "<dir>/<name>": the Person Search fixtures,
// whose names start with person-search-, when search is set, and the others
// when it is not. It fails tb when it finds none.
func storedFixtures(tb testing.TB, search bool) []storedBody {
	tb.Helper()
	var fixtures []storedBody
	for _, dir := range []string{"provider", "synthetic"} {
		entries, err := os.ReadDir(filepath.Join("testdata", dir))
		require.NoError(tb, err)
		for _, e := range entries {
			name := e.Name()
			if !e.Type().IsRegular() || filepath.Ext(name) != ".json" ||
				strings.HasPrefix(name, "person-search-") != search {
				continue
			}
			rel := dir + "/" + name
			fixtures = append(fixtures, storedBody{name: rel, body: testutil.Fixture(tb, rel)})
		}
	}
	require.NotEmpty(tb, fixtures, "no fixtures found")
	return fixtures
}

// storedSynthetic lists short bodies, each failing one check of the decoding,
// in a Person Search form and a Contact Enrichment form.
func storedSynthetic() []struct{ name, search, enrich string } {
	return []struct{ name, search, enrich string }{
		{"empty", ``, ``},
		{"not json", `not json`, `not json`},
		{"trailing data", `{"isError":false,"persons":[]} {}`, `{"isError":false} {}`},
		{"repeated key", `{"isError":false,"isError":false,"persons":[]}`, `{"isError":false,"isError":false}`},
		{"invalid utf8", "{\"isError\":false,\"persons\":[],\"message\":\"\xff\"}", "{\"isError\":false,\"message\":\"\xff\"}"},
		{"byte order mark", "\ufeff" + emptySearchPage, "\ufeff" + emptyEnvelope},
		{"isError not a boolean", `{"isError":"false","persons":[]}`, `{"isError":"false"}`},
		{"wrong type", `{"isError":false,"persons":[{"age":41.5}]}`, `{"isError":false,"identityScore":"87"}`},
		{"top-level array", `[]`, `[]`},
	}
}

// storedSearchBodies returns the Person Search fixtures and synthetic bodies.
func storedSearchBodies(tb testing.TB) []storedBody {
	tb.Helper()
	bodies := storedFixtures(tb, true)
	for _, s := range storedSynthetic() {
		bodies = append(bodies, storedBody{name: s.name, body: []byte(s.search)})
	}
	return bodies
}

// storedEnrichBodies returns the Contact Enrichment fixtures and synthetic
// bodies.
func storedEnrichBodies(tb testing.TB) []storedBody {
	tb.Helper()
	bodies := storedFixtures(tb, false)
	for _, s := range storedSynthetic() {
		bodies = append(bodies, storedBody{name: s.name, body: []byte(s.enrich)})
	}
	return bodies
}

// requireSameOutcome fails t unless a decode gave the outcome a call gave:
// equal results, or *CallError values alike in every exported field, their
// text, cause, diagnostics and sentinel matches, with no result.
func requireSameOutcome[R any](t *testing.T, want *R, wantErr error, got *R, gotErr error) {
	t.Helper()
	if wantErr == nil {
		require.NoError(t, gotErr)
		require.NotNil(t, got)
		assert.Equal(t, want, got)
		return
	}
	require.Error(t, gotErr)
	assert.Nil(t, want)
	assert.Nil(t, got)
	w, g := callError(t, wantErr), callError(t, gotErr)
	assert.Equal(t, w.Kind, g.Kind)
	assert.Equal(t, w.StatusCode, g.StatusCode)
	assert.Equal(t, w.RequestID, g.RequestID)
	assert.Equal(t, w.Field, g.Field)
	assert.Equal(t, w.Error(), g.Error())
	assert.Equal(t, errors.Unwrap(w), errors.Unwrap(g))
	assert.Equal(t, w.Diagnostics(), g.Diagnostics())
	for kind, sentinel := range kindSentinels() {
		assert.Equal(t, errors.Is(wantErr, sentinel), errors.Is(gotErr, sentinel), "errors.Is for %s", kind)
	}
}

// storedMatchesTheCall serves each body through a local server, then decodes
// the same bytes directly, and requires the same outcome.
func storedMatchesTheCall[R any](t *testing.T, op storedOperation[R], bodies []storedBody) {
	t.Helper()
	for _, tt := range bodies {
		t.Run(tt.name, func(t *testing.T) {
			c, srv := newServerClient(t, respond(http.StatusOK, tt.body))
			want, wantErr := op.call(t, c)
			require.Equal(t, 1, srv.Hits(), "the call is made exactly once")
			got, gotErr := op.decode(tt.body)
			requireSameOutcome(t, want, wantErr, got, gotErr)
		})
	}
}

// TestDecodeSearchPersonsMatchesTheCall checks that every kept Person Search
// body decodes to the result, or the error, that the call returned for it.
func TestDecodeSearchPersonsMatchesTheCall(t *testing.T) {
	storedMatchesTheCall(t, storedSearch(), storedSearchBodies(t))
}

// TestDecodeEnrichContactMatchesTheCall checks that every kept Contact
// Enrichment body decodes to the result, or the error, that the call returned
// for it.
func TestDecodeEnrichContactMatchesTheCall(t *testing.T) {
	storedMatchesTheCall(t, storedEnrich(), storedEnrichBodies(t))
}

// TestDecodersKeepTheirSignatures pins each decoder's exact type, so that a
// signature change fails here and not only in a caller's build.
func TestDecodersKeepTheirSignatures(t *testing.T) {
	assert.IsType(t, (func([]byte) (*enformion.PersonSearchResult, error))(nil), enformion.DecodeSearchPersons)
	assert.IsType(t, (func([]byte) (*enformion.EnrichContactResult, error))(nil), enformion.DecodeEnrichContact)
}

// storedPadded returns base followed by spaces, n bytes in all: valid JSON of
// an exact size.
func storedPadded(base string, n int) []byte {
	b := bytes.Repeat([]byte{' '}, n)
	copy(b, base)
	return b
}

// TestDecodeRefusesABodyOverTheLimit checks the decoders' bound at its edge: a
// body one byte over the largest any call reads fails as a call's oversized
// response does, and a body of exactly that size decodes.
func TestDecodeRefusesABodyOverTheLimit(t *testing.T) {
	c, _ := newServerClient(t, respond(http.StatusOK, []byte(emptySearchPage)), enformion.WithMaxResponseBytes(1))
	_, callErr := c.SearchPersons(t.Context(), validSearch())
	require.ErrorIs(t, callErr, enformion.ErrResponseTooLarge)
	cause := errors.Unwrap(callErr)
	require.Error(t, cause)

	decoders := []struct {
		name   string
		base   string
		decode func([]byte) (bool, error)
	}{
		{"search", emptySearchPage, func(b []byte) (bool, error) {
			res, err := enformion.DecodeSearchPersons(b)
			return res != nil, err
		}},
		{"enrich", emptyEnvelope, func(b []byte) (bool, error) {
			res, err := enformion.DecodeEnrichContact(b)
			return res != nil, err
		}},
	}
	for _, d := range decoders {
		t.Run(d.name+" one byte over the limit", func(t *testing.T) {
			hasResult, err := d.decode(storedPadded(d.base, storedLimit+1))
			ce := callError(t, err)
			assert.False(t, hasResult)
			assert.Equal(t, enformion.FailureResponseTooLarge, ce.Kind)
			assert.Equal(t, http.StatusOK, ce.StatusCode)
			require.ErrorIs(t, err, enformion.ErrResponseTooLarge)
			assert.Equal(t, cause, errors.Unwrap(err))
		})
		t.Run(d.name+" at the limit", func(t *testing.T) {
			hasResult, err := d.decode(storedPadded(d.base, storedLimit))
			require.NoError(t, err)
			assert.True(t, hasResult)
		})
	}
}

// storedNoDiagnostics checks one operation against a client created with
// WithProviderDiagnostics: for isError, a body the provider refused, the
// call's error keeps the diagnostics and the decoded error has none; for
// withWarning, a successful body with a warning, both results carry it.
func storedNoDiagnostics[R any](t *testing.T, op storedOperation[R], isError, withWarning string) {
	t.Helper()
	t.Run(isError, func(t *testing.T) {
		body := testutil.Fixture(t, isError)
		c, _ := newServerClient(t, respond(http.StatusOK, body), enformion.WithProviderDiagnostics())
		_, callErr := op.call(t, c)
		want := callError(t, callErr)
		require.NotEqual(t, enformion.ProviderDiagnostics{}, want.Diagnostics(), "the call keeps the diagnostics")

		res, err := op.decode(body)
		assert.Nil(t, res)
		got := callError(t, err)
		assert.Equal(t, enformion.ProviderDiagnostics{}, got.Diagnostics(), "a decoded error keeps none")
		assert.Equal(t, want.Kind, got.Kind)
		assert.Equal(t, want.StatusCode, got.StatusCode)
		assert.Equal(t, want.RequestID, got.RequestID)
		assert.Equal(t, want.Field, got.Field)
		assert.Equal(t, want.Error(), got.Error())
	})
	t.Run(withWarning, func(t *testing.T) {
		body := testutil.Fixture(t, withWarning)
		c, _ := newServerClient(t, respond(http.StatusOK, body), enformion.WithProviderDiagnostics())
		want, err := op.call(t, c)
		require.NoError(t, err)
		got, err := op.decode(body)
		require.NoError(t, err)
		assert.NotEqual(t, enformion.ProviderDiagnostics{}, op.diagnostics(got), "a result carries its diagnostics")
		assert.Equal(t, op.diagnostics(want), op.diagnostics(got))
	})
}

// TestDecodeErrorsKeepNoDiagnostics checks that a decoded error carries no
// provider diagnostics, as from a client created without
// WithProviderDiagnostics, while a decoded result carries its own.
func TestDecodeErrorsKeepNoDiagnostics(t *testing.T) {
	t.Run("search", func(t *testing.T) {
		storedNoDiagnostics(t, storedSearch(), "synthetic/person-search-is-error.json",
			"synthetic/person-search-no-persons.json")
	})
	t.Run("enrich", func(t *testing.T) {
		storedNoDiagnostics(t, storedEnrich(), "synthetic/is-error.json", "synthetic/no-person.json")
	})
}

// storedShareNothing decodes a copy of fixture, then overwrites the copy, and
// checks that the result still equals a decode of the original: it shares no
// memory with the body it came from.
func storedShareNothing[R any](t *testing.T, decode func([]byte) (*R, error), fixture string) {
	t.Helper()
	t.Run(fixture, func(t *testing.T) {
		original := testutil.Fixture(t, fixture)
		body := bytes.Clone(original)
		res, err := decode(body)
		require.NoError(t, err)
		assert.Equal(t, original, body, "the body is never modified")

		for i := range body {
			body[i] = 'x'
		}
		want, err := decode(original)
		require.NoError(t, err)
		assert.Equal(t, want, res, "the result is unchanged after its body is overwritten")
		again, err := decode(original)
		require.NoError(t, err)
		assert.Equal(t, want, again, "two decodes of the same bytes are equal")
	})
}

// TestDecodedResultsShareNoMemoryWithTheBody checks that a decoded result,
// raw members and Extra included, holds its own copy of every byte.
func TestDecodedResultsShareNoMemoryWithTheBody(t *testing.T) {
	storedShareNothing(t, enformion.DecodeSearchPersons, "provider/person-search-page.json")
	storedShareNothing(t, enformion.DecodeSearchPersons, "synthetic/person-search-every-field.json")
	storedShareNothing(t, enformion.DecodeEnrichContact, "provider/match.json")
}
