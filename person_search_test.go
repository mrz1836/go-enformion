package enformion_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion"
	"github.com/mrz1836/go-enformion/internal/testutil"
)

// psFailingBody is a testutil.Body that fails with err, when set, instead of
// returning io.EOF.
type psFailingBody struct {
	*testutil.Body

	err error
}

func (b *psFailingBody) Read(p []byte) (int, error) {
	n, err := b.Body.Read(p)
	if b.err != nil && errors.Is(err, io.EOF) {
		return n, b.err
	}
	return n, err
}

var errPSReadFailed = errors.New("synthetic search read failure")

func TestSearchPageFixture(t *testing.T) {
	c, srv := newServerClient(t, respond(http.StatusOK, testutil.Fixture(t, "provider/person-search-page.json")))
	res, err := c.SearchPersons(t.Context(), validSearch())
	require.NoError(t, err)
	assert.Equal(t, 1, srv.Hits())

	require.Len(t, res.Persons, 2)
	p := res.Persons[0]
	assert.Equal(t, "G-1000000000000000101", p.TahoeID)
	assert.Equal(t, "Riley Q Sample", p.FullName)
	require.NotNil(t, p.Age)
	assert.Equal(t, 41, *p.Age)
	require.Len(t, p.Addresses, 1)
	assert.Equal(t, "100 Sample Way; Exampleton, OH 45000-0001", p.Addresses[0].FullAddress)
	require.Len(t, p.PhoneNumbers, 2)
	assert.Equal(t, "(312) 555-0199", p.PhoneNumbers[1].PhoneNumber)
	require.NotNil(t, p.PhoneNumbers[1].IsConnected)
	assert.False(t, *p.PhoneNumbers[1].IsConnected)
	require.Len(t, p.RelativesSummary, 1)
	assert.Equal(t, "Spouse", p.RelativesSummary[0].RelativeType)
	assert.JSONEq(t, `[]`, string(p.Extra["driversLicenseDetail"]))
	assert.Equal(t, "Riley Sample Jr", res.Persons[1].FullName)
	require.NotNil(t, res.Counts.SearchResults)
	assert.Equal(t, 2, *res.Counts.SearchResults)
	assert.Equal(t, enformion.PersonSearchPagination{}, res.Pagination, "observed as an empty object")
	assert.Equal(t, "synthetic-request-0101", res.RequestID)
	assert.Equal(t, enformion.ProviderDiagnostics{}, res.Diagnostics)
}

func TestSearchNoPersonsFixture(t *testing.T) {
	c, _ := newServerClient(t, respond(http.StatusOK, testutil.Fixture(t, "synthetic/person-search-no-persons.json")))
	res, err := c.SearchPersons(t.Context(), validSearch())
	require.NoError(t, err)
	assert.Nil(t, res.Persons, "no candidates; not proof that no match exists")
	assert.Equal(t, "synthetic-request-0102", res.RequestID)
	require.Len(t, res.Diagnostics.Warnings, 1)
}

func TestSearchIsErrorFixture(t *testing.T) {
	for _, withDiag := range []bool{false, true} {
		t.Run("diagnostics="+strconv.FormatBool(withDiag), func(t *testing.T) {
			var opts []enformion.Option
			if withDiag {
				opts = append(opts, enformion.WithProviderDiagnostics())
			}
			c, srv := newServerClient(t, respond(http.StatusOK, testutil.Fixture(t, "synthetic/person-search-is-error.json")), opts...)
			res, err := c.SearchPersons(t.Context(), validSearch())
			assert.Nil(t, res, "a refusal never returns its candidates")
			require.ErrorIs(t, err, enformion.ErrRejected)
			ce := callError(t, err)
			assert.Equal(t, http.StatusOK, ce.StatusCode)
			assert.Equal(t, "synthetic-request-0103", ce.RequestID)
			assert.Equal(t, withDiag, ce.Diagnostics().Message != "")
			assert.Equal(t, 1, srv.Hits())
		})
	}
}

func TestSearchStatusTable(t *testing.T) {
	tests := []struct {
		status int
		kind   enformion.FailureKind
	}{
		{http.StatusCreated, enformion.FailureUnexpectedResponse},
		{http.StatusNoContent, enformion.FailureUnexpectedResponse},
		{http.StatusBadRequest, enformion.FailureRejected},
		{http.StatusUnauthorized, enformion.FailureUnauthorized},
		{http.StatusForbidden, enformion.FailureUnauthorized},
		{http.StatusNotFound, enformion.FailureRejected},
		{http.StatusTooManyRequests, enformion.FailureRateLimited},
		{http.StatusInternalServerError, enformion.FailureUpstream},
		{http.StatusBadGateway, enformion.FailureUpstream},
		{600, enformion.FailureUnexpectedResponse},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.status), func(t *testing.T) {
			requestID := "ps-req-" + strconv.Itoa(tt.status)
			body := []byte(`{"requestId":"` + requestID + `","persons":[{"tahoeId":"G-1"}],"isError":true}`)
			c, srv := newServerClient(t, respond(tt.status, body))
			res, err := c.SearchPersons(t.Context(), validSearch())
			assert.Nil(t, res, "a refusal is never a page, nor an empty one")
			require.ErrorIs(t, err, kindSentinels()[tt.kind])
			ce := callError(t, err)
			assert.Equal(t, tt.kind, ce.Kind)
			assert.Equal(t, tt.status, ce.StatusCode)
			assert.Equal(t, 1, srv.Hits(), "exactly one attempt")
			if tt.status == http.StatusNoContent {
				assert.Empty(t, ce.RequestID, "no body, no request ID")
			} else {
				assert.Equal(t, requestID, ce.RequestID)
			}
			assert.Equal(t, "enformion: "+string(tt.kind)+" (HTTP "+strconv.Itoa(tt.status)+")", err.Error())
		})
	}
}

func TestSearchMalformedAndUnexpectedBodies(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		kind  enformion.FailureKind
		field string
	}{
		{"invalid JSON", `{"isError":false,"persons":[`, enformion.FailureMalformedResponse, ""},
		{"trailing data", emptySearchPage + ` {}`, enformion.FailureMalformedResponse, ""},
		{"invalid UTF-8", "{\"isError\":false,\"persons\":[{\"fullName\":\"\xff\"}]}", enformion.FailureMalformedResponse, ""},
		{"duplicate key", `{"isError":false,"persons":[],"persons":[]}`, enformion.FailureMalformedResponse, ""},
		{"wrong field type", `{"isError":false,"persons":[{"age":"41"}]}`, enformion.FailureMalformedResponse, "persons[0].age"},
		{"nested wrong field type", `{"isError":false,"persons":[{"addresses":[{"zip":45000}]}]}`, enformion.FailureMalformedResponse, "persons[0].addresses[0].zip"},
		{"array", `[]`, enformion.FailureUnexpectedResponse, ""},
		{"empty object", `{}`, enformion.FailureUnexpectedResponse, "isError"},
		{"persons missing", `{"isError":false}`, enformion.FailureUnexpectedResponse, "persons"},
		{"a renamed person member", `{"isError":false,"persons":[{"personName":"Riley"}]}`, enformion.FailureUnexpectedResponse, "persons[0]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, srv := newServerClient(t, respond(http.StatusOK, []byte(tt.body)))
			res, err := c.SearchPersons(t.Context(), validSearch())
			assert.Nil(t, res)
			require.ErrorIs(t, err, kindSentinels()[tt.kind])
			ce := callError(t, err)
			assert.Equal(t, tt.kind, ce.Kind)
			assert.Equal(t, tt.field, ce.Field)
			assert.Equal(t, http.StatusOK, ce.StatusCode)
			assert.Equal(t, 1, srv.Hits(), "exactly one attempt")
		})
	}
}

func TestSearchZeroCalls(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Minute))
	defer cancelExpired()
	var nilCtx context.Context

	noMode := validSearch()
	noMode.Mode = ""
	noCriteria := enformion.PersonSearchRequest{Mode: enformion.PersonSearchModePerson, Page: 2}
	badInclude := validSearch()
	badInclude.Includes = []enformion.PersonSearchInclude{"Phone Numbers"}

	tests := []struct {
		name     string
		ctx      context.Context //nolint:containedctx // each case runs with its own context
		req      enformion.PersonSearchRequest
		sentinel error
		field    string
		reason   string
		kind     enformion.FailureKind
	}{
		{name: "no mode", ctx: context.Background(), req: noMode, sentinel: enformion.ErrInvalidRequest, field: "Mode", reason: "required"},
		{name: "no criteria", ctx: context.Background(), req: noCriteria, sentinel: enformion.ErrInvalidRequest, field: "request", reason: "no_criteria"},
		{name: "invalid token", ctx: context.Background(), req: badInclude, sentinel: enformion.ErrInvalidRequest, field: "Includes[0]", reason: "invalid_format"},
		{name: "nil context", ctx: nilCtx, req: validSearch(), sentinel: enformion.ErrInvalidRequest, field: "ctx", reason: "nil"},
		{name: "nil context before an invalid request", ctx: nilCtx, req: noMode, sentinel: enformion.ErrInvalidRequest, field: "ctx", reason: "nil"},
		{name: "invalid request beats a canceled context", ctx: canceled, req: noCriteria, sentinel: enformion.ErrInvalidRequest, field: "request", reason: "no_criteria"},
		{name: "canceled context", ctx: canceled, req: validSearch(), sentinel: context.Canceled, kind: enformion.FailureCanceled},
		{name: "expired deadline", ctx: expired, req: validSearch(), sentinel: context.DeadlineExceeded, kind: enformion.FailureDeadlineExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct := &testutil.CountingTransport{Next: testutil.StaticTransport(http.StatusOK, emptySearchPage)}
			c := newTransportClient(t, ct)
			res, err := c.SearchPersons(tt.ctx, tt.req)
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

// TestSearchOneAttemptOneClose proves that every path makes one attempt and
// closes the response body exactly once, and that a read error never reaches
// the error text.
func TestSearchOneAttemptOneClose(t *testing.T) {
	page := string(testutil.Fixture(t, "provider/person-search-page.json"))
	tests := []struct {
		name     string
		status   int
		content  string
		readErr  error
		wantKind enformion.FailureKind
	}{
		{name: "page", status: http.StatusOK, content: page},
		{name: "no persons", status: http.StatusOK, content: emptySearchPage},
		{name: "malformed", status: http.StatusOK, content: `{"isError":false,"persons":[{"age":"x"}]}`, wantKind: enformion.FailureMalformedResponse},
		{name: "unexpected", status: http.StatusOK, content: `{"isError":false}`, wantKind: enformion.FailureUnexpectedResponse},
		{name: "rejected in the envelope", status: http.StatusOK, content: `{"isError":true,"persons":[]}`, wantKind: enformion.FailureRejected},
		{name: "read error", status: http.StatusOK, content: `{"isError":`, readErr: errPSReadFailed, wantKind: enformion.FailureTransport},
		{name: "rejected", status: http.StatusBadRequest, content: `{}`, wantKind: enformion.FailureRejected},
		{name: "rate limited", status: http.StatusTooManyRequests, content: `{}`, wantKind: enformion.FailureRateLimited},
		{name: "upstream with a read error", status: http.StatusBadGateway, content: `{`, readErr: errPSReadFailed, wantKind: enformion.FailureUpstream},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body *psFailingBody
			ct := &testutil.CountingTransport{Next: testutil.Transport(tt.status, -1, func() io.ReadCloser {
				body = &psFailingBody{Body: testutil.NewBody(tt.content), err: tt.readErr}
				return body
			})}
			c := newTransportClient(t, ct)
			res, err := c.SearchPersons(t.Context(), validSearch())
			if tt.wantKind == "" {
				require.NoError(t, err)
				require.NotNil(t, res)
			} else {
				assert.Nil(t, res)
				ce := callError(t, err)
				assert.Equal(t, tt.wantKind, ce.Kind)
				assert.Equal(t, tt.status, ce.StatusCode)
			}
			if tt.readErr != nil {
				assertErrorClean(t, err, tt.readErr.Error())
			}
			require.NotNil(t, body)
			assert.Equal(t, 1, body.Closes(), "the body is closed exactly once")
			assert.Equal(t, 1, ct.Attempts())
		})
	}
}

func TestSearchRequestUnchanged(t *testing.T) {
	c, _ := newServerClient(t, respond(http.StatusOK, testutil.Fixture(t, "provider/person-search-page.json")))
	build := func() enformion.PersonSearchRequest {
		return enformion.PersonSearchRequest{
			Mode:      enformion.PersonSearchModePerson,
			FirstName: feFirst, LastName: searchLast,
			AKAs:      []enformion.PersonSearchRequestName{{FirstName: "Rye", LastName: searchLast}},
			Addresses: []enformion.PersonSearchRequestAddress{{AddressLine2: "Exampleton, OH", County: searchLast}},
			Relatives: []enformion.PersonSearchRequestName{{FirstName: "Avery"}},
			TahoeIDs:  []string{"G-1000000000000000101"},
			Includes:  []enformion.PersonSearchInclude{enformion.PersonSearchIncludeAddresses},
			FilterOptions: []enformion.PersonSearchFilter{
				enformion.PersonSearchFilterIncludeLowQualityAddresses,
			},
			FirstNameCharOffset: new(1),
			MaxPhoneYears:       new(0),
			Page:                2,
			ResultsPerPage:      10,
			SessionID:           "batch-7f3a9c-0002",
		}
	}
	req := build()
	_, err := c.SearchPersons(t.Context(), req)
	require.NoError(t, err)
	assert.True(t, reflect.DeepEqual(build(), req), "the request, its slices and its pointers are unchanged")
}

func TestSearchResultsShareNoMemory(t *testing.T) {
	c, _ := newServerClient(t, respond(http.StatusOK, testutil.Fixture(t, "synthetic/person-search-every-field.json")))
	a, err := c.SearchPersons(t.Context(), validSearch())
	require.NoError(t, err)
	b, err := c.SearchPersons(t.Context(), validSearch())
	require.NoError(t, err)
	require.Equal(t, a, b)

	a.Persons[0].Addresses[0].PhoneNumbers[0] = "changed"
	a.Persons[0].Extra["relatives"][0] = 'X'
	a.Persons[0].Name.SourceSummary[0] = 'X'
	a.CriteriaType[0] = 'X'
	assert.Equal(t, "(312) 555-0142", b.Persons[0].Addresses[0].PhoneNumbers[0])
	assert.True(t, json.Valid(b.Persons[0].Extra["relatives"]))
	assert.True(t, json.Valid(b.Persons[0].Name.SourceSummary))
	assert.True(t, json.Valid(b.CriteriaType))
}
