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
