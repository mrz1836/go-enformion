package enformion_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion"
)

// ccEcho answers each request with a person built from the request's own
// session ID, "session-<marker>": the marker becomes the first name,
// "<marker>@example.com" the person's email and "req-<marker>" the request ID.
// A response that reached the wrong caller therefore shows up as a mismatch.
// (The recording server has already read the body, so the handler uses the
// header; the test then checks every recorded body against its header.)
func ccEcho(w http.ResponseWriter, r *http.Request) {
	marker, ok := strings.CutPrefix(r.Header.Get("Galaxy-Client-Session-Id"), "session-")
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	out := map[string]any{
		"isError":   false,
		"requestId": "req-" + marker,
		"person": map[string]any{
			"name":   map[string]any{"firstName": marker, "lastName": "Example"},
			"emails": []any{map[string]any{"email": marker + "@example.com"}},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		return
	}
}

// ccCall is one call's request and outcome.
type ccCall struct {
	req enformion.EnrichContactRequest
	res *enformion.EnrichContactResult
	err error
}

// TestConcurrencySharedClient shares one Client across 64 goroutines making 4
// calls each, and checks that every caller gets its own response and every
// request its own headers and body.
func TestConcurrencySharedClient(t *testing.T) {
	const (
		goroutines = 64
		perG       = 4
	)
	c, srv := newServerClient(t, ccEcho)

	calls := make([][]ccCall, goroutines)
	var wg sync.WaitGroup
	for g := range goroutines {
		calls[g] = make([]ccCall, perG)
		wg.Go(func() {
			for i := range perG {
				marker := fmt.Sprintf("cc%02dx%d", g, i)
				req := validRequest()
				req.Email = marker + "@example.com"
				req.SessionID = "session-" + marker
				res, err := c.EnrichContact(t.Context(), req)
				calls[g][i] = ccCall{req: req, res: res, err: err}
			}
		})
	}
	wg.Wait()

	results := make([]*enformion.EnrichContactResult, 0, goroutines*perG)
	for g := range goroutines {
		for i := range perG {
			call := calls[g][i]
			require.NoError(t, call.err)
			marker, _, _ := strings.Cut(call.req.Email, "@")
			require.NotNil(t, call.res.Person)
			assert.Equal(t, marker, call.res.Person.Name.FirstName, "no cross-talk between callers")
			require.Len(t, call.res.Person.Emails, 1)
			assert.Equal(t, call.req.Email, call.res.Person.Emails[0].Address)
			assert.Equal(t, "req-"+marker, call.res.RequestID)
			results = append(results, call.res)
		}
	}

	reqs := srv.Requests()
	require.Len(t, reqs, goroutines*perG)
	assert.Equal(t, goroutines*perG, srv.Hits())
	seen := map[string]bool{}
	for _, r := range reqs {
		var body struct {
			Email string `json:"Email"`
		}
		require.NoError(t, json.Unmarshal(r.Body, &body))
		marker, _, _ := strings.Cut(body.Email, "@")
		assert.Equal(t, []string{"session-" + marker}, r.Header.Values("Galaxy-Client-Session-Id"),
			"each request carries its own session ID")
		assert.Equal(t, []string{testKeyName}, r.Header.Values("Galaxy-Ap-Name"))
		assert.False(t, seen[marker], "each request is sent once")
		seen[marker] = true
	}

	// Results share no memory: changing one leaves the others as they were.
	results[0].Person.Name.FirstName = "changed"
	results[0].Person.Emails[0].Address = "changed@example.com"
	for _, res := range results[1:] {
		assert.NotEqual(t, "changed", res.Person.Name.FirstName)
		assert.NotEqual(t, "changed@example.com", res.Person.Emails[0].Address)
	}
	assert.Equal(t, "cc00x0@example.com", calls[0][0].req.Email, "the request is never changed")
}

// ccMixedEcho answers Contact Enrichment as ccEcho does, and Person Search
// with a page whose one person is built from the session ID's marker and
// whose criteriaType echoes the search type header.
func ccMixedEcho(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/PersonSearch" {
		ccEcho(w, r)
		return
	}
	marker, ok := strings.CutPrefix(r.Header.Get("Galaxy-Client-Session-Id"), "session-")
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	out := map[string]any{
		"isError":      false,
		"requestId":    "req-" + marker,
		"criteriaType": map[string]any{"searchType": r.Header.Get("Galaxy-Search-Type")},
		"persons":      []any{map[string]any{"tahoeId": "G-" + marker, "fullName": marker}},
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		return
	}
}

// TestConcurrencyMixedOperations shares one Client across goroutines that
// make Contact Enrichment calls and Person Search calls in every mode, and
// checks that each caller gets its own response, from its own operation.
func TestConcurrencyMixedOperations(t *testing.T) {
	const goroutines = 40
	c, srv := newServerClient(t, ccMixedEcho)
	modes := searchModes()

	type outcome struct {
		marker string
		mode   enformion.PersonSearchMode // "" for Contact Enrichment
		search *enformion.PersonSearchResult
		enrich *enformion.EnrichContactResult
		err    error
	}
	outcomes := make([]outcome, goroutines)
	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Go(func() {
			o := outcome{marker: fmt.Sprintf("mx%02d", g)}
			if g%5 == 0 {
				req := validRequest()
				req.Email = o.marker + "@example.com"
				req.SessionID = "session-" + o.marker
				o.enrich, o.err = c.EnrichContact(t.Context(), req)
			} else {
				o.mode = modes[g%len(modes)]
				req := validSearch()
				req.Mode = o.mode
				req.SessionID = "session-" + o.marker
				o.search, o.err = c.SearchPersons(t.Context(), req)
			}
			outcomes[g] = o
		})
	}
	wg.Wait()

	for _, o := range outcomes {
		require.NoError(t, o.err)
		if o.mode == "" {
			require.NotNil(t, o.enrich.Person)
			assert.Equal(t, o.marker, o.enrich.Person.Name.FirstName, "no cross-talk between operations")
			assert.Equal(t, "req-"+o.marker, o.enrich.RequestID)
			continue
		}
		require.Len(t, o.search.Persons, 1)
		assert.Equal(t, "G-"+o.marker, o.search.Persons[0].TahoeID, "no cross-talk between callers")
		assert.Equal(t, "req-"+o.marker, o.search.RequestID)
		assert.JSONEq(t, `{"searchType":"`+string(o.mode)+`"}`, string(o.search.CriteriaType), "each call carries its own mode")
	}

	reqs := srv.Requests()
	require.Len(t, reqs, goroutines)
	for _, r := range reqs {
		switch r.URL {
		case "/Contact/Enrich":
			assert.Equal(t, []string{"DevAPIContactEnrich"}, r.Header.Values("Galaxy-Search-Type"))
		case "/PersonSearch":
			assert.Equal(t, `{"FirstName":"Riley","LastName":"Sample"}`, string(r.Body)) //nolint:testifylint // byte-exact, not just equivalent JSON
			assert.Contains(t, modes, enformion.PersonSearchMode(r.Header.Get("Galaxy-Search-Type")))
		default:
			t.Errorf("unexpected path %q", r.URL)
		}
		assert.Equal(t, []string{testKeyName}, r.Header.Values("Galaxy-Ap-Name"))
	}
}
