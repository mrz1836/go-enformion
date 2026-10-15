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
