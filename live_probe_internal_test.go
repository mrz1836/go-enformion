//go:build live

package enformion

// The live contract probe makes real, potentially billable calls to
// devapi.enformion.com to learn the shape of responses the documentation leaves
// out. It is run only by the repository owner, by hand, after approving the
// records, the call count and the budget. CI and automation never run it.
//
// It skips unless all of these hold:
//
//	ENFORMION_LIVE=1
//	ENFORMION_LIVE_ACK_BILLING=I-understand
//	CI unset
//
// Once enabled, it reads, only in this test harness:
//
//	ENFORMION_LIVE_PROBES=<comma-separated names: nomatch, validation, auth, casing>
//	ENFORMION_AP_NAME, ENFORMION_AP_PASSWORD
//
// and fails before sending anything when no known probe is named, when the
// chosen probes need more than 4 calls (casing takes 2, the others 1 each, so
// all four together need 5), or when the credentials are invalid.
//
// and runs with:
//
//	go test -tags live -count=1 -p 1 -run TestLiveProbe -v .
//
// At most 4 calls are made, never retried, and an HTTP 429 stops every probe.
// Output is response shape only: the status, response header names, the JSON
// type tree with safe key names, and the isError flag. No value is ever printed
// or written. If the nomatch probe unexpectedly returns person data, its
// response is discarded unprinted and the run stops.

import (
	"context"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion/internal/testutil"
)

// liveMaxCalls bounds the calls of one run.
const liveMaxCalls = 4

// liveProbe is one approved probe and the calls it makes.
type liveProbe struct {
	name  string
	calls int
	run   func(t *testing.T, lc *liveClient) (stop bool)
}

// liveProbes returns the probes, in the order they run.
func liveProbes() []liveProbe {
	return []liveProbe{
		{name: "nomatch", calls: 1, run: liveNoMatch},
		{name: "validation", calls: 1, run: liveValidation},
		{name: "auth", calls: 1, run: liveAuth},
		{name: "casing", calls: 2, run: liveCasing},
	}
}

func TestLiveProbe(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("live probes never run in CI")
	}
	if os.Getenv("ENFORMION_LIVE") != "1" || os.Getenv("ENFORMION_LIVE_ACK_BILLING") != "I-understand" {
		t.Skip("live probes are opt-in; see the comment at the top of live_probe_internal_test.go")
	}
	selected := strings.Split(os.Getenv("ENFORMION_LIVE_PROBES"), ",")
	var run []liveProbe
	total := 0
	for _, p := range liveProbes() {
		if slices.Contains(selected, p.name) {
			run = append(run, p)
			total += p.calls
		}
	}
	require.NotEmpty(t, run, "ENFORMION_LIVE_PROBES names no known probe")
	require.LessOrEqual(t, total, liveMaxCalls, "the selected probes exceed the call budget; nothing was sent")

	lc := newLiveClient(t, os.Getenv("ENFORMION_AP_NAME"), os.Getenv("ENFORMION_AP_PASSWORD"))
	for _, p := range run {
		t.Logf("probe %s (%d call(s))", p.name, p.calls)
		if p.run(t, lc) {
			t.Logf("stopping: no further probes run")
			return
		}
	}
}

// liveClient sends probe calls and records response header names.
type liveClient struct {
	client *Client
	rec    *liveRecorder
}

func newLiveClient(t *testing.T, keyName, keyPassword string) *liveClient {
	t.Helper()
	rec := &liveRecorder{next: newTransport()}
	c, err := NewClient(keyName, keyPassword, WithHTTPClient(&http.Client{Transport: rec}))
	require.NoError(t, err, "credentials are invalid; nothing was sent")
	return &liveClient{client: c, rec: rec}
}

// send makes one call with a raw body and returns the exchange, or nil after
// logging the failure kind.
func (lc *liveClient) send(t *testing.T, label string, body []byte) *exchange {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), defaultTimeout)
	defer cancel()
	ex, err := lc.client.send(ctx, &call{path: contactEnrichPath, searchType: contactEnrichSearchType, body: body})
	if err != nil {
		t.Logf("%s: failed: %v", label, err)
		return nil
	}
	return ex
}

// report logs the shape of an exchange, never its values.
func (lc *liveClient) report(t *testing.T, label string, ex *exchange) {
	t.Helper()
	t.Logf("%s: HTTP %d", label, ex.status)
	t.Logf("%s: response headers %v", label, lc.rec.lastHeaderNames())
	if ex.body == nil {
		t.Logf("%s: body unreadable or over the limit", label)
		return
	}
	shape, ok := testutil.JSONShape(ex.body)
	if !ok {
		t.Logf("%s: body is not JSON (%d bytes)", label, len(ex.body))
		return
	}
	isError := "absent or not a boolean"
	if kind, _ := checkStrict(ex.body); kind == "" {
		env, _ := readObject(ex.body, envelopeNames(), false)
		if v := string(env.field(envIsError)); v == "true" || v == "false" {
			isError = v
		}
	}
	t.Logf("%s: isError %s", label, isError)
	t.Logf("%s: shape\n%s", label, shape)
}

// liveNoMatchRequest is a subject invented for the probe: a fictional 555-01xx
// number, an example.com address and a made-up name.
func liveNoMatchRequest() EnrichContactRequest {
	return EnrichContactRequest{
		FirstName: "Zyxprobe",
		LastName:  "Qwvexample",
		Phone:     "(312) 555-0100",
		Email:     "nomatch-probe@example.com",
	}
}

func liveNoMatch(t *testing.T, lc *liveClient) bool {
	req := liveNoMatchRequest()
	body, err := req.encode()
	require.NoError(t, err)
	ex := lc.send(t, "nomatch", body)
	if ex == nil {
		return true
	}
	if ex.status == http.StatusTooManyRequests {
		lc.report(t, "nomatch", ex)
		return true
	}
	if res, derr := decodeEnrichContact(ex, false); derr == nil && res.Person != nil {
		// Discard the response unprinted: it holds someone's personal data.
		ex.body = nil
		t.Logf("nomatch: UNEXPECTED person data returned; response discarded unprinted")
		return true
	}
	lc.report(t, "nomatch", ex)
	return false
}

func liveValidation(t *testing.T, lc *liveClient) bool {
	ex := lc.send(t, "validation", []byte(`{}`))
	if ex == nil {
		return true
	}
	lc.report(t, "validation", ex)
	return ex.status == http.StatusTooManyRequests
}

func liveAuth(t *testing.T, _ *liveClient) bool {
	bogus := newLiveClient(t, "probe-invalid-key-name", "probe-invalid-key-password")
	req := liveNoMatchRequest()
	body, err := req.encode()
	require.NoError(t, err)
	ex := bogus.send(t, "auth", body)
	if ex == nil {
		return true
	}
	bogus.report(t, "auth", ex)
	return ex.status == http.StatusTooManyRequests
}

// liveCasing sends the same invented phone and address twice, once with the
// address keys of the documentation's JSON examples (addressLine1) and once
// with those of its properties table (AddressLine1), to compare the shapes.
func liveCasing(t *testing.T, lc *liveClient) bool {
	bodies := []struct{ label, body string }{
		{"casing-camel", `{"Address":{"addressLine1":"1 Nowhere Probe Rd","addressLine2":"Springfield, IL 62701"},"Phone":"(312) 555-0100"}`},
		{"casing-pascal", `{"Address":{"AddressLine1":"1 Nowhere Probe Rd","AddressLine2":"Springfield, IL 62701"},"Phone":"(312) 555-0100"}`},
	}
	for _, b := range bodies {
		ex := lc.send(t, b.label, []byte(b.body))
		if ex == nil {
			return true
		}
		if res, derr := decodeEnrichContact(ex, false); derr == nil && res.Person != nil {
			ex.body = nil
			t.Logf("%s: UNEXPECTED person data returned; response discarded unprinted", b.label)
			return true
		}
		lc.report(t, b.label, ex)
		if ex.status == http.StatusTooManyRequests {
			return true
		}
	}
	return false
}

// liveRecorder passes requests to next and keeps the header names of the last
// response.
type liveRecorder struct {
	next  http.RoundTripper
	mu    sync.Mutex
	names []string
}

func (r *liveRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := r.next.RoundTrip(req)
	if resp != nil {
		r.mu.Lock()
		r.names = testutil.HeaderNames(resp.Header)
		r.mu.Unlock()
	}
	return resp, err
}

func (r *liveRecorder) lastHeaderNames() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.names)
}
