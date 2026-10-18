package enformion_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion"
	"github.com/mrz1836/go-enformion/internal/testutil"
)

// Leak markers. Each is distinct, so a failure names what leaked.
const (
	pvSessionID    = "pv-session-marker-4d1e"
	pvFirstName    = "PvFirstMarker"
	pvMiddleName   = "PvMiddleMarker"
	pvLastName     = "PvLastMarker"
	pvDOB          = "02/03/1981"
	pvAddressLine1 = "PvLineOneMarker"
	pvAddressLine2 = "PvLineTwoMarker"
	pvPhone        = "PvPhoneMarker"
	pvEmail        = "pv.email.marker@example.com"

	pvRespName    = "PvRespNameMarker"
	pvRespScore   = "PvRespScoreMarker"
	pvRespNumber  = "424242424242"
	pvUnknownData = "PvUnknownValueMarker"
	pvInvalidJSON = "PvInvalidJSONMarker"
	pvNotObject   = "PvNotObjectMarker"
	pvMessage     = "PvProviderMessageMarker"
	pvInputError  = "PvInputErrorMarker"
	pvWarning     = "PvWarningMarker"
	pvLocation    = "pvlocationmarker"
	pvTransport   = "PvTransportMarker"
	pvTooLarge    = "PvTooLargeMarker"
	pvBadValue    = "PvBadValueMarker"
	pvBadURL      = "pv-url-marker"

	// pvRequestID is the provider request ID of pvDiagBody.
	pvRequestID = "pv-request-0001"

	// pvDiagBody is a failure envelope whose provider text holds markers and
	// echoes a request value, as provider text may.
	pvDiagBody = `{"isError":true,"requestId":"` + pvRequestID + `","message":"` + pvMessage + ` ` + pvFirstName +
		`","error":{"inputErrors":[{"field":"Phone","value":"` + pvInputError + `"}],"warnings":["` + pvWarning + `"]}}`
)

// errPvEcho is the transport failure whose message echoes the request.
var errPvEcho = errors.New("pv transport echo")

// pvRequest returns a valid request whose every field is a marker.
func pvRequest() enformion.EnrichContactRequest {
	return enformion.EnrichContactRequest{
		FirstName:    pvFirstName,
		MiddleName:   pvMiddleName,
		LastName:     pvLastName,
		DOB:          pvDOB,
		Age:          37,
		AddressLine1: pvAddressLine1,
		AddressLine2: pvAddressLine2,
		Phone:        pvPhone,
		Email:        pvEmail,
		SessionID:    pvSessionID,
	}
}

// pvSecrets returns every marker that must never print.
func pvSecrets() []string {
	return []string{
		testKeyName, testKeyPassword, pvSessionID,
		pvFirstName, pvMiddleName, pvLastName, pvDOB, pvAddressLine1, pvAddressLine2, pvPhone, pvEmail,
		pvRespName, pvRespScore, pvRespNumber, pvUnknownData, pvInvalidJSON, pvNotObject,
		pvMessage, pvInputError, pvWarning, pvLocation, pvTransport, pvTooLarge, pvBadValue, pvBadURL,
	}
}

// pvFlow is one real call that fails with kind.
type pvFlow struct {
	name string
	kind enformion.FailureKind
	diag bool // the response carries provider diagnostics
	call func(t *testing.T, opts ...enformion.Option) error
}

// pvEnrich calls EnrichContact through rt with the marker request.
func pvEnrich(ctx context.Context, t *testing.T, rt http.RoundTripper, opts ...enformion.Option) error {
	t.Helper()
	_, err := newTransportClient(t, rt, opts...).EnrichContact(ctx, pvRequest())
	return err
}

// pvStatic returns a flow body that answers with status and body.
func pvStatic(status int, body string, flowOpts ...enformion.Option) func(*testing.T, ...enformion.Option) error {
	return func(t *testing.T, opts ...enformion.Option) error {
		return pvEnrich(t.Context(), t, testutil.StaticTransport(status, body), append(flowOpts, opts...)...)
	}
}

// pvEcho fails with an error that quotes the credential headers, the session
// ID and the request body.
func pvEcho(r *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(r.Body)
	return nil, fmt.Errorf("%w: %s %s %s %s %s", errPvEcho,
		r.Header.Get("Galaxy-Ap-Name"), r.Header.Get("Galaxy-Ap-Password"),
		r.Header.Get("Galaxy-Client-Session-Id"), body, pvTransport)
}

// pvFlows returns a real flow for every failure kind.
func pvFlows() []pvFlow {
	tooLarge := `{"isError":false,"message":"` + strings.Repeat(pvTooLarge, 16) + `"}`
	return []pvFlow{
		{name: "unauthorized 401", kind: enformion.FailureUnauthorized, diag: true, call: pvStatic(http.StatusUnauthorized, pvDiagBody)},
		{name: "unauthorized 403", kind: enformion.FailureUnauthorized, diag: true, call: pvStatic(http.StatusForbidden, pvDiagBody)},
		{name: "rate limited 429", kind: enformion.FailureRateLimited, diag: true, call: pvStatic(http.StatusTooManyRequests, pvDiagBody)},
		{name: "rejected 400", kind: enformion.FailureRejected, diag: true, call: pvStatic(http.StatusBadRequest, pvDiagBody)},
		{name: "rejected isError true", kind: enformion.FailureRejected, diag: true, call: pvStatic(http.StatusOK, pvDiagBody)},
		{name: "upstream 500", kind: enformion.FailureUpstream, diag: true, call: pvStatic(http.StatusInternalServerError, pvDiagBody)},
		{name: "unexpected redirect", kind: enformion.FailureUnexpectedResponse, call: func(t *testing.T, opts ...enformion.Option) error {
			// The Location and the body both hold markers.
			rt := redirectTransport("https://"+pvLocation+".example.com/next?k="+pvLocation,
				io.NopCloser(strings.NewReader(`{"message":"`+pvLocation+`"}`)), -1)
			return pvEnrich(t.Context(), t, rt, opts...)
		}},
		{name: "unexpected not an object", kind: enformion.FailureUnexpectedResponse, call: pvStatic(http.StatusOK, `["`+pvNotObject+`"]`)},
		{name: "unexpected isError missing", kind: enformion.FailureUnexpectedResponse, call: pvStatic(http.StatusOK,
			`{"person":{"name":{"firstName":"`+pvRespName+`"}},"message":"`+pvMessage+`"}`)},
		{name: "unexpected unknown keys", kind: enformion.FailureUnexpectedResponse, call: pvStatic(http.StatusOK,
			`{"isError":false,"person":{"fullName":"`+pvUnknownData+`"}}`)},
		{name: "malformed wrong number type", kind: enformion.FailureMalformedResponse, call: pvStatic(http.StatusOK,
			`{"isError":false,"person":{"age":`+pvRespNumber+`,"name":{"firstName":"`+pvRespName+`"}}}`)},
		{name: "malformed wrong string type", kind: enformion.FailureMalformedResponse, call: pvStatic(http.StatusOK,
			`{"isError":false,"identityScore":"`+pvRespScore+`"}`)},
		{name: "malformed invalid JSON", kind: enformion.FailureMalformedResponse, call: pvStatic(http.StatusOK,
			`{"isError":false,"message":"`+pvInvalidJSON)},
		{name: "malformed duplicate key", kind: enformion.FailureMalformedResponse, call: pvStatic(http.StatusOK,
			`{"isError":false,"isError":false,"person":{"name":{"firstName":"`+pvRespName+`"}}}`)},
		{name: "too large declared", kind: enformion.FailureResponseTooLarge, call: pvStatic(http.StatusOK, tooLarge,
			enformion.WithMaxResponseBytes(64))},
		{name: "too large undeclared", kind: enformion.FailureResponseTooLarge, call: func(t *testing.T, opts ...enformion.Option) error {
			rt := testutil.Transport(http.StatusOK, -1, func() io.ReadCloser { return io.NopCloser(strings.NewReader(tooLarge)) })
			return pvEnrich(t.Context(), t, rt, append([]enformion.Option{enformion.WithMaxResponseBytes(64)}, opts...)...)
		}},
		{name: "transport echo", kind: enformion.FailureTransport, call: func(t *testing.T, opts ...enformion.Option) error {
			return pvEnrich(t.Context(), t, testutil.RoundTripFunc(pvEcho), opts...)
		}},
		{name: "timeout", kind: enformion.FailureTimeout, call: func(t *testing.T, opts ...enformion.Option) error {
			return pvEnrich(t.Context(), t, testutil.RoundTripFunc(testutil.BlockUntilDone),
				append([]enformion.Option{enformion.WithTimeout(time.Millisecond)}, opts...)...)
		}},
		{name: "canceled in flight", kind: enformion.FailureCanceled, call: func(t *testing.T, opts ...enformion.Option) error {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			rt := testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
				cancel()
				return testutil.BlockUntilDone(r)
			})
			return pvEnrich(ctx, t, rt, opts...)
		}},
		{name: "canceled before send", kind: enformion.FailureCanceled, call: func(t *testing.T, opts ...enformion.Option) error {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			return pvEnrich(ctx, t, testutil.StaticTransport(http.StatusOK, pvDiagBody), opts...)
		}},
		{name: "deadline exceeded", kind: enformion.FailureDeadlineExceeded, call: func(t *testing.T, opts ...enformion.Option) error {
			ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
			defer cancel()
			return pvEnrich(ctx, t, testutil.RoundTripFunc(testutil.BlockUntilDone), opts...)
		}},
	}
}

func TestPrivacyCallErrorsNeverPrintSecrets(t *testing.T) {
	secrets := pvSecrets()
	for _, withDiag := range []bool{false, true} {
		var opts []enformion.Option
		if withDiag {
			opts = append(opts, enformion.WithProviderDiagnostics())
		}
		for _, f := range pvFlows() {
			t.Run(fmt.Sprintf("%s diagnostics=%t", f.name, withDiag), func(t *testing.T) {
				err := f.call(t, opts...)
				ce := callError(t, err)
				require.Equal(t, f.kind, ce.Kind)
				for _, s := range testutil.Shapes(ce) {
					testutil.AssertNeverPrints(t, s.Value, secrets...)
				}
				assertErrorClean(t, err, secrets...)
			})
		}
	}
}

// TestPrivacyDiagnosticsOnlyWithOptIn proves that every flow whose response
// carries diagnostics keeps them on the error only with the option, reachable
// only through Diagnostics, which returns a deep copy.
func TestPrivacyDiagnosticsOnlyWithOptIn(t *testing.T) {
	want := enformion.ProviderDiagnostics{
		Message:     pvMessage + " " + pvFirstName,
		InputErrors: []json.RawMessage{json.RawMessage(`{"field":"Phone","value":"` + pvInputError + `"}`)},
		Warnings:    []json.RawMessage{json.RawMessage(`"` + pvWarning + `"`)},
	}
	for _, f := range pvFlows() {
		if !f.diag {
			continue
		}
		t.Run(f.name, func(t *testing.T) {
			plain := callError(t, f.call(t))
			assert.Equal(t, pvRequestID, plain.RequestID)
			assert.Equal(t, enformion.ProviderDiagnostics{}, plain.Diagnostics(), "diagnostics are off by default")

			err := f.call(t, enformion.WithProviderDiagnostics())
			opted := callError(t, err)
			assert.Equal(t, pvRequestID, opted.RequestID)
			got := opted.Diagnostics()
			assert.Equal(t, want, got, "provider text may echo request values")

			// Mutating a returned copy never changes the error.
			require.Len(t, got.InputErrors, 1)
			require.Len(t, got.Warnings, 1)
			got.Message = "changed"
			got.InputErrors[0][0] = 'X'
			got.InputErrors = append(got.InputErrors, []byte(`"added"`))
			got.Warnings[0] = nil
			assert.Equal(t, want, opted.Diagnostics(), "Diagnostics returns a deep copy")

			// Reflection never reaches the diagnostics, opted in or not.
			for _, e := range []error{plain, err} {
				testutil.AssertRedacted(t, strings.Join(testutil.ReflectStrings(e), "\n"),
					[]string{pvMessage, pvInputError, pvWarning}, "reflection walk")
			}
		})
	}
}

func TestPrivacyValidationErrorsNeverPrintSecrets(t *testing.T) {
	secrets := pvSecrets()
	newWith := func(opts ...enformion.Option) error {
		_, err := enformion.NewClient(testKeyName, testKeyPassword, opts...)
		return err
	}
	enrich := func(t *testing.T, mutate func(*enformion.EnrichContactRequest)) error {
		req := pvRequest()
		mutate(&req)
		_, err := newTransportClient(t, testutil.StaticTransport(http.StatusOK, pvDiagBody)).EnrichContact(t.Context(), req)
		return err
	}
	var nilCtx context.Context
	tests := []struct {
		name     string
		sentinel error
		run      func(t *testing.T) error
	}{
		{"empty key name", enformion.ErrInvalidConfig, func(*testing.T) error {
			_, err := enformion.NewClient("", testKeyPassword)
			return err
		}},
		{"key name leading space", enformion.ErrInvalidConfig, func(*testing.T) error {
			_, err := enformion.NewClient(" "+pvBadValue, testKeyPassword)
			return err
		}},
		{"key name CRLF", enformion.ErrInvalidConfig, func(*testing.T) error {
			_, err := enformion.NewClient(pvBadValue+"\r\n", testKeyPassword)
			return err
		}},
		{"key password NUL", enformion.ErrInvalidConfig, func(*testing.T) error {
			_, err := enformion.NewClient(testKeyName, pvBadValue+"\x00")
			return err
		}},
		{"key password non-ASCII", enformion.ErrInvalidConfig, func(*testing.T) error {
			_, err := enformion.NewClient(testKeyName, pvBadValue+"é")
			return err
		}},
		{"key password too long", enformion.ErrInvalidConfig, func(*testing.T) error {
			_, err := enformion.NewClient(testKeyName, strings.Repeat(pvBadValue, 100))
			return err
		}},
		{"test URL http", enformion.ErrInvalidConfig, func(*testing.T) error {
			return newWith(enformion.WithTestBaseURL("http://" + pvBadURL + ".example.com/" + pvBadValue))
		}},
		{"test URL userinfo", enformion.ErrInvalidConfig, func(*testing.T) error {
			return newWith(enformion.WithTestBaseURL("https://" + pvBadValue + ":" + pvBadValue + "@127.0.0.1:8443"))
		}},
		{"test URL localhost", enformion.ErrInvalidConfig, func(*testing.T) error {
			return newWith(enformion.WithTestBaseURL("https://localhost/" + pvBadValue))
		}},
		{"timeout zero", enformion.ErrInvalidConfig, func(*testing.T) error { return newWith(enformion.WithTimeout(0)) }},
		{"timeout too long", enformion.ErrInvalidConfig, func(*testing.T) error { return newWith(enformion.WithTimeout(time.Hour)) }},
		{"max bytes zero", enformion.ErrInvalidConfig, func(*testing.T) error { return newWith(enformion.WithMaxResponseBytes(0)) }},
		{"nil HTTP client", enformion.ErrInvalidConfig, func(*testing.T) error { return newWith(enformion.WithHTTPClient(nil)) }},
		{"zero client", enformion.ErrInvalidConfig, func(t *testing.T) error {
			_, err := (&enformion.Client{}).EnrichContact(t.Context(), pvRequest())
			return err
		}},
		{"nil client", enformion.ErrInvalidConfig, func(t *testing.T) error {
			var c *enformion.Client
			_, err := c.EnrichContact(t.Context(), pvRequest())
			return err
		}},
		{"nil context", enformion.ErrInvalidRequest, func(t *testing.T) error {
			_, err := newTransportClient(t, testutil.StaticTransport(http.StatusOK, pvDiagBody)).EnrichContact(nilCtx, pvRequest())
			return err
		}},
		{"control character", enformion.ErrInvalidRequest, func(t *testing.T) error {
			return enrich(t, func(r *enformion.EnrichContactRequest) { r.FirstName = pvBadValue + "\t" })
		}},
		{"invalid UTF-8", enformion.ErrInvalidRequest, func(t *testing.T) error {
			return enrich(t, func(r *enformion.EnrichContactRequest) { r.LastName = pvBadValue + "\xff" })
		}},
		{"too long", enformion.ErrInvalidRequest, func(t *testing.T) error {
			return enrich(t, func(r *enformion.EnrichContactRequest) { r.AddressLine1 = strings.Repeat(pvBadValue, 20) })
		}},
		{"blank", enformion.ErrInvalidRequest, func(t *testing.T) error {
			return enrich(t, func(r *enformion.EnrichContactRequest) { r.MiddleName = "   " })
		}},
		{"DOB format", enformion.ErrInvalidRequest, func(t *testing.T) error {
			return enrich(t, func(r *enformion.EnrichContactRequest) { r.DOB = pvBadValue })
		}},
		{"age out of range", enformion.ErrInvalidRequest, func(t *testing.T) error {
			return enrich(t, func(r *enformion.EnrichContactRequest) { r.Age = 151 })
		}},
		{"session ID with a space", enformion.ErrInvalidRequest, func(t *testing.T) error {
			return enrich(t, func(r *enformion.EnrichContactRequest) { r.SessionID = pvBadValue + " x" })
		}},
		{"insufficient categories", enformion.ErrInvalidRequest, func(t *testing.T) error {
			return enrich(t, func(r *enformion.EnrichContactRequest) {
				*r = enformion.EnrichContactRequest{FirstName: pvFirstName, MiddleName: pvMiddleName, Email: pvEmail}
			})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			require.ErrorIs(t, err, tt.sentinel)
			ve := validationError(t, err)
			for _, s := range testutil.Shapes(ve) {
				testutil.AssertNeverPrints(t, s.Value, secrets...)
			}
			assertErrorClean(t, err, secrets...)
		})
	}
}

func TestPrivacyClientNeverPrintsCredentials(t *testing.T) {
	creds := []string{testKeyName, testKeyPassword, "other-name-marker-55aa", "other-pass-marker-66bb"}
	every, srv := newServerClient(t, respond(http.StatusOK, testutil.Fixture(t, "provider/match.json")),
		enformion.WithTimeout(5*time.Second),
		enformion.WithMaxResponseBytes(1<<16),
		enformion.WithProviderDiagnostics(),
	)
	clients := map[string]*enformion.Client{
		"defaults":     newClient(t),
		"every option": every,
	}
	other, err := enformion.NewClient("other-name-marker-55aa", "other-pass-marker-66bb")
	require.NoError(t, err)

	for name, c := range clients {
		t.Run(name, func(t *testing.T) {
			for _, s := range testutil.Shapes(c) {
				testutil.AssertNeverPrints(t, s.Value, creds...)
			}
			testutil.AssertRedacted(t, strings.Join(testutil.ReflectStrings(c), "\n"), creds, "reflection walk")
			testutil.AssertDiffRedacted(t, c, other, creds...)
		})
	}

	t.Run("after a successful call", func(t *testing.T) {
		res, err := every.EnrichContact(t.Context(), pvRequest())
		require.NoError(t, err)
		require.NotNil(t, res.Person, "results legitimately hold the response's personal data")
		for _, s := range testutil.Shapes(every) {
			testutil.AssertNeverPrints(t, s.Value, creds...)
		}
		require.Len(t, srv.Requests(), 1)
		assert.Equal(t, testKeyPassword, srv.Requests()[0].Header.Get("Galaxy-Ap-Password"),
			"the credential reaches the server, and only the server")
	})
}

func TestPrivacyCallErrorDiffsNeverPrintSecrets(t *testing.T) {
	secrets := pvSecrets()
	flows := pvFlows()
	diag := enformion.WithProviderDiagnostics()
	for i := 1; i < len(flows); i++ {
		a, b := flows[i-1], flows[i]
		t.Run(a.name+" vs "+b.name, func(t *testing.T) {
			ea := callError(t, a.call(t, diag))
			eb := callError(t, b.call(t, diag))
			testutil.AssertDiffRedacted(t, ea, eb, secrets...)
		})
	}
}

// Person Search leak markers. Each is distinct, so a failure names what
// leaked; tokens and the IP are markers that still validate.
const (
	pvAKA        = "PvAkaMarker"
	pvRelative   = "PvRelativeMarker"
	pvCounty     = "PvCountyMarker"
	pvTahoeID    = "G-PvTahoeMarker"
	pvClientIP   = "192.0.2.77"
	pvInclude    = "PvIncludeMarker"
	pvFilter     = "PvFilterMarker"
	pvDOBFormat  = "PvDobFormatMarker"
	pvMode       = "PvModeMarker"
	pvRespTahoe  = "PvRespTahoeMarker"
	pvRespNested = "PvRespNestedMarker"
	pvExtraKey   = "pvExtraKeyMarker"
	pvExtraValue = "PvExtraValueMarker"
)

// pvSearchRequest returns a valid Person Search request whose every field is
// a marker.
func pvSearchRequest() enformion.PersonSearchRequest {
	return enformion.PersonSearchRequest{
		Mode:      enformion.PersonSearchModePerson,
		FirstName: pvFirstName, MiddleName: pvMiddleName, LastName: pvLastName,
		AKAs: []enformion.PersonSearchRequestName{{FirstName: pvAKA}},
		DOB:  pvDOB, Age: 37,
		Addresses: []enformion.PersonSearchRequestAddress{
			{AddressLine1: pvAddressLine1, AddressLine2: pvAddressLine2, County: pvCounty},
		},
		Email: pvEmail, Phone: pvPhone, ClientIP: pvClientIP,
		Relatives:     []enformion.PersonSearchRequestName{{LastName: pvRelative}},
		TahoeIDs:      []string{pvTahoeID},
		DOBFormat:     pvDOBFormat,
		Includes:      []enformion.PersonSearchInclude{pvInclude},
		FilterOptions: []enformion.PersonSearchFilter{pvFilter},
		SessionID:     pvSessionID,
	}
}

// pvSearchSecrets returns every marker that must never print from a Person
// Search flow.
func pvSearchSecrets() []string {
	return append(pvSecrets(),
		pvAKA, pvRelative, pvCounty, pvTahoeID, pvClientIP, pvInclude, pvFilter, pvDOBFormat, pvMode,
		pvRespTahoe, pvRespNested, pvExtraKey, pvExtraValue)
}

// pvSearch calls SearchPersons through rt with the marker request.
func pvSearch(ctx context.Context, t *testing.T, rt http.RoundTripper, opts ...enformion.Option) error {
	t.Helper()
	_, err := newTransportClient(t, rt, opts...).SearchPersons(ctx, pvSearchRequest())
	return err
}

// pvSearchStatic returns a flow body that answers a search with status and
// body.
func pvSearchStatic(status int, body string, flowOpts ...enformion.Option) func(*testing.T, ...enformion.Option) error {
	return func(t *testing.T, opts ...enformion.Option) error {
		return pvSearch(t.Context(), t, testutil.StaticTransport(status, body), append(flowOpts, opts...)...)
	}
}

// pvSearchPerson is a person whose members hold markers at every nested
// level: names, addresses and their markers, phones, emails and their
// engagement data, relatives, associates and an undocumented member.
const pvSearchPerson = `{"tahoeId":"` + pvRespTahoe + `","name":{"firstName":"` + pvRespName + `","rawNames":["` + pvRespNested + `"]},` +
	`"addresses":[{"fullAddress":"` + pvRespNested + `","highRiskMarker":{"sic":"` + pvRespNested + `"}}],` +
	`"phoneNumbers":[{"phoneNumber":"` + pvRespNested + `"}],` +
	`"emailAddresses":[{"emailAddress":"` + pvRespNested + `","emailEngagementData":{"naics":["` + pvRespNested + `"]}}],` +
	`"relativesSummary":[{"firstName":"` + pvRespNested + `"}],` +
	`"associates":[{"tahoeId":"` + pvRespNested + `","addresses":[{"zip":"` + pvRespNested + `"}]}],` +
	`"` + pvExtraKey + `":"` + pvExtraValue + `"}`

// pvSearchFlows returns a real Person Search flow for every failure kind.
func pvSearchFlows() []pvFlow {
	tooLarge := `{"isError":false,"persons":[],"message":"` + strings.Repeat(pvTooLarge, 16) + `"}`
	nestedBad := func(person string) string {
		return `{"isError":false,"persons":[` + pvSearchPerson + `,` + person + `]}`
	}
	emptyAddresses := `{"tahoeId":"` + pvRespTahoe + `","addresses":[` + strings.TrimSuffix(strings.Repeat(`{},`, 1000), ",") + `]}`
	budget := `{"isError":false,"persons":[` + strings.TrimSuffix(strings.Repeat(emptyAddresses+",", 400), ",") + `]}`
	return []pvFlow{
		{name: "unauthorized 401", kind: enformion.FailureUnauthorized, diag: true, call: pvSearchStatic(http.StatusUnauthorized, pvDiagBody)},
		{name: "rate limited 429", kind: enformion.FailureRateLimited, diag: true, call: pvSearchStatic(http.StatusTooManyRequests, pvDiagBody)},
		{name: "rejected 400", kind: enformion.FailureRejected, diag: true, call: pvSearchStatic(http.StatusBadRequest, pvDiagBody)},
		{name: "rejected isError true", kind: enformion.FailureRejected, diag: true, call: pvSearchStatic(http.StatusOK, pvDiagBody)},
		{name: "upstream 503", kind: enformion.FailureUpstream, diag: true, call: pvSearchStatic(http.StatusServiceUnavailable, pvDiagBody)},
		{name: "unexpected redirect", kind: enformion.FailureUnexpectedResponse, call: func(t *testing.T, opts ...enformion.Option) error {
			rt := redirectTransport("https://"+pvLocation+".example.com/next?k="+pvLocation,
				io.NopCloser(strings.NewReader(`{"message":"`+pvLocation+`"}`)), -1)
			return pvSearch(t.Context(), t, rt, opts...)
		}},
		{name: "unexpected not an object", kind: enformion.FailureUnexpectedResponse, call: pvSearchStatic(http.StatusOK, `["`+pvNotObject+`"]`)},
		{name: "unexpected persons missing", kind: enformion.FailureUnexpectedResponse, call: pvSearchStatic(http.StatusOK,
			`{"isError":false,"person":`+pvSearchPerson+`,"message":"`+pvMessage+`"}`)},
		{name: "unexpected isError missing", kind: enformion.FailureUnexpectedResponse, call: pvSearchStatic(http.StatusOK,
			`{"persons":[`+pvSearchPerson+`]}`)},
		{name: "unexpected unknown keys", kind: enformion.FailureUnexpectedResponse, call: pvSearchStatic(http.StatusOK,
			nestedBad(`{"`+pvExtraKey+`":"`+pvUnknownData+`"}`))},
		{name: "unexpected nested unknown keys", kind: enformion.FailureUnexpectedResponse, call: pvSearchStatic(http.StatusOK,
			nestedBad(`{"tahoeId":"`+pvRespTahoe+`","associates":[{"phoneNumbers":[{"`+pvExtraKey+`":"`+pvUnknownData+`"}]}]}`))},
		{name: "malformed deep wrong type", kind: enformion.FailureMalformedResponse, call: pvSearchStatic(http.StatusOK,
			nestedBad(`{"associates":[{"emailAddresses":[{"emailEngagementData":{"naics":["`+pvRespName+`",`+pvRespNumber+`]}}]}]}`))},
		{name: "malformed wrong string type", kind: enformion.FailureMalformedResponse, call: pvSearchStatic(http.StatusOK,
			`{"isError":false,"persons":[],"counts":{"searchResults":"`+pvRespScore+`"}}`)},
		{name: "malformed invalid JSON", kind: enformion.FailureMalformedResponse, call: pvSearchStatic(http.StatusOK,
			`{"isError":false,"persons":[`+pvSearchPerson+`,{"fullName":"`+pvInvalidJSON)},
		{name: "malformed duplicate key", kind: enformion.FailureMalformedResponse, call: pvSearchStatic(http.StatusOK,
			`{"isError":false,"persons":[`+pvSearchPerson+`],"persons":[]}`)},
		{name: "too large declared", kind: enformion.FailureResponseTooLarge, call: pvSearchStatic(http.StatusOK, tooLarge,
			enformion.WithMaxResponseBytes(64))},
		{name: "too large over the decode budget", kind: enformion.FailureResponseTooLarge, call: pvSearchStatic(http.StatusOK, budget)},
		{name: "too many entries", kind: enformion.FailureResponseTooLarge, call: pvSearchStatic(http.StatusOK,
			`{"isError":false,"persons":[{"tahoeId":"`+pvRespTahoe+`","name":{"rawNames":[`+
				strings.TrimSuffix(strings.Repeat(`"`+pvRespNested+`",`, 1001), ",")+`]}}]}`)},
		{name: "transport echo", kind: enformion.FailureTransport, call: func(t *testing.T, opts ...enformion.Option) error {
			return pvSearch(t.Context(), t, testutil.RoundTripFunc(pvEcho), opts...)
		}},
		{name: "timeout", kind: enformion.FailureTimeout, call: func(t *testing.T, opts ...enformion.Option) error {
			return pvSearch(t.Context(), t, testutil.RoundTripFunc(testutil.BlockUntilDone),
				append([]enformion.Option{enformion.WithTimeout(time.Millisecond)}, opts...)...)
		}},
		{name: "canceled in flight", kind: enformion.FailureCanceled, call: func(t *testing.T, opts ...enformion.Option) error {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			rt := testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
				cancel()
				return testutil.BlockUntilDone(r)
			})
			return pvSearch(ctx, t, rt, opts...)
		}},
		{name: "deadline exceeded", kind: enformion.FailureDeadlineExceeded, call: func(t *testing.T, opts ...enformion.Option) error {
			ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
			defer cancel()
			return pvSearch(ctx, t, testutil.RoundTripFunc(testutil.BlockUntilDone), opts...)
		}},
	}
}

func TestPrivacySearchCallErrorsNeverPrintSecrets(t *testing.T) {
	secrets := pvSearchSecrets()
	for _, withDiag := range []bool{false, true} {
		var opts []enformion.Option
		if withDiag {
			opts = append(opts, enformion.WithProviderDiagnostics())
		}
		for _, f := range pvSearchFlows() {
			t.Run(fmt.Sprintf("%s diagnostics=%t", f.name, withDiag), func(t *testing.T) {
				err := f.call(t, opts...)
				ce := callError(t, err)
				require.Equal(t, f.kind, ce.Kind)
				for _, s := range testutil.Shapes(ce) {
					testutil.AssertNeverPrints(t, s.Value, secrets...)
				}
				assertErrorClean(t, err, secrets...)
			})
		}
	}
}

func TestPrivacySearchDiagnosticsOnlyWithOptIn(t *testing.T) {
	for _, f := range pvSearchFlows() {
		if !f.diag {
			continue
		}
		t.Run(f.name, func(t *testing.T) {
			plain := callError(t, f.call(t))
			assert.Equal(t, pvRequestID, plain.RequestID)
			assert.Equal(t, enformion.ProviderDiagnostics{}, plain.Diagnostics(), "diagnostics are off by default")
			opted := callError(t, f.call(t, enformion.WithProviderDiagnostics()))
			assert.Equal(t, pvMessage+" "+pvFirstName, opted.Diagnostics().Message, "provider text may echo request values")
			testutil.AssertRedacted(t, strings.Join(testutil.ReflectStrings(opted), "\n"),
				[]string{pvMessage, pvInputError, pvWarning}, "reflection walk")
		})
	}
}

func TestPrivacySearchValidationErrorsNeverPrintSecrets(t *testing.T) {
	secrets := append(pvSearchSecrets(), pvBadValue)
	search := func(t *testing.T, mutate func(*enformion.PersonSearchRequest)) error {
		req := pvSearchRequest()
		mutate(&req)
		ct := &testutil.CountingTransport{Next: testutil.StaticTransport(http.StatusOK, pvDiagBody)}
		_, err := newTransportClient(t, ct).SearchPersons(t.Context(), req)
		assert.Zero(t, ct.Attempts(), "nothing is sent")
		return err
	}
	tests := []struct {
		name, field string
		mutate      func(*enformion.PersonSearchRequest)
	}{
		{"unsupported mode", "Mode", func(r *enformion.PersonSearchRequest) { r.Mode = pvMode }},
		{"include token", "Includes[0]", func(r *enformion.PersonSearchRequest) { r.Includes[0] = pvInclude + "!" }},
		{"filter token", "FilterOptions[0]", func(r *enformion.PersonSearchRequest) { r.FilterOptions[0] = pvFilter + " x" }},
		{"client IP", "ClientIP", func(r *enformion.PersonSearchRequest) { r.ClientIP = pvBadValue }},
		{"age range", "AgeRange", func(r *enformion.PersonSearchRequest) { r.AgeRange = pvBadValue }},
		{"county control character", "Addresses[0].County", func(r *enformion.PersonSearchRequest) { r.Addresses[0].County = pvCounty + "\x00" }},
		{"AKA invalid UTF-8", "AKAs[0].FirstName", func(r *enformion.PersonSearchRequest) { r.AKAs[0].FirstName = pvAKA + "\xff" }},
		{"relative blank", "Relatives[0].LastName", func(r *enformion.PersonSearchRequest) { r.Relatives[0].LastName = "  " }},
		{"Tahoe ID tab", "TahoeIDs[0]", func(r *enformion.PersonSearchRequest) { r.TahoeIDs[0] = pvTahoeID + "\t" }},
		{"DOB format too long", "DOBFormat", func(r *enformion.PersonSearchRequest) { r.DOBFormat = strings.Repeat(pvDOBFormat, 20) }},
		{"session ID with a space", "SessionID", func(r *enformion.PersonSearchRequest) { r.SessionID = pvBadValue + " x" }},
		{"no criteria", "request", func(r *enformion.PersonSearchRequest) {
			*r = enformion.PersonSearchRequest{Mode: enformion.PersonSearchModeTeaser, DOBFormat: pvDOBFormat, SessionID: pvSessionID}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := search(t, tt.mutate)
			require.ErrorIs(t, err, enformion.ErrInvalidRequest)
			ve := validationError(t, err)
			assert.Equal(t, tt.field, ve.Field)
			for _, s := range testutil.Shapes(ve) {
				testutil.AssertNeverPrints(t, s.Value, secrets...)
			}
			assertErrorClean(t, err, secrets...)
		})
	}
}

func TestPrivacySearchCallErrorDiffsNeverPrintSecrets(t *testing.T) {
	secrets := pvSearchSecrets()
	flows := pvSearchFlows()
	diag := enformion.WithProviderDiagnostics()
	for i := 1; i < len(flows); i++ {
		a, b := flows[i-1], flows[i]
		t.Run(a.name+" vs "+b.name, func(t *testing.T) {
			ea := callError(t, a.call(t, diag))
			eb := callError(t, b.call(t, diag))
			testutil.AssertDiffRedacted(t, ea, eb, secrets...)
		})
	}
}
