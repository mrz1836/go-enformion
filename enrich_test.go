package enformion_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion"
	"github.com/mrz1836/go-enformion/internal/testutil"
)

func TestEnrichMatchFixture(t *testing.T) {
	c, srv := newServerClient(t, respond(http.StatusOK, testutil.Fixture(t, "provider/match.json")))
	res, err := c.EnrichContact(t.Context(), validRequest())
	require.NoError(t, err)
	assert.Equal(t, 1, srv.Hits())

	require.NotNil(t, res.Person)
	p := res.Person
	assert.Equal(t, "G-1000000000000000042", p.PersonID)
	assert.Equal(t, enformion.Name{FirstName: "Avery", MiddleName: "Q", LastName: "Example"}, p.Name)
	assert.Equal(t, "41", p.Age)
	assert.Equal(t, "6/1985", p.PartialDOB)
	require.Len(t, p.Addresses, 2)
	assert.Equal(t, enformion.Address{
		Street: "200 Placeholder Rd", Unit: "4B", City: "Testville", State: "OH", Zip: "45001",
		FirstReportedDate: "1/9/2004", LastReportedDate: "2/27/2011",
	}, p.Addresses[1])
	require.Len(t, p.Phones, 2)
	assert.Equal(t, "(312) 555-0142", p.Phones[0].Number)
	require.NotNil(t, p.Phones[0].IsConnected)
	assert.True(t, *p.Phones[0].IsConnected)
	require.NotNil(t, p.Phones[1].IsConnected)
	assert.False(t, *p.Phones[1].IsConnected)
	require.Len(t, p.Emails, 2)
	assert.Equal(t, "avery.example@example.com", p.Emails[0].Address)
	assert.Nil(t, p.Emails[1].IsValidated, "a flag the provider did not report stays nil")
	require.NotNil(t, res.IdentityScore)
	assert.Equal(t, 87, *res.IdentityScore)
	assert.Empty(t, res.RequestID, "the fixture carries no requestId")
	assert.Equal(t, enformion.ProviderDiagnostics{}, res.Diagnostics)
}

func TestEnrichNoPersonFixture(t *testing.T) {
	c, _ := newServerClient(t, respond(http.StatusOK, testutil.Fixture(t, "synthetic/no-person.json")))
	res, err := c.EnrichContact(t.Context(), validRequest())
	require.NoError(t, err)
	assert.Nil(t, res.Person)
	assert.Equal(t, "synthetic-request-0001", res.RequestID)
	require.NotNil(t, res.IdentityScore)
	assert.Zero(t, *res.IdentityScore)
	require.Len(t, res.Diagnostics.Warnings, 1)
	assert.JSONEq(t, `"Synthetic warning: no person was returned."`, string(res.Diagnostics.Warnings[0]))
	assert.Nil(t, res.Diagnostics.InputErrors)
}

func TestEnrichIsErrorFixture(t *testing.T) {
	tests := []struct {
		name string
		opts []enformion.Option
		want enformion.ProviderDiagnostics
	}{
		{name: "diagnostics off by default"},
		{
			name: "diagnostics opted in",
			opts: []enformion.Option{enformion.WithProviderDiagnostics()},
			want: enformion.ProviderDiagnostics{
				Message: "Synthetic message: the request was refused.",
				InputErrors: []json.RawMessage{
					json.RawMessage(`{"field":"Phone","message":"Synthetic input error."}`),
					json.RawMessage(`"Synthetic input error as a string."`),
				},
				Warnings: []json.RawMessage{json.RawMessage(`7`)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, srv := newServerClient(t, respond(http.StatusOK, testutil.Fixture(t, "synthetic/is-error.json")), tt.opts...)
			res, err := c.EnrichContact(t.Context(), validRequest())
			assert.Nil(t, res)
			require.ErrorIs(t, err, enformion.ErrRejected)
			ce := callError(t, err)
			assert.Equal(t, enformion.FailureRejected, ce.Kind)
			assert.Equal(t, http.StatusOK, ce.StatusCode)
			assert.Equal(t, "synthetic-request-0002", ce.RequestID)
			assert.Equal(t, tt.want, ce.Diagnostics())
			assert.Equal(t, 1, srv.Hits())
		})
	}
}

func TestEnrichStatusTable(t *testing.T) {
	tests := []struct {
		status   int
		kind     enformion.FailureKind
		sentinel error
	}{
		{http.StatusCreated, enformion.FailureUnexpectedResponse, enformion.ErrUnexpectedResponse},
		{http.StatusNoContent, enformion.FailureUnexpectedResponse, enformion.ErrUnexpectedResponse},
		{http.StatusNotModified, enformion.FailureUnexpectedResponse, enformion.ErrUnexpectedResponse},
		{http.StatusBadRequest, enformion.FailureRejected, enformion.ErrRejected},
		{http.StatusUnauthorized, enformion.FailureUnauthorized, enformion.ErrUnauthorized},
		{http.StatusForbidden, enformion.FailureUnauthorized, enformion.ErrUnauthorized},
		{http.StatusNotFound, enformion.FailureRejected, enformion.ErrRejected},
		{http.StatusTeapot, enformion.FailureRejected, enformion.ErrRejected},
		{http.StatusTooManyRequests, enformion.FailureRateLimited, enformion.ErrRateLimited},
		{http.StatusInternalServerError, enformion.FailureUpstream, enformion.ErrUpstream},
		{http.StatusServiceUnavailable, enformion.FailureUpstream, enformion.ErrUpstream},
		{600, enformion.FailureUnexpectedResponse, enformion.ErrUnexpectedResponse},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.status), func(t *testing.T) {
			requestID := "en-req-" + strconv.Itoa(tt.status)
			body := []byte(`{"requestId":"` + requestID + `","message":"refused","isError":true}`)
			c, srv := newServerClient(t, respond(tt.status, body))
			res, err := c.EnrichContact(t.Context(), validRequest())
			assert.Nil(t, res, "a refusal is never a result, nor a missing person")
			require.ErrorIs(t, err, tt.sentinel)
			ce := callError(t, err)
			assert.Equal(t, tt.kind, ce.Kind)
			assert.Equal(t, tt.status, ce.StatusCode)
			assert.Equal(t, 1, srv.Hits(), "exactly one attempt")
			if tt.status == http.StatusNoContent || tt.status == http.StatusNotModified {
				assert.Empty(t, ce.RequestID, "no body, no request ID")
			} else {
				assert.Equal(t, requestID, ce.RequestID)
			}
			assert.Equal(t, enformion.ProviderDiagnostics{}, ce.Diagnostics(), "diagnostics are off by default")
			assert.Equal(t, "enformion: "+string(tt.kind)+" (HTTP "+strconv.Itoa(tt.status)+")", err.Error())
		})
	}
}

func TestEnrichMalformedAndUnexpectedBodies(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		sentinel error
		kind     enformion.FailureKind
	}{
		{"invalid JSON", `{"isError":false`, enformion.ErrMalformedResponse, enformion.FailureMalformedResponse},
		{"trailing data", `{"isError":false} {}`, enformion.ErrMalformedResponse, enformion.FailureMalformedResponse},
		{"invalid UTF-8", "{\"isError\":false,\"message\":\"\xff\"}", enformion.ErrMalformedResponse, enformion.FailureMalformedResponse},
		{"byte order mark", "\xef\xbb\xbf{\"isError\":false}", enformion.ErrMalformedResponse, enformion.FailureMalformedResponse},
		{"duplicate key", `{"isError":false,"isError":false}`, enformion.ErrMalformedResponse, enformion.FailureMalformedResponse},
		{"wrong field type", `{"isError":false,"identityScore":"90"}`, enformion.ErrMalformedResponse, enformion.FailureMalformedResponse},
		{"empty body", ``, enformion.ErrMalformedResponse, enformion.FailureMalformedResponse},
		{"array", `[]`, enformion.ErrUnexpectedResponse, enformion.FailureUnexpectedResponse},
		{"string", `"ok"`, enformion.ErrUnexpectedResponse, enformion.FailureUnexpectedResponse},
		{"null", `null`, enformion.ErrUnexpectedResponse, enformion.FailureUnexpectedResponse},
		{"empty object", `{}`, enformion.ErrUnexpectedResponse, enformion.FailureUnexpectedResponse},
		{"isError missing", `{"person":null}`, enformion.ErrUnexpectedResponse, enformion.FailureUnexpectedResponse},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, srv := newServerClient(t, respond(http.StatusOK, []byte(tt.body)))
			res, err := c.EnrichContact(t.Context(), validRequest())
			assert.Nil(t, res)
			require.ErrorIs(t, err, tt.sentinel)
			ce := callError(t, err)
			assert.Equal(t, tt.kind, ce.Kind)
			assert.Equal(t, http.StatusOK, ce.StatusCode)
			assert.Equal(t, 1, srv.Hits(), "exactly one attempt")
		})
	}
}

func TestEnrichZeroCalls(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Minute))
	defer cancelExpired()
	var nilCtx context.Context

	invalid := validRequest()
	invalid.Email = ""

	tests := []struct {
		name     string
		ctx      context.Context //nolint:containedctx // each case runs with its own context
		req      enformion.EnrichContactRequest
		sentinel error
		field    string
		reason   string
		kind     enformion.FailureKind
	}{
		{
			name: "invalid request", ctx: context.Background(), req: invalid,
			sentinel: enformion.ErrInvalidRequest, field: "request", reason: "insufficient_categories",
		},
		{name: "nil context", ctx: nilCtx, req: validRequest(), sentinel: enformion.ErrInvalidRequest, field: "ctx", reason: "nil"},
		{
			name: "nil context before an invalid request", ctx: nilCtx, req: enformion.EnrichContactRequest{},
			sentinel: enformion.ErrInvalidRequest, field: "ctx", reason: "nil",
		},
		{
			name: "invalid request beats a canceled context", ctx: canceled, req: invalid,
			sentinel: enformion.ErrInvalidRequest, field: "request", reason: "insufficient_categories",
		},
		{name: "canceled context", ctx: canceled, req: validRequest(), sentinel: context.Canceled, kind: enformion.FailureCanceled},
		{name: "expired deadline", ctx: expired, req: validRequest(), sentinel: context.DeadlineExceeded, kind: enformion.FailureDeadlineExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct := &testutil.CountingTransport{Next: testutil.StaticTransport(http.StatusOK, string(testutil.Fixture(t, "provider/match.json")))}
			c := newTransportClient(t, ct)
			res, err := c.EnrichContact(tt.ctx, tt.req)
			assert.Nil(t, res)
			require.ErrorIs(t, err, tt.sentinel)
			assert.Zero(t, ct.Attempts(), "nothing may be sent")
			if tt.kind == "" {
				ve := validationError(t, err)
				assert.Equal(t, tt.field, ve.Field)
				assert.Equal(t, tt.reason, ve.Reason)
				return
			}
			ce := callError(t, err)
			assert.Equal(t, tt.kind, ce.Kind)
			assert.Zero(t, ce.StatusCode)
			assert.NotErrorIs(t, err, enformion.ErrTimeout, "a caller's context is never the SDK's timeout")
		})
	}
}

func TestEnrichRequestUnchanged(t *testing.T) {
	c, _ := newServerClient(t, respond(http.StatusOK, testutil.Fixture(t, "provider/match.json")))
	req := validRequest()
	req.MiddleName = "Q"
	req.Phone = "(312) 555-0142"
	req.AddressLine1 = "100 Sample Way"
	req.AddressLine2 = "Exampleton, OH"
	req.DOB = "06/01/1985"
	req.Age = 41
	req.SessionID = "batch-7f3a9c-0002"
	before := req
	_, err := c.EnrichContact(t.Context(), req)
	require.NoError(t, err)
	assert.Equal(t, before, req)
}
