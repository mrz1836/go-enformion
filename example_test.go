package enformion_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"time"

	"github.com/mrz1836/go-enformion"
)

// exMatchBody is a synthetic HTTP 200 response in the documented shape.
const exMatchBody = `{
  "person": {
    "personId": "G-1000000000000000042",
    "name": {"firstName": "Riley", "middleName": "Q", "lastName": "Sample"},
    "age": "41",
    "partialDob": "6/1985",
    "addresses": [
      {"street": "100 Sample Way", "unit": "", "city": "Exampleton", "state": "OH", "zip": "45000",
       "firstReportedDate": "3/2/2011", "lastReportedDate": "8/14/2025"}
    ],
    "phones": [
      {"number": "(312) 555-0142", "type": "mobile", "isConnected": true, "lastReportedDate": "9/30/2025"},
      {"number": "(312) 555-0199", "type": "landline"}
    ],
    "emails": [{"email": "riley.sample@example.com", "isValidated": true, "isBusiness": false}]
  },
  "message": "",
  "identityScore": 87,
  "requestId": "synthetic-request-0001",
  "isError": false,
  "error": {"inputErrors": [], "warnings": []}
}`

// exServer starts a local TLS server that answers every request with status
// and body. It stands in for the provider, so these examples run offline.
func exServer(status int, body string) *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
}

// exLocalClient returns a client pointed at a local test server.
func exLocalClient(srv *httptest.Server, opts ...enformion.Option) (*enformion.Client, error) {
	return enformion.NewClient("example-key-name", "example-key-password",
		append([]enformion.Option{
			enformion.WithTestBaseURL(srv.URL),
			enformion.WithHTTPClient(srv.Client()),
		}, opts...)...)
}

// ExampleNewClient shows a production client. The application chooses where
// the credentials come from: the SDK never reads credentials from the
// environment or files.
// This example is compiled but never run, because every call to the provider
// is live and may be billed.
func ExampleNewClient() {
	var cfg struct {
		EnformionKeyName     string // the dashboard's API key name
		EnformionKeyPassword string // the dashboard's API key password
	}
	client, err := enformion.NewClient(cfg.EnformionKeyName, cfg.EnformionKeyPassword,
		enformion.WithTimeout(10*time.Second),
		enformion.WithMaxResponseBytes(512<<10),
	)
	if err != nil {
		fmt.Println("enformion config:", err) // never quotes the credentials
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	res, err := client.EnrichContact(ctx, enformion.EnrichContactRequest{
		FirstName: "Riley",
		LastName:  "Sample",
		Email:     "riley.sample@example.com",
		SessionID: "batch-7f3a9c-0001",
	})
	if err != nil {
		fmt.Println("enrich:", err) // kind, status and field path only
		return
	}
	fmt.Println(res.Person != nil)
}

// ExampleClient_EnrichContact enriches one subject with two categories, name
// and email, and reads the provider's observation.
func ExampleClient_EnrichContact() {
	srv := exServer(http.StatusOK, exMatchBody)
	defer srv.Close()

	client, err := exLocalClient(srv, enformion.WithTimeout(10*time.Second))
	if err != nil {
		fmt.Println("config:", err)
		return
	}
	res, err := client.EnrichContact(context.Background(), enformion.EnrichContactRequest{
		FirstName: "Riley",
		LastName:  "Sample",
		Email:     "riley.sample@example.com",
		SessionID: "batch-7f3a9c-0001",
	})
	if err != nil {
		fmt.Println("enrich:", err)
		return
	}
	if res.Person == nil {
		fmt.Println("no person data in the response") // not proof that no match exists
		return
	}
	p := res.Person
	fmt.Println(p.PersonID, p.Name.FirstName, p.Name.LastName, p.Age, p.PartialDOB)
	for _, ph := range p.Phones {
		connected := "not reported"
		if ph.IsConnected != nil {
			connected = strconv.FormatBool(*ph.IsConnected) // not consent to contact
		}
		fmt.Printf("%s (%s) connected: %s\n", ph.Number, ph.Type, connected)
	}
	for _, a := range p.Addresses {
		fmt.Println(a.Street, a.City, a.State, a.Zip)
	}
	for _, e := range p.Emails {
		fmt.Println(e.Address, "validated:", e.IsValidated != nil && *e.IsValidated)
	}
	if res.IdentityScore != nil {
		fmt.Println("provider identityScore (uninterpreted):", *res.IdentityScore)
	}
	fmt.Println("request ID:", res.RequestID)
	// Output:
	// G-1000000000000000042 Riley Sample 41 6/1985
	// (312) 555-0142 (mobile) connected: true
	// (312) 555-0199 (landline) connected: not reported
	// 100 Sample Way Exampleton OH 45000
	// riley.sample@example.com validated: true
	// provider identityScore (uninterpreted): 87
	// request ID: synthetic-request-0001
}

// ExampleCallError classifies failures with errors.Is. Error strings hold only
// the kind, the HTTP status and a field path, so they are safe to log; provider
// text is reachable only through Diagnostics, after WithProviderDiagnostics.
func ExampleCallError() {
	limited := exServer(http.StatusTooManyRequests, `{}`)
	defer limited.Close()
	rejecting := exServer(http.StatusBadRequest, `{"isError": true,
		"message": "Synthetic: Phone is not a valid number.",
		"error": {"inputErrors": [{"field": "Phone"}], "warnings": []}}`)
	defer rejecting.Close()

	for _, srv := range []*httptest.Server{limited, rejecting} {
		client, err := exLocalClient(srv, enformion.WithProviderDiagnostics())
		if err != nil {
			fmt.Println("config:", err)
			return
		}
		_, err = client.EnrichContact(context.Background(), enformion.EnrichContactRequest{
			FirstName: "Riley", LastName: "Sample", Phone: "(312) 555-0142",
		})
		ce, _ := errors.AsType[*enformion.CallError](err)
		switch {
		case errors.Is(err, context.Canceled):
			fmt.Println("canceled: the call may already have been billed")
		case errors.Is(err, enformion.ErrTimeout): // check before context.DeadlineExceeded
			fmt.Println("SDK timeout")
		case errors.Is(err, context.DeadlineExceeded):
			fmt.Println("caller deadline")
		case errors.Is(err, enformion.ErrUnauthorized):
			fmt.Println("check the API key:", ce.StatusCode)
		case errors.Is(err, enformion.ErrRateLimited):
			fmt.Println("rate limit or monthly quota; the SDK never retries:", ce.StatusCode)
		case errors.Is(err, enformion.ErrRejected):
			d := ce.Diagnostics() // may echo personal data: inspect, don't log
			fmt.Println("rejected:", ce.StatusCode, d.Message, len(d.InputErrors))
		case errors.Is(err, enformion.ErrMalformedResponse), errors.Is(err, enformion.ErrUnexpectedResponse):
			fmt.Println("unreadable response at", ce.Field, "(it may have been billed)")
		}
		fmt.Println("log line:", err)
	}
	// Output:
	// rate limit or monthly quota; the SDK never retries: 429
	// log line: enformion: rate_limited (HTTP 429)
	// rejected: 400 Synthetic: Phone is not a valid number. 1
	// log line: enformion: rejected (HTTP 400)
}

// ExampleValidationError shows a request rejected locally, before anything is
// sent: a first name alone is not a complete name, so only one category (the
// phone) counts.
func ExampleValidationError() {
	// A loopback address with no server: even a bug could not reach the provider.
	client, err := enformion.NewClient("example-key-name", "example-key-password",
		enformion.WithTestBaseURL("https://127.0.0.1:9"))
	if err != nil {
		fmt.Println("config:", err)
		return
	}
	_, err = client.EnrichContact(context.Background(), enformion.EnrichContactRequest{
		FirstName: "Riley",
		Phone:     "(312) 555-0142",
	})
	if ve, ok := errors.AsType[*enformion.ValidationError](err); ok {
		fmt.Println(ve.Field, ve.Reason)
	}
	fmt.Println(errors.Is(err, enformion.ErrInvalidRequest))
	fmt.Println(err)
	// Output:
	// request insufficient_categories
	// true
	// enformion: invalid request: request: insufficient_categories
}

// Example_noPerson reads a response that carried no person data. The provider
// does not document what that means, so it is not proof that no match exists.
func Example_noPerson() {
	srv := exServer(http.StatusOK, `{"person": null, "message": "", "requestId": "synthetic-request-0003",
		"isError": false, "error": {"inputErrors": [], "warnings": ["Synthetic warning: no person was returned."]}}`)
	defer srv.Close()

	client, err := exLocalClient(srv)
	if err != nil {
		fmt.Println("config:", err)
		return
	}
	res, err := client.EnrichContact(context.Background(), enformion.EnrichContactRequest{
		FirstName: "Riley", LastName: "Sample", Email: "riley.sample@example.com",
	})
	if err != nil {
		fmt.Println("enrich:", err)
		return
	}
	if res.Person == nil {
		fmt.Println("no person data in the response; not proof that no match exists")
	}
	fmt.Println("warnings:", len(res.Diagnostics.Warnings))
	fmt.Println(string(res.Diagnostics.Warnings[0]))
	// Output:
	// no person data in the response; not proof that no match exists
	// warnings: 1
	// "Synthetic warning: no person was returned."
}

// exContactEnricher is the small interface an application owns, so that its
// tests can use a fake instead of the provider.
type exContactEnricher interface {
	EnrichContact(ctx context.Context, req enformion.EnrichContactRequest) (*enformion.EnrichContactResult, error)
}

// The real client satisfies the application's interface.
var _ exContactEnricher = (*enformion.Client)(nil)

// exFakeEnricher is an application's test fake.
type exFakeEnricher struct {
	res *enformion.EnrichContactResult
	err error
}

func (f *exFakeEnricher) EnrichContact(context.Context, enformion.EnrichContactRequest) (*enformion.EnrichContactResult, error) {
	return f.res, f.err
}

// Example_fake shows an application test fake. A CallError built from its
// exported fields matches errors.Is like one the SDK returns.
func Example_fake() {
	var e exContactEnricher = &exFakeEnricher{
		err: &enformion.CallError{Kind: enformion.FailureRateLimited, StatusCode: http.StatusTooManyRequests},
	}
	_, err := e.EnrichContact(context.Background(), enformion.EnrichContactRequest{})
	fmt.Println(errors.Is(err, enformion.ErrRateLimited))
	fmt.Println(err)
	// Output:
	// true
	// enformion: rate_limited (HTTP 429)
}

// Example_jsonRoundTrip shows that the SDK's own JSON keeps a false flag apart
// from an unreported one, and a zero score apart from an absent one.
func Example_jsonRoundTrip() {
	in := enformion.EnrichContactResult{
		IdentityScore: new(90),
		Person: &enformion.Person{Phones: []enformion.Phone{
			{Number: "(312) 555-0142", IsConnected: new(false)},
			{Number: "312-555-0199"},
		}},
	}
	b, err := json.Marshal(in)
	if err != nil {
		fmt.Println(err)
		return
	}
	var out enformion.EnrichContactResult
	if err = json.Unmarshal(b, &out); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(reflect.DeepEqual(in, out))
	fmt.Println(*out.Person.Phones[0].IsConnected, out.Person.Phones[1].IsConnected == nil)
	fmt.Println(string(b))
	// Output:
	// true
	// false true
	// {"person":{"phones":[{"number":"(312) 555-0142","isConnected":false},{"number":"312-555-0199"}]},"identityScore":90}
}

// exHandler is an application's HTTP handler that enriches a contact under the
// request's context and its own deadline, and logs only the error's kind and
// status.
type exHandler struct {
	enricher exContactEnricher
	log      *slog.Logger
}

// HandleEnrich answers with the provider's person, or 204 when the response
// carried none.
func (s *exHandler) HandleEnrich(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	res, err := s.enricher.EnrichContact(ctx, enformion.EnrichContactRequest{
		FirstName: r.FormValue("first_name"),
		LastName:  r.FormValue("last_name"),
		Email:     r.FormValue("email"),
		Phone:     r.FormValue("phone"),
	})
	if err != nil {
		s.log.WarnContext(ctx, "contact enrichment failed", "error", err) // Error() holds no personal data
		status := exStatusFor(err)
		http.Error(w, http.StatusText(status), status)
		return
	}
	if res.Person == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(res.Person); err != nil {
		s.log.WarnContext(ctx, "writing the enrichment response failed", "error", err)
	}
}

// exStatusFor maps an enrichment error onto the application's own status.
func exStatusFor(err error) int {
	switch {
	case errors.Is(err, enformion.ErrInvalidRequest):
		return http.StatusBadRequest
	case errors.Is(err, enformion.ErrRateLimited):
		return http.StatusTooManyRequests
	case errors.Is(err, enformion.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout
	}
	return http.StatusBadGateway
}

// Example_httpHandler wires the handler into a server. It is compiled but
// never run.
func Example_httpHandler() {
	client, err := enformion.NewClient("example-key-name", "example-key-password")
	if err != nil {
		fmt.Println("config:", err)
		return
	}
	h := &exHandler{enricher: client, log: slog.Default()}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /enrich", h.HandleEnrich)
	srv := &http.Server{Addr: "127.0.0.1:8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	_ = srv // in an application: srv.ListenAndServe()
}
