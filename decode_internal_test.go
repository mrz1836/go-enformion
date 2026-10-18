package enformion

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion/internal/testutil"
)

// dcLast is the synthetic last name the decode tests repeat.
const dcLast = "Example"

// dcDecode decodes body as an HTTP 200 response. A []byte body is used as it
// is, never copied.
func dcDecode[B string | []byte](body B, withDiagnostics bool) (*EnrichContactResult, error) {
	return decodeEnrichContact(&exchange{status: http.StatusOK, body: []byte(body)}, withDiagnostics)
}

// dcPerson wraps a person member in a successful envelope.
func dcPerson(person string) string { return `{"person":` + person + `,"isError":false}` }

// dcMatch is the result the provider-shaped match fixture decodes to.
func dcMatch() *EnrichContactResult {
	return &EnrichContactResult{
		Person: &Person{
			PersonID:   "G-1000000000000000042",
			Name:       Name{FirstName: "Avery", MiddleName: "Q", LastName: dcLast},
			Age:        "41",
			PartialDOB: "6/1985",
			Addresses: []Address{
				{
					Street: "100 Sample Way", Unit: "", City: "Exampleton", State: "OH", Zip: "45000",
					FirstReportedDate: "3/2/2011", LastReportedDate: "8/14/2025",
				},
				{
					Street: "200 Placeholder Rd", Unit: "4B", City: "Testville", State: "OH", Zip: "45001",
					FirstReportedDate: "1/9/2004", LastReportedDate: "2/27/2011",
				},
			},
			Phones: []Phone{
				{
					Number: "(312) 555-0142", Type: "mobile", IsConnected: new(true),
					FirstReportedDate: "5/5/2015", LastReportedDate: "9/30/2025",
				},
				{
					Number: "(312) 555-0199", Type: "landline", IsConnected: new(false),
					FirstReportedDate: "7/1/2006", LastReportedDate: "12/31/2012",
				},
			},
			Emails: []Email{
				{Address: "avery.example@example.com", IsValidated: new(true), IsBusiness: new(false)},
				{Address: "a.example@example.org"},
			},
		},
		IdentityScore: new(87),
	}
}

func TestDecodeProviderMatch(t *testing.T) {
	body := testutil.Fixture(t, "provider/match.json")
	for _, diag := range []bool{false, true} {
		res, err := dcDecode(body, diag)
		require.NoError(t, err)
		assert.Equal(t, dcMatch(), res)
	}
}

func TestDecodeProviderRejected400(t *testing.T) {
	_, err := decodeEnrichContact(&exchange{status: http.StatusBadRequest, body: testutil.Fixture(t, "provider/rejected-400.json")}, true)
	ce := asCallError(t, err)
	assert.Equal(t, FailureRejected, ce.Kind)
	assert.Equal(t, http.StatusBadRequest, ce.StatusCode)
	assert.Empty(t, ce.RequestID)
	assert.Empty(t, ce.Field)
	assert.Equal(t, ProviderDiagnostics{}, ce.Diagnostics())
	require.ErrorIs(t, err, ErrRejected)
	require.ErrorIs(t, err, errStatus)
	assert.Equal(t, "enformion: rejected (HTTP 400)", err.Error())
}

func TestDecodeStatusKinds(t *testing.T) {
	tests := []struct {
		status int
		kind   FailureKind
	}{
		{100, FailureUnexpectedResponse},
		{101, FailureUnexpectedResponse},
		{199, FailureUnexpectedResponse},
		{201, FailureUnexpectedResponse},
		{204, FailureUnexpectedResponse},
		{299, FailureUnexpectedResponse},
		{300, FailureUnexpectedResponse},
		{304, FailureUnexpectedResponse},
		{399, FailureUnexpectedResponse},
		{400, FailureRejected},
		{401, FailureUnauthorized},
		{403, FailureUnauthorized},
		{404, FailureRejected},
		{418, FailureRejected},
		{429, FailureRateLimited},
		{499, FailureRejected},
		{500, FailureUpstream},
		{503, FailureUpstream},
		{599, FailureUpstream},
		{600, FailureUnexpectedResponse},
		{999, FailureUnexpectedResponse},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.status), func(t *testing.T) {
			assert.Equal(t, tt.kind, statusKind(tt.status))
			res, err := decodeEnrichContact(&exchange{status: tt.status, body: testutil.Fixture(t, "provider/match.json")}, true)
			assert.Nil(t, res, "a match body never rescues a non-200 status")
			ce := asCallError(t, err)
			assert.Equal(t, tt.kind, ce.Kind)
			assert.Equal(t, tt.status, ce.StatusCode)
			require.ErrorIs(t, err, errStatus)
		})
	}
}

func TestDecodeFailureEnvelope(t *testing.T) {
	const withDiag = `{"requestId":"rid-429","message":"slow down","error":{"inputErrors":[],"warnings":["w1",{"k":1}]}}`
	tests := []struct {
		name      string
		status    int
		body      []byte
		diag      bool
		kind      FailureKind
		requestID string
		want      ProviderDiagnostics
	}{
		{name: "request ID without diagnostics", status: 429, body: []byte(withDiag), kind: FailureRateLimited, requestID: "rid-429"},
		{
			name: "request ID and diagnostics", status: 429, body: []byte(withDiag), diag: true, kind: FailureRateLimited, requestID: "rid-429",
			want: ProviderDiagnostics{Message: "slow down", Warnings: []json.RawMessage{json.RawMessage(`"w1"`), json.RawMessage(`{"k":1}`)}},
		},
		{name: "invalid JSON", status: 400, body: []byte(`{"requestId":"r",`), diag: true, kind: FailureRejected},
		{name: "HTML", status: 502, body: []byte(`<html>bad gateway</html>`), diag: true, kind: FailureUpstream},
		{name: "not an object", status: 500, body: []byte(`["requestId"]`), diag: true, kind: FailureUpstream},
		{name: "exact duplicate key", status: 401, body: []byte(`{"requestId":"a","requestId":"b"}`), diag: true, kind: FailureUnauthorized},
		{name: "case-folded duplicate key", status: 403, body: []byte(`{"requestId":"a","RequestId":"b"}`), diag: true, kind: FailureUnauthorized},
		{name: "invalid UTF-8", status: 404, body: []byte("{\"requestId\":\"r\",\"message\":\"\xff\"}"), diag: true, kind: FailureRejected},
		{
			name: "wrong types are skipped", status: 404, body: []byte(`{"requestId":"r","message":5,"error":"x"}`), diag: true,
			kind: FailureRejected, requestID: "r",
		},
		{
			name: "wrong list type is skipped", status: 503, body: []byte(`{"requestId":"r","message":"m","error":{"inputErrors":{},"warnings":["w"]}}`),
			diag: true, kind: FailureUpstream, requestID: "r",
			want: ProviderDiagnostics{Message: "m", Warnings: []json.RawMessage{json.RawMessage(`"w"`)}},
		},
		{
			name: "duplicated list is skipped", status: 422, body: []byte(`{"message":"m","error":{"warnings":["a"],"WARNINGS":["b"]}}`),
			diag: true, kind: FailureRejected, want: ProviderDiagnostics{Message: "m"},
		},
		{name: "no body", status: 418, body: nil, diag: true, kind: FailureRejected},
		{name: "empty body", status: 418, body: []byte{}, diag: true, kind: FailureRejected},
		{name: "redirect", status: 302, body: []byte(`{"requestId":"r3"}`), diag: true, kind: FailureUnexpectedResponse, requestID: "r3"},
		{name: "invalid request ID", status: 429, body: []byte(`{"requestId":"has space"}`), kind: FailureRateLimited},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := decodeEnrichContact(&exchange{status: tt.status, body: tt.body}, tt.diag)
			assert.Nil(t, res)
			ce := asCallError(t, err)
			assert.Equal(t, tt.kind, ce.Kind)
			assert.Equal(t, tt.status, ce.StatusCode)
			assert.Equal(t, tt.requestID, ce.RequestID)
			assert.Empty(t, ce.Field)
			assert.Equal(t, tt.want, ce.Diagnostics())
		})
	}
}

func TestDecodeSyntheticFixtures(t *testing.T) {
	t.Run("no person", func(t *testing.T) {
		for _, diag := range []bool{false, true} {
			res, err := dcDecode(testutil.Fixture(t, "synthetic/no-person.json"), diag)
			require.NoError(t, err)
			assert.Equal(t, &EnrichContactResult{
				IdentityScore: new(0),
				RequestID:     "synthetic-request-0001",
				Diagnostics: ProviderDiagnostics{
					Warnings: []json.RawMessage{json.RawMessage(`"Synthetic warning: no person was returned."`)},
				},
			}, res, "a result always carries its diagnostics")
		}
	})
	t.Run("is error", func(t *testing.T) {
		body := testutil.Fixture(t, "synthetic/is-error.json")
		_, err := dcDecode(body, false)
		ce := asCallError(t, err)
		assert.Equal(t, FailureRejected, ce.Kind)
		assert.Equal(t, http.StatusOK, ce.StatusCode)
		assert.Equal(t, "synthetic-request-0002", ce.RequestID)
		assert.Empty(t, ce.Field)
		assert.Equal(t, ProviderDiagnostics{}, ce.Diagnostics(), "diagnostics are off by default")
		require.ErrorIs(t, err, ErrRejected)
		require.ErrorIs(t, err, errProviderError)

		_, err = dcDecode(body, true)
		ce = asCallError(t, err)
		assert.Equal(t, ProviderDiagnostics{
			Message: "Synthetic message: the request was refused.",
			InputErrors: []json.RawMessage{
				json.RawMessage(`{"field":"Phone","message":"Synthetic input error."}`),
				json.RawMessage(`"Synthetic input error as a string."`),
			},
			Warnings: []json.RawMessage{json.RawMessage(`7`)},
		}, ce.Diagnostics())
	})
	t.Run("defaults person", func(t *testing.T) {
		res, err := dcDecode(testutil.Fixture(t, "synthetic/defaults-person.json"), false)
		require.NoError(t, err)
		assert.Equal(t, &EnrichContactResult{}, res)
	})
	t.Run("unknown keys person", func(t *testing.T) {
		_, err := dcDecode(testutil.Fixture(t, "synthetic/unknown-keys-person.json"), false)
		ce := asCallError(t, err)
		assert.Equal(t, FailureUnexpectedResponse, ce.Kind)
		assert.Equal(t, personKey, ce.Field)
		require.ErrorIs(t, err, ErrUnexpectedResponse)
		require.ErrorIs(t, err, errUnrecognizedData)
	})
}

func TestDecodePersonPresence(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		person *Person
	}{
		{name: "absent", body: `{"isError":false}`},
		{name: "JSON null", body: dcPerson(jsonNull)},
		{name: "empty object", body: dcPerson(`{}`)},
		{
			name: "defaults only",
			body: dcPerson(`{"personId":null,"name":{"firstName":"","middleName":null,"lastName":""},"age":"","partialDob":null,` +
				`"addresses":[{"street":"","zip":null}],"phones":[{"number":"","isConnected":null}],"emails":[],"relatives":[]}`),
		},
		{name: "false flags are not data", body: dcPerson(`{"phones":[{"isConnected":false}],"emails":[{"isValidated":false,"isBusiness":false}]}`)},
		{name: "unknown keys without data", body: dcPerson(`{"nickname":"","score":0,"tags":[],"extra":{"a":null}}`)},
		{
			name:   "recognized data with unknown keys",
			body:   dcPerson(`{"age":"52","nickname":"Ace","relatives":[{"personId":"G-2"}],"future":{"x":true}}`),
			person: &Person{Age: "52"},
		},
		{
			name:   "name null",
			body:   dcPerson(`{"name":null,"age":"52"}`),
			person: &Person{Age: "52"},
		},
		{
			name:   "unknown name keys beside recognized ones",
			body:   dcPerson(`{"name":{"firstName":"Avery","suffix":"Jr"}}`),
			person: &Person{Name: Name{FirstName: "Avery"}},
		},
		{
			name:   "a flag alone",
			body:   dcPerson(`{"emails":[{"isBusiness":true}]}`),
			person: &Person{Emails: []Email{{IsBusiness: new(true)}}},
		},
		{
			name:   "unknown keys may repeat across case",
			body:   `{"foo":1,"FOO":2,"isError":false,"person":{"age":"3","bar":1,"Bar":2}}`,
			person: &Person{Age: "3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := dcDecode(tt.body, false)
			require.NoError(t, err)
			assert.Equal(t, tt.person, res.Person)
		})
	}
}

func TestDecodeFailures(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		kind      FailureKind
		field     string
		cause     error
		requestID string
	}{
		{name: "person array", body: dcPerson(`[]`), kind: FailureMalformedResponse, field: personKey, cause: errWrongType},
		{name: "person string", body: dcPerson(`"Avery"`), kind: FailureMalformedResponse, field: personKey, cause: errWrongType},
		{name: "person number", body: dcPerson(`1`), kind: FailureMalformedResponse, field: personKey, cause: errWrongType},
		{name: "person true", body: dcPerson(`true`), kind: FailureMalformedResponse, field: personKey, cause: errWrongType},
		{name: "person unknown keys only", body: dcPerson(`{"fullName":"Avery Example"}`), kind: FailureUnexpectedResponse, field: personKey, cause: errUnrecognizedData},
		{name: "person relatives only", body: dcPerson(`{"relatives":[{"personId":"G-3"}]}`), kind: FailureUnexpectedResponse, field: personKey, cause: errUnrecognizedData},
		{name: "person nested unknown data", body: dcPerson(`{"extra":{"deep":[0,{"x":1}]}}`), kind: FailureUnexpectedResponse, field: personKey, cause: errUnrecognizedData},
		{
			name: "name unknown keys only", body: dcPerson(`{"age":"30","name":{"first":"Avery","last":"Example"}}`),
			kind: FailureUnexpectedResponse, field: "person.name", cause: errUnrecognizedData,
		},
		{name: "name string", body: dcPerson(`{"age":"30","name":"Avery"}`), kind: FailureMalformedResponse, field: "person.name", cause: errWrongType},
		{name: "first name number", body: dcPerson(`{"name":{"firstName":5}}`), kind: FailureMalformedResponse, field: "person.name.firstName", cause: errWrongType},
		{name: "age number", body: dcPerson(`{"age":31337}`), kind: FailureMalformedResponse, field: "person.age", cause: errWrongType},
		{name: "person ID number", body: dcPerson(`{"personId":12345678901234567890}`), kind: FailureMalformedResponse, field: "person.personId", cause: errWrongType},
		{name: "partial DOB boolean", body: dcPerson(`{"partialDob":true}`), kind: FailureMalformedResponse, field: "person.partialDob", cause: errWrongType},
		{name: "addresses object", body: dcPerson(`{"addresses":{}}`), kind: FailureMalformedResponse, field: "person.addresses", cause: errWrongType},
		{name: "phones string", body: dcPerson(`{"phones":"(312) 555-0142"}`), kind: FailureMalformedResponse, field: "person.phones", cause: errWrongType},
		{
			name: "address entry string", body: dcPerson(`{"age":"30","addresses":[{},{},"100 Sample Way"]}`),
			kind: FailureMalformedResponse, field: "person.addresses[2]", cause: errWrongType,
		},
		{
			name: "address entry number", body: dcPerson(`{"age":"30","addresses":[{},{},45000]}`),
			kind: FailureMalformedResponse, field: "person.addresses[2]", cause: errWrongType,
		},
		{name: "address entry array", body: dcPerson(`{"addresses":[[]]}`), kind: FailureMalformedResponse, field: "person.addresses[0]", cause: errWrongType},
		{name: "zip number", body: dcPerson(`{"addresses":[{"zip":45000}]}`), kind: FailureMalformedResponse, field: "person.addresses[0].zip", cause: errWrongType},
		{
			name: "phone number number", body: dcPerson(`{"phones":[{"number":3125550142}]}`),
			kind: FailureMalformedResponse, field: "person.phones[0].number", cause: errWrongType,
		},
		{
			name: "isConnected string", body: dcPerson(`{"phones":[{"number":"(312) 555-0142","isConnected":"yes"}]}`),
			kind: FailureMalformedResponse, field: "person.phones[0].isConnected", cause: errWrongType,
		},
		{
			name: "isValidated number", body: dcPerson(`{"emails":[{"email":"a@example.com","isValidated":1}]}`),
			kind: FailureMalformedResponse, field: "person.emails[0].isValidated", cause: errWrongType,
		},
		{
			name: "phone entry unknown keys only", body: dcPerson(`{"age":"30","phones":[{"number":"(312) 555-0142"},{"phoneNumber":"(312) 555-0143"}]}`),
			kind: FailureUnexpectedResponse, field: "person.phones[1]", cause: errUnrecognizedData,
		},
		{
			name: "null elements count in the index", body: dcPerson(`{"age":"30","phones":[null,{"phoneNumber":"(312) 555-0143"}]}`),
			kind: FailureUnexpectedResponse, field: "person.phones[1]", cause: errUnrecognizedData,
		},
		{
			name: "email entry unknown keys only", body: dcPerson(`{"age":"30","emails":[null,null,{"address":"a@example.com"}]}`),
			kind: FailureUnexpectedResponse, field: "person.emails[2]", cause: errUnrecognizedData,
		},
		{name: "message number", body: `{"isError":false,"message":5}`, kind: FailureMalformedResponse, field: "message", cause: errWrongType},
		{name: "message object", body: `{"isError":false,"message":{}}`, kind: FailureMalformedResponse, field: "message", cause: errWrongType},
		{name: "error string", body: `{"isError":false,"error":"oops"}`, kind: FailureMalformedResponse, field: "error", cause: errWrongType},
		{name: "error array", body: `{"isError":false,"error":[]}`, kind: FailureMalformedResponse, field: "error", cause: errWrongType},
		{
			name: "input errors object", body: `{"isError":false,"error":{"inputErrors":{}}}`,
			kind: FailureMalformedResponse, field: "error.inputErrors", cause: errWrongType,
		},
		{
			name: "warnings string", body: `{"isError":false,"error":{"warnings":"w"}}`,
			kind: FailureMalformedResponse, field: "error.warnings", cause: errWrongType,
		},
		{
			name: "strict check failure keeps no request ID", body: `{"requestId":"rid-7","isError":false,"isError":false}`,
			kind: FailureMalformedResponse, cause: errDuplicateKey,
		},
		{name: "isError absent from an empty object", body: `{}`, kind: FailureUnexpectedResponse, field: isErrorKey, cause: errIsErrorFlag},
		{
			name: "isError missing", body: `{"person":null,"requestId":"rid-7"}`,
			kind: FailureUnexpectedResponse, field: isErrorKey, cause: errIsErrorFlag, requestID: "rid-7",
		},
		{
			name: "isError JSON null", body: `{"isError":null,"requestId":"rid-7"}`,
			kind: FailureUnexpectedResponse, field: isErrorKey, cause: errIsErrorFlag, requestID: "rid-7",
		},
		{
			name: "isError string false", body: `{"isError":"false","requestId":"rid-7"}`,
			kind: FailureUnexpectedResponse, field: isErrorKey, cause: errIsErrorFlag, requestID: "rid-7",
		},
		{
			name: "isError string true", body: `{"isError":"true","requestId":"rid-7"}`,
			kind: FailureUnexpectedResponse, field: isErrorKey, cause: errIsErrorFlag, requestID: "rid-7",
		},
		{
			name: "isError zero", body: `{"isError":0,"requestId":"rid-7"}`,
			kind: FailureUnexpectedResponse, field: isErrorKey, cause: errIsErrorFlag, requestID: "rid-7",
		},
		{
			name: "isError object", body: `{"isError":{},"requestId":"rid-7"}`,
			kind: FailureUnexpectedResponse, field: isErrorKey, cause: errIsErrorFlag, requestID: "rid-7",
		},
		{
			name: "isError true with a person", body: `{"isError":true,"requestId":"rid-7","person":{"age":"40"}}`,
			kind: FailureRejected, cause: errProviderError, requestID: "rid-7",
		},
		{
			name: "isError true with a malformed person", body: `{"isError":true,"requestId":"rid-7","person":5}`,
			kind: FailureRejected, cause: errProviderError, requestID: "rid-7",
		},
		{
			name: "isError true with diagnostics of the wrong type", body: `{"isError":true,"requestId":"rid-7","message":5,"error":"x"}`,
			kind: FailureRejected, cause: errProviderError, requestID: "rid-7",
		},
		{
			name: "folded duplicate person", body: `{"person":null,"Person":null,"isError":false,"requestId":"rid"}`,
			kind: FailureMalformedResponse, field: personKey, cause: errDuplicateKey, requestID: "rid",
		},
		{
			name: "folded duplicate isError keeps the request ID", body: `{"requestId":"r-0001","isError":false,"ISERROR":false}`,
			kind: FailureMalformedResponse, field: isErrorKey, cause: errDuplicateKey, requestID: "r-0001",
		},
		{
			name: "folded duplicate requestId reports no request ID", body: `{"requestId":"r-0001","RequestId":"r-0002","isError":false}`,
			kind: FailureMalformedResponse, field: "requestId", cause: errDuplicateKey,
		},
		{
			name: "folded duplicate person member", body: dcPerson(`{"age":"40","AGE":"41"}`),
			kind: FailureMalformedResponse, field: "person.age", cause: errDuplicateKey,
		},
		{
			name: "folded duplicate name member", body: dcPerson(`{"name":{"firstName":"A","FIRSTNAME":"B"}}`),
			kind: FailureMalformedResponse, field: "person.name.firstName", cause: errDuplicateKey,
		},
		{
			name: "folded duplicate phone member", body: dcPerson(`{"phones":[{"number":"1","Number":"2"}]}`),
			kind: FailureMalformedResponse, field: "person.phones[0].number", cause: errDuplicateKey,
		},
		{
			name: "folded duplicate email member", body: dcPerson(`{"emails":[{},{"isBusiness":true,"IsBusiness":false}]}`),
			kind: FailureMalformedResponse, field: "person.emails[1].isBusiness", cause: errDuplicateKey,
		},
		{
			name: "folded duplicate error member", body: `{"isError":false,"error":{"warnings":[],"Warnings":[]}}`,
			kind: FailureMalformedResponse, field: "error.warnings", cause: errDuplicateKey,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := dcDecode(tt.body, true)
			assert.Nil(t, res)
			ce := asCallError(t, err)
			assert.Equal(t, tt.kind, ce.Kind)
			assert.Equal(t, tt.field, ce.Field)
			assert.Equal(t, http.StatusOK, ce.StatusCode)
			assert.Equal(t, tt.requestID, ce.RequestID)
			require.ErrorIs(t, err, tt.cause)
			assert.Equal(t, ProviderDiagnostics{}, ce.Diagnostics())
		})
	}
}

func TestDecodeErrorsNeverQuoteValues(t *testing.T) {
	tests := []struct {
		body   string
		values []string
	}{
		{body: dcPerson(`{"age":31337}`), values: []string{"31337"}},
		{body: dcPerson(`{"phones":[{"number":3125550142}]}`), values: []string{"3125550142", "555", "0142"}},
		{body: dcPerson(`{"personId":98765432109876543210}`), values: []string{"98765", "43210"}},
		{body: `{"isError":false,"identityScore":7.25e3}`, values: []string{"7.25", "e3"}},
		{body: dcPerson(`{"fullName":"Zephyrine Quill"}`), values: []string{"Zephyrine", "Quill", "fullName"}},
		{body: `{"isError":"Zephyrine"}`, values: []string{"Zephyrine"}},
	}
	for _, tt := range tests {
		_, err := dcDecode(tt.body, true)
		require.Error(t, err)
		ce := asCallError(t, err)
		for _, v := range tt.values {
			assert.NotContains(t, err.Error(), v)
			assert.NotContains(t, ce.GoString(), v)
			for e := errors.Unwrap(err); e != nil; e = errors.Unwrap(e) {
				assert.NotContains(t, e.Error(), v)
			}
		}
	}
}

func TestDecodeCollections(t *testing.T) {
	t.Run("null and empty lists are nil", func(t *testing.T) {
		res, err := dcDecode(dcPerson(`{"age":"30","addresses":null,"phones":[],"emails":[null,null]}`), false)
		require.NoError(t, err)
		assert.Equal(t, &Person{Age: "30"}, res.Person)
	})
	t.Run("null elements are dropped", func(t *testing.T) {
		res, err := dcDecode(dcPerson(`{"addresses":[null,{"street":"1 Sample Way"},null,{"city":"Exampleton"}]}`), false)
		require.NoError(t, err)
		assert.Equal(t, &Person{Addresses: []Address{{Street: "1 Sample Way"}, {City: "Exampleton"}}}, res.Person)
	})
	t.Run("empty entries are kept", func(t *testing.T) {
		res, err := dcDecode(dcPerson(`{"age":"30","addresses":[{},{"city":"Exampleton"},{"street":""}],"phones":[{"isConnected":false}]}`), false)
		require.NoError(t, err)
		assert.Equal(t, &Person{
			Age:       "30",
			Addresses: []Address{{}, {City: "Exampleton"}, {}},
			Phones:    []Phone{{IsConnected: new(false)}},
		}, res.Person)
	})
	t.Run("an entry with recognized data keeps it and ignores unknown keys", func(t *testing.T) {
		res, err := dcDecode(dcPerson(`{"phones":[{"number":"(312) 555-0142","carrier":"Example Wireless","type":"satellite"}]}`), false)
		require.NoError(t, err)
		assert.Equal(t, &Person{Phones: []Phone{{Number: "(312) 555-0142", Type: "satellite"}}}, res.Person,
			"unknown enum values are kept verbatim")
	})
	t.Run("fifty entries are kept in order", func(t *testing.T) {
		var b strings.Builder
		b.WriteString(`{"phones":[`)
		want := make([]Phone, 0, 50)
		for i := range 50 {
			if i > 0 {
				b.WriteByte(',')
			}
			number := "(312) 555-01" + strconv.Itoa(10+i)
			b.WriteString(`{"number":"` + number + `"}`)
			want = append(want, Phone{Number: number})
		}
		b.WriteString(`]}`)
		res, err := dcDecode(dcPerson(b.String()), false)
		require.NoError(t, err)
		assert.Equal(t, &Person{Phones: want}, res.Person)
	})
	t.Run("a 20-digit person ID is kept exactly", func(t *testing.T) {
		res, err := dcDecode(dcPerson(`{"personId":"12345678901234567890"}`), false)
		require.NoError(t, err)
		assert.Equal(t, &Person{PersonID: "12345678901234567890"}, res.Person)
	})
	t.Run("strings keep escapes decoded", func(t *testing.T) {
		res, err := dcDecode(dcPerson(testutil.Esc(`{"name":{"firstName":"A~u0076ery","lastName":"O~"Example"}}`)), false)
		require.NoError(t, err)
		assert.Equal(t, &Person{Name: Name{FirstName: "Avery", LastName: `O"Example`}}, res.Person)
	})
}

func TestDecodeIdentityScore(t *testing.T) {
	tests := []struct {
		name  string
		score string
		want  *int
		fails bool
	}{
		{name: "absent"},
		{name: "JSON null", score: jsonNull},
		{name: "zero", score: `0`, want: new(0)},
		{name: "negative zero", score: `-0`, want: new(0)},
		{name: "ninety", score: `90`, want: new(90)},
		{name: "negative", score: `-5`, want: new(-5)},
		{name: "fraction", score: `1.5`, fails: true},
		{name: "integral fraction", score: `90.0`, fails: true},
		{name: "exponent", score: `1e2`, fails: true},
		{name: "string", score: `"90"`, fails: true},
		{name: "boolean", score: `true`, fails: true},
		{name: "object", score: `{}`, fails: true},
		{name: "huge", score: `99999999999999999999`, fails: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"isError":false}`
			if tt.score != "" {
				body = `{"isError":false,"identityScore":` + tt.score + `}`
			}
			res, err := dcDecode(body, false)
			if tt.fails {
				ce := asCallError(t, err)
				assert.Equal(t, FailureMalformedResponse, ce.Kind)
				assert.Equal(t, "identityScore", ce.Field)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, res.IdentityScore)
		})
	}
}

func TestDecodeCaseVariants(t *testing.T) {
	body := `{"PERSON":{"PersonId":"G-7","NAME":{"FirstName":"Avery","LASTNAME":"Example"},"Phones":[{"Number":"(312) 555-0142","ISCONNECTED":true}]},` +
		`"IdentityScore":5,"RequestID":"rid-case","IsError":false,"Message":"m","Error":{"Warnings":["w"]}}`
	res, err := dcDecode(body, false)
	require.NoError(t, err)
	assert.Equal(t, &EnrichContactResult{
		Person: &Person{
			PersonID: "G-7",
			Name:     Name{FirstName: "Avery", LastName: dcLast},
			Phones:   []Phone{{Number: "(312) 555-0142", IsConnected: new(true)}},
		},
		IdentityScore: new(5),
		RequestID:     "rid-case",
		Diagnostics:   ProviderDiagnostics{Message: "m", Warnings: []json.RawMessage{json.RawMessage(`"w"`)}},
	}, res)
}

func TestDecodeRequestID(t *testing.T) {
	long := strings.Repeat("r", 128)
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "plain", raw: `"rid-0001"`, want: "rid-0001"},
		{name: "punctuation", raw: `"a:b/c_d.e~f"`, want: "a:b/c_d.e~f"},
		{name: "escaped", raw: testutil.Esc(`"r~u0069d"`), want: "rid"},
		{name: "128 bytes", raw: `"` + long + `"`, want: long},
		{name: "129 bytes", raw: `"` + long + `r"`},
		{name: "empty", raw: `""`},
		{name: "space", raw: `" "`},
		{name: "inner space", raw: `"a b"`},
		{name: "tab escape", raw: testutil.Esc(`"a~tb"`)},
		{name: "non-ASCII", raw: `"rid-é"`},
		{name: "number", raw: `42`},
		{name: "JSON null", raw: jsonNull},
		{name: "object", raw: `{"id":"x"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := dcDecode(`{"isError":false,"requestId":`+tt.raw+`}`, false)
			require.NoError(t, err, "an unusable request ID never fails a call")
			assert.Equal(t, tt.want, res.RequestID)
			assert.Equal(t, tt.want, providerRequestID([]byte(tt.raw)))
		})
	}
	assert.Empty(t, providerRequestID(nil))
}

func TestDecodeDiagnosticsShapes(t *testing.T) {
	tests := []struct {
		name string
		body string
		want ProviderDiagnostics
	}{
		{name: "absent", body: `{"isError":false}`},
		{name: "nulls", body: `{"isError":false,"message":null,"error":null}`},
		{name: "empty error", body: `{"isError":false,"error":{}}`},
		{name: "empty lists", body: `{"isError":false,"message":"","error":{"inputErrors":[],"warnings":[]}}`},
		{name: "unknown error members", body: `{"isError":false,"error":{"code":5,"details":{"x":1}}}`},
		{
			name: "entries of every type",
			body: `{"isError":false,"error":{"inputErrors":[{"field":"Phone"},"text",1.5,true,false,[1,{"a":null}]],"warnings":[null]}}`,
			want: ProviderDiagnostics{InputErrors: []json.RawMessage{
				json.RawMessage(`{"field":"Phone"}`), json.RawMessage(`"text"`), json.RawMessage(`1.5`),
				json.RawMessage(`true`), json.RawMessage(`false`), json.RawMessage(`[1,{"a":null}]`),
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := dcDecode(tt.body, false)
			require.NoError(t, err)
			assert.Equal(t, tt.want, res.Diagnostics)
		})
	}
}

func TestDecodeDiagnosticsBounds(t *testing.T) {
	t.Run("message is cut on a UTF-8 boundary", func(t *testing.T) {
		msg := strings.Repeat("a", 1023) + "é" + "tail"
		res, err := dcDecode(`{"isError":false,"message":"`+msg+`"}`, false)
		require.NoError(t, err)
		assert.Equal(t, strings.Repeat("a", 1023), res.Diagnostics.Message)
	})
	t.Run("message of exactly 1 KiB is kept", func(t *testing.T) {
		msg := strings.Repeat("b", 1022) + "é"
		res, err := dcDecode(`{"isError":false,"message":"`+msg+`"}`, false)
		require.NoError(t, err)
		assert.Equal(t, msg, res.Diagnostics.Message)
	})
	t.Run("at most 32 entries", func(t *testing.T) {
		entries := make([]string, 40)
		for i := range entries {
			entries[i] = strconv.Itoa(i + 1)
		}
		res, err := dcDecode(`{"isError":false,"error":{"warnings":[`+strings.Join(entries, ",")+`]}}`, false)
		require.NoError(t, err)
		require.Len(t, res.Diagnostics.Warnings, 32)
		for i, w := range res.Diagnostics.Warnings {
			assert.Equal(t, strconv.Itoa(i+1), string(w))
		}
	})
	t.Run("long and null entries are dropped and not counted", func(t *testing.T) {
		big := `"` + strings.Repeat("x", 4100) + `"`
		parts := make([]string, 0, 38)
		parts = append(parts, big, jsonNull, `"kept-1"`)
		for i := range 35 {
			parts = append(parts, `"n`+strconv.Itoa(i)+`"`)
		}
		res, err := dcDecode(`{"isError":false,"error":{"inputErrors":[`+strings.Join(parts, ",")+`]}}`, false)
		require.NoError(t, err)
		require.Len(t, res.Diagnostics.InputErrors, 32)
		assert.Equal(t, `"kept-1"`, string(res.Diagnostics.InputErrors[0]))
		assert.Equal(t, `"n30"`, string(res.Diagnostics.InputErrors[31]))
	})
	t.Run("an entry of exactly 4 KiB is kept", func(t *testing.T) {
		entry := `"` + strings.Repeat("y", 4094) + `"`
		require.Len(t, entry, 4096)
		res, err := dcDecode(`{"isError":false,"error":{"warnings":[`+entry+`]}}`, false)
		require.NoError(t, err)
		assert.Equal(t, []json.RawMessage{json.RawMessage(entry)}, res.Diagnostics.Warnings)
	})
	t.Run("entries are normalized", func(t *testing.T) {
		body := "{\"isError\":false,\"error\":{\"warnings\":[ { \"a\" : \"<b>&\" ,\n \"c\" : [ 1 , 2 ] } , \"line\u2028sep\" ]}}"
		res, err := dcDecode(body, false)
		require.NoError(t, err)
		require.Len(t, res.Diagnostics.Warnings, 2)
		w0 := string(res.Diagnostics.Warnings[0])
		assert.JSONEq(t, `{"a":"<b>&","c":[1,2]}`, w0)
		assert.Contains(t, w0, `"a":"\u003cb\u003e\u0026"`, "HTML-sensitive characters are escaped")
		assert.NotContains(t, w0, " ", "whitespace is compacted")
		assert.Len(t, w0, len(`{"a":"\u003cb\u003e\u0026","c":[1,2]}`))
		assert.Equal(t, `"line\u2028sep"`, string(res.Diagnostics.Warnings[1]))
	})
}

func TestDecodeIgnoredMembersNeverFail(t *testing.T) {
	body := `{"isError":false,"person":{"age":"30","relatives":"not a list"},` +
		`"pagination":"garbage","searchCriteria":{"echo":"Avery"},"databaseQueryInfo":7,` +
		`"requestType":[],"totalRequestExecutionTimeMs":"slow","requestTime":null,"newMember":{"x":[1]}}`
	res, err := dcDecode(body, false)
	require.NoError(t, err)
	assert.Equal(t, &EnrichContactResult{Person: &Person{Age: "30"}}, res)
}

func TestDecodeNeverAliasesBody(t *testing.T) {
	src := `{"person":{"personId":"G-9","name":{"firstName":"Avery","lastName":"Example"},"age":"41",` +
		`"addresses":[{"street":"100 Sample Way","zip":"45000"}],"phones":[{"number":"(312) 555-0142","isConnected":true}],` +
		`"emails":[{"email":"avery.example@example.com"}]},"identityScore":3,"requestId":"rid-alias",` +
		`"isError":false,"message":"note","error":{"inputErrors":[{"f":"x"}],"warnings":["w"]}}`
	body := []byte(src)
	res, err := dcDecode(body, false)
	require.NoError(t, err)
	want, err := dcDecode(src, false)
	require.NoError(t, err)
	for i := range body {
		body[i] = 'Z'
	}
	assert.Equal(t, want, res)

	errBody := []byte(`{"isError":true,"requestId":"rid-alias","message":"note","error":{"warnings":[{"w":1}]}}`)
	_, err = dcDecode(errBody, true)
	ce := asCallError(t, err)
	for i := range errBody {
		errBody[i] = 'Z'
	}
	assert.Equal(t, "rid-alias", ce.RequestID)
	assert.Equal(t, ProviderDiagnostics{Message: "note", Warnings: []json.RawMessage{json.RawMessage(`{"w":1}`)}}, ce.Diagnostics())
}

func TestDecodeJSONRoundTrip(t *testing.T) {
	bodies := map[string]string{
		"match":      string(testutil.Fixture(t, "provider/match.json")),
		"no person":  string(testutil.Fixture(t, "synthetic/no-person.json")),
		"defaults":   string(testutil.Fixture(t, "synthetic/defaults-person.json")),
		"false flag": dcPerson(`{"phones":[{"number":"1","isConnected":false},{"number":"2"}],"emails":[{"email":"e","isBusiness":false}]}`),
		"zero score": `{"isError":false,"identityScore":0}`,
		"html and separators": "{\"isError\":false,\"message\":\"<a>&\u2028\",\"person\":{\"name\":{\"firstName\":\"<b>&\u2029\"}}," +
			"\"error\":{\"warnings\":[\"<i>&\u2028\",{\"k\":\"<>\"}],\"inputErrors\":[1e2,-0.0]}}",
		"escapes": testutil.Esc(`{"isError":false,"message":"~ud800~u0000","error":{"warnings":["~ud83d~ude00","~/"]}}`),
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			res, err := dcDecode(body, false)
			require.NoError(t, err)
			data, err := json.Marshal(res)
			require.NoError(t, err)
			var back EnrichContactResult
			require.NoError(t, json.Unmarshal(data, &back))
			assert.Equal(t, *res, back)
		})
	}
	t.Run("pointer semantics survive", func(t *testing.T) {
		res, err := dcDecode(bodies["false flag"], false)
		require.NoError(t, err)
		data, err := json.Marshal(res)
		require.NoError(t, err)
		var back EnrichContactResult
		require.NoError(t, json.Unmarshal(data, &back))
		assert.Equal(t, &Person{
			Phones: []Phone{{Number: "1", IsConnected: new(false)}, {Number: "2"}},
			Emails: []Email{{Address: "e", IsBusiness: new(false)}},
		}, back.Person, "a false flag stays false and a missing one nil")
		assert.Nil(t, back.IdentityScore)

		zero, err := dcDecode(bodies["zero score"], false)
		require.NoError(t, err)
		data, err = json.Marshal(zero)
		require.NoError(t, err)
		assert.JSONEq(t, `{"identityScore":0}`, string(data))
	})
}

func TestDecodeTruncateUTF8(t *testing.T) {
	assert.Equal(t, "abc", truncateUTF8("abc", 3))
	assert.Equal(t, "ab", truncateUTF8("abc", 2))
	assert.Equal(t, "a", truncateUTF8("aé", 2))
	assert.Equal(t, "aé", truncateUTF8("aé", 3))
	assert.Empty(t, truncateUTF8("é", 1))
	assert.Empty(t, truncateUTF8("\U0001F600", 3))
	assert.Empty(t, truncateUTF8("abc", 0))
}

// dcNames returns the names k0 to k(n-1), which stManyKeys objects use.
func dcNames(n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = "k" + strconv.Itoa(i)
	}
	return names
}

// TestDecodeReadObjectFieldCounts proves that objects of any member count are
// read, inside and past the inline storage.
func TestDecodeReadObjectFieldCounts(t *testing.T) {
	for _, n := range []int{0, 1, 7, 8, 9, 16, 40} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			o, dup := readObject([]byte(stManyKeys("k", n, "")), dcNames(n), true)
			assert.Equal(t, -1, dup)
			for i := range n {
				assert.Equal(t, strconv.Itoa(i), string(o.field(i)), "member %d", i)
			}
			assert.Nil(t, o.field(-1))
			assert.Nil(t, o.field(n), "an index past the names is absent")
			assert.Nil(t, o.field(n+inlineFields))
			assert.False(t, o.ignoredData)
			assert.Len(t, o.more, max(n-inlineFields, 0), "more holds only the members past the inline storage")
		})
	}
	t.Run("absent members are nil", func(t *testing.T) {
		o, dup := readObject([]byte(`{"k3":1,"k39":2}`), dcNames(40), true)
		assert.Equal(t, -1, dup)
		for i := range 40 {
			switch i {
			case 3:
				assert.Equal(t, "1", string(o.field(i)))
			case 39:
				assert.Equal(t, "2", string(o.field(i)))
			default:
				assert.Nil(t, o.field(i), "member %d", i)
			}
		}
	})
	t.Run("setField ignores an index out of range", func(t *testing.T) {
		var o object
		o.reserve(9)
		for _, i := range []int{-1, 9, 100} {
			o.setField(i, []byte("x"))
			assert.Nil(t, o.field(i))
		}
		o.setField(8, []byte("y"))
		assert.Equal(t, "y", string(o.field(8)))
	})
}

// TestDecodeReadObjectDuplicates proves that a case-folded duplicate is found
// at every index, and that the first one repeated in body order wins.
func TestDecodeReadObjectDuplicates(t *testing.T) {
	for _, i := range []int{0, 7, 8, 39} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, dup := readObject([]byte(stManyKeys("k", 40, `,"K`+strconv.Itoa(i)+`":0`)), dcNames(40), false)
			assert.Equal(t, i, dup)
		})
	}
	t.Run("first repeated in body order", func(t *testing.T) {
		_, dup := readObject([]byte(stManyKeys("k", 40, `,"K39":1,"K8":2,"K7":3`)), dcNames(40), false)
		assert.Equal(t, 39, dup)
	})
	t.Run("escaped duplicate past the inline storage", func(t *testing.T) {
		_, dup := readObject([]byte(stManyKeys("k", 40, testutil.Esc(`,"~u004b20":0`))), dcNames(40), false)
		assert.Equal(t, 20, dup)
	})
}

// TestDecodeReadObjectIgnoredData proves that data under an unrecognized key
// is seen past the inline storage, and only when tracked.
func TestDecodeReadObjectIgnoredData(t *testing.T) {
	tests := []struct {
		name  string
		extra string
		track bool
		want  bool
	}{
		{name: "data, tracked", extra: `,"other":1`, track: true, want: true},
		{name: "nested data, tracked", extra: `,"other":{"x":[0,"y"]}`, track: true, want: true},
		{name: "data, untracked", extra: `,"other":1`},
		{name: "zeros only, tracked", extra: `,"other":{"x":[0,"",false,null]}`, track: true},
		{name: "a 41st name is not recognized", extra: `,"k40":"x"`, track: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o, dup := readObject([]byte(stManyKeys("k", 40, tt.extra)), dcNames(40), tt.track)
			assert.Equal(t, -1, dup)
			assert.Equal(t, tt.want, o.ignoredData)
			assert.Equal(t, "39", string(o.field(39)))
		})
	}
}

// TestDecodeReadObjectNeverPanics reads every strict-check seed, valid or not,
// against more names than the inline storage holds.
func TestDecodeReadObjectNeverPanics(t *testing.T) {
	names := dcNames(40)
	slots := make([]slot, len(names))
	for i, n := range names {
		slots[i] = slot{name: n}
	}
	for _, s := range fdStrictSeeds() {
		assert.NotPanics(t, func() {
			for _, track := range []bool{false, true} {
				o, _ := readObject([]byte(s), names, track)
				for i := -1; i <= len(names); i++ {
					_ = o.field(i)
				}
				_, _ = readSlots([]byte(s), slots, track)
			}
		}, "seed %q", s)
	}
}

// TestDecodeReadObjectAllocations proves that objects within the inline
// storage, every Contact Enrichment object among them, cost no allocation.
func TestDecodeReadObjectAllocations(t *testing.T) {
	testutil.SkipUnderRace(t)
	for _, tt := range []struct {
		n    int
		want float64
	}{{7, 0}, {8, 0}, {9, 1}, {40, 1}} {
		t.Run(strconv.Itoa(tt.n), func(t *testing.T) {
			raw, names := []byte(stManyKeys("k", tt.n, "")), dcNames(tt.n)
			allocs := testing.AllocsPerRun(100, func() { _, _ = readObject(raw, names, true) })
			assert.InDelta(t, tt.want, allocs, 0)
		})
	}
	t.Run("decodeFlat of a Contact address", func(t *testing.T) {
		raw := []byte(`{"street":"100 Sample Way","unit":"","city":"Exampleton","state":"OH","zip":"45000",` +
			`"firstReportedDate":"3/2/2011","lastReportedDate":"8/14/2025"}`)
		allocs := testing.AllocsPerRun(100, func() { _, _, _ = decodeAddress(raw, at("person.addresses", 0)) })
		assert.InDelta(t, 6, allocs, 0, "one per non-empty string kept, and none for the slots or the address")
	})
}

// TestDecodeFlatManySlots decodes an object of 26 string slots, past the inline
// storage, and fails at the right member.
func TestDecodeFlatManySlots(t *testing.T) {
	const n = 26
	decode := func(raw string) ([]string, bool, *decodeError) {
		got := make([]string, n)
		slots := make([]slot, n)
		for i := range slots {
			slots[i] = slot{name: "s" + strconv.Itoa(i), str: &got[i]}
		}
		data, derr := decodeFlat([]byte(raw), at("x", -1), slots)
		return got, data, derr
	}
	var b strings.Builder
	b.WriteByte('{')
	for i := range n {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`"s` + strconv.Itoa(i) + `":"v` + strconv.Itoa(i) + `"`)
	}
	b.WriteByte('}')

	got, data, derr := decode(b.String())
	require.Nil(t, derr)
	assert.True(t, data)
	for i, v := range got {
		assert.Equal(t, "v"+strconv.Itoa(i), v)
	}

	_, _, derr = decode(`{"s3":"a","s25":5}`)
	require.NotNil(t, derr)
	assert.Equal(t, "x.s25", derr.field)
	assert.Equal(t, errWrongType, derr.cause)

	_, _, derr = decode(`{"s20":"a","S20":"b"}`)
	require.NotNil(t, derr)
	assert.Equal(t, "x.s20", derr.field)
	assert.Equal(t, errDuplicateKey, derr.cause)

	_, _, derr = decode(`{"s26":"unrecognized"}`)
	require.NotNil(t, derr)
	assert.Equal(t, "x", derr.field)
	assert.Equal(t, errUnrecognizedData, derr.cause)
}

// TestDecodeSlotKinds covers each slot destination: what it keeps, what counts
// as data and what fails.
func TestDecodeSlotKinds(t *testing.T) {
	t.Run("num", func(t *testing.T) {
		tests := []struct {
			raw   string
			want  *int
			data  bool
			fails bool
		}{
			{raw: ``},
			{raw: jsonNull},
			{raw: `0`, want: new(0)},
			{raw: `-0`, want: new(0)},
			{raw: `7`, want: new(7), data: true},
			{raw: `-3`, want: new(-3), data: true},
			{raw: `1.5`, fails: true},
			{raw: `1e2`, fails: true},
			{raw: `"7"`, fails: true},
			{raw: `true`, fails: true},
			{raw: `99999999999999999999`, fails: true},
		}
		for _, tt := range tests {
			var got *int
			s := slot{name: "count", num: &got}
			data, derr := s.decode([]byte(tt.raw), at("x", 2), nil)
			if tt.fails {
				require.NotNil(t, derr, tt.raw)
				assert.Equal(t, "x[2].count", derr.field)
				assert.Equal(t, FailureMalformedResponse, derr.kind)
				continue
			}
			require.Nil(t, derr, tt.raw)
			assert.Equal(t, tt.want, got, tt.raw)
			assert.Equal(t, tt.data, data, tt.raw)
		}
	})
	t.Run("raw", func(t *testing.T) {
		tests := []struct {
			raw, want string
			data      bool
		}{
			{raw: ``},
			{raw: jsonNull},
			{raw: `0`, want: `0`},
			{raw: `""`, want: `""`},
			{raw: `[]`, want: `[]`},
			{raw: `{"a":[0,false,null]}`, want: `{"a":[0,false,null]}`},
			{raw: `{ "a" : [ 1 , "<b>&" ] }`, want: testutil.Esc(`{"a":[1,"~u003cb~u003e~u0026"]}`), data: true},
			{raw: testutil.Esc(`"~ud800"`), want: testutil.Esc(`"~ufffd"`), data: true},
			{raw: "\"\u2028\"", want: testutil.Esc(`"~u2028"`), data: true},
		}
		for _, tt := range tests {
			var got json.RawMessage
			s := slot{name: "summary", raw: &got}
			body := []byte(tt.raw)
			data, derr := s.decode(body, at("x", -1), nil)
			require.Nil(t, derr, tt.raw)
			assert.Equal(t, tt.data, data, tt.raw)
			if tt.want == "" {
				assert.Nil(t, got, tt.raw)
				continue
			}
			assert.Equal(t, tt.want, string(got), tt.raw)
			for i := range body {
				body[i] = 'Z'
			}
			assert.Equal(t, tt.want, string(got), "never aliases the body: %s", tt.raw)
		}
	})
	t.Run("sub paths are relative to the member", func(t *testing.T) {
		inner := func(raw []byte, p path) (bool, *decodeError) {
			var zip string
			return decodeObject(raw, p, nil, true, []slot{{name: "zip", str: &zip}})
		}
		list := func(raw []byte, p path) (bool, *decodeError) {
			_, data, derr := decodeArray(raw, p, nil, func(el []byte, ep path) (struct{}, bool, *decodeError) {
				var zip string
				data, derr := decodeObject(el, ep, nil, true, []slot{
					{name: "zip", str: &zip},
					{name: "marker", sub: inner},
				})
				return struct{}{}, data, derr
			})
			return data, derr
		}
		tests := []struct {
			body, field string
		}{
			{body: `{"addresses":5}`, field: "persons[3].addresses"},
			{body: `{"addresses":[null,{"zip":5}]}`, field: "persons[3].addresses[1].zip"},
			{body: `{"addresses":[{"marker":[]}]}`, field: "persons[3].addresses[0].marker"},
			{body: `{"addresses":[{"marker":{"zip":true}}]}`, field: "persons[3].addresses[0].marker.zip"},
			{body: `{"addresses":[{"marker":{"other":"x"}}]}`, field: "persons[3].addresses[0].marker"},
			{body: `{"addresses":[{},{},{"marker":{"zip":"1","ZIP":"2"}}]}`, field: "persons[3].addresses[2].marker.zip"},
		}
		for _, tt := range tests {
			_, derr := decodeSlots([]byte(tt.body), at("persons", 3), nil, true, []slot{{name: "addresses", sub: list}})
			require.NotNil(t, derr, tt.body)
			assert.Equal(t, tt.field, derr.field, tt.body)
		}
		data, derr := decodeSlots([]byte(`{"addresses":[{"marker":{"zip":"45000"}}]}`), at("persons", 3), nil, true,
			[]slot{{name: "addresses", sub: list}})
		require.Nil(t, derr)
		assert.True(t, data, "data in a nested object is the member's data")
	})
	t.Run("untracked objects ignore other members", func(t *testing.T) {
		var total *int
		slots := []slot{{name: "total", num: &total}}
		data, derr := decodeObject([]byte(`{"total":0,"added":7}`), at("counts", -1), nil, false, slots)
		require.Nil(t, derr)
		assert.False(t, data)
		assert.Equal(t, new(0), total)

		_, derr = decodeObject([]byte(`{"total":0,"added":7}`), at("counts", -1), nil, true, slots)
		require.NotNil(t, derr)
		assert.Equal(t, errUnrecognizedData, derr.cause)

		_, derr = decodeObject([]byte(`[]`), at("counts", -1), nil, false, slots)
		require.NotNil(t, derr)
		assert.Equal(t, "counts", derr.field)
		assert.Equal(t, errWrongType, derr.cause)

		data, derr = decodeObject([]byte(jsonNull), at("counts", -1), nil, false, slots)
		require.Nil(t, derr)
		assert.False(t, data)
	})
}

// TestDecodeArrays covers lists of strings and of raw values, and their check
// order: null elements, then the entry bound, then the budget, then the type.
func TestDecodeArrays(t *testing.T) {
	t.Run("strings", func(t *testing.T) {
		got, data, derr := decodeArray([]byte(`["a",null,"",`+testutil.Esc(`"~u0062"`)+`]`), at("rawNames", -1), nil, stringEntry)
		require.Nil(t, derr)
		assert.True(t, data)
		assert.Equal(t, []string{"a", "", "b"}, got, "an empty string is an entry; null is not")

		got, data, derr = decodeArray([]byte(`["",""]`), at("rawNames", -1), nil, stringEntry)
		require.Nil(t, derr)
		assert.False(t, data)
		assert.Equal(t, []string{"", ""}, got)

		_, _, derr = decodeArray([]byte(`["a",null,5]`), at("rawNames", -1), nil, stringEntry)
		require.NotNil(t, derr)
		assert.Equal(t, "rawNames[2]", derr.field)
		assert.Equal(t, errWrongType, derr.cause)

		for _, raw := range []string{``, jsonNull, `[]`, `[null]`} {
			got, data, derr = decodeArray([]byte(raw), at("rawNames", -1), nil, stringEntry)
			require.Nil(t, derr)
			assert.False(t, data)
			assert.Nil(t, got, "an empty list is nil: %q", raw)
		}
		_, _, derr = decodeArray([]byte(`"a"`), at("rawNames", -1), nil, stringEntry)
		require.NotNil(t, derr)
		assert.Equal(t, "rawNames", derr.field)
	})
	t.Run("raw values", func(t *testing.T) {
		b := newBudget(0)
		got, data, derr := decodeArray([]byte(`[0, null, {"a" : "<"}, []]`), at("neighbors", -1), &b, b.rawEntry)
		require.Nil(t, derr)
		assert.True(t, data)
		assert.Equal(t, []json.RawMessage{
			json.RawMessage(`0`), json.RawMessage(testutil.Esc(`{"a":"~u003c"}`)), json.RawMessage(`[]`),
		}, got)
	})
	t.Run("entry bound before the budget", func(t *testing.T) {
		raw := []byte(`[` + strings.TrimSuffix(strings.Repeat(`"",`, maxListEntries+1), ",") + `]`)
		b := budget(1 << 30)
		_, _, derr := decodeArray(raw, at("rawNames", -1), &b, stringEntry)
		require.NotNil(t, derr)
		assert.Equal(t, errTooManyEntries, derr.cause)
		assert.Equal(t, "rawNames", derr.field)
	})
}

// TestDecodeBudget proves the budget at its edge: entries are charged their
// size and raw values their normalized length, and a nil budget is unlimited.
func TestDecodeBudget(t *testing.T) {
	assert.Equal(t, budget(budgetPerBodyByte*1000+budgetSlack), newBudget(1000))

	var unlimited *budget
	assert.Nil(t, unlimited.charge(1<<40, at("x", -1)))

	entry := int(reflect.TypeFor[string]().Size())
	b := budget(3 * entry)
	got, _, derr := decodeArray([]byte(`["a","b","c"]`), at("rawNames", -1), &b, stringEntry)
	require.Nil(t, derr)
	assert.Len(t, got, 3)
	assert.Zero(t, b, "exactly spent")

	b = budget(3*entry - 1)
	_, _, derr = decodeArray([]byte(`["a",null,"b","c"]`), at("rawNames", -1), &b, stringEntry)
	require.NotNil(t, derr)
	assert.Equal(t, FailureResponseTooLarge, derr.kind)
	assert.Equal(t, errDecodeBudget, derr.cause)
	assert.Equal(t, "rawNames[3]", derr.field, "the entry that ran over")

	norm := len(testutil.Esc(`"~u003c~u003c"`))
	b = budget(norm)
	var v json.RawMessage
	_, derr = (&slot{name: "summary", raw: &v}).decode([]byte(`"<<"`), at("x", 0), &b)
	require.Nil(t, derr)
	assert.Zero(t, b, "a raw value costs its normalized length")

	b = budget(norm - 1)
	_, derr = (&slot{name: "summary", raw: &v}).decode([]byte(`"<<"`), at("x", 0), &b)
	require.NotNil(t, derr)
	assert.Equal(t, errDecodeBudget, derr.cause)
	assert.Equal(t, "x[0].summary", derr.field)

	b = budget(norm - 1)
	_, derr = decodeObject([]byte(`{"a":{"summary":"<<"}}`), at("x", -1), &b, true, []slot{
		{name: "a", sub: func(raw []byte, p path) (bool, *decodeError) {
			return decodeObject(raw, p, &b, true, []slot{{name: "summary", raw: &v}})
		}},
	})
	require.NotNil(t, derr)
	assert.Equal(t, errDecodeBudget, derr.cause)
	assert.Equal(t, "x.a.summary", derr.field, "a nested object charges the same budget")
}

// TestDecodeNormalizeJSON proves the normalization diagnostics and raw values
// share, and that it never aliases its input.
func TestDecodeNormalizeJSON(t *testing.T) {
	in := []byte(testutil.Esc(` { "k" : [ "<a>&", "~ud800", 1.50 ] } `))
	got, err := normalizeJSON(in)
	require.NoError(t, err)
	assert.Equal(t, testutil.Esc(`{"k":["~u003ca~u003e~u0026","~ufffd",1.50]}`), string(got))
	for i := range in {
		in[i] = 'Z'
	}
	assert.Equal(t, testutil.Esc(`{"k":["~u003ca~u003e~u0026","~ufffd",1.50]}`), string(got))

	same := []byte(`[1]`)
	got, err = normalizeJSON(same)
	require.NoError(t, err)
	assert.NotSame(t, &same[0], &got[0], "a value already normalized is still copied")
}
