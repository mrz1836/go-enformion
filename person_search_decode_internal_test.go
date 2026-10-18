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

// sdDecode decodes body as an HTTP 200 Person Search response. A []byte body
// is used as it is, never copied.
func sdDecode[B string | []byte](body B, withDiagnostics bool) (*PersonSearchResult, error) {
	return decodeSearchPersons(&exchange{status: http.StatusOK, body: []byte(body)}, withDiagnostics)
}

// sdPersons wraps the members of a persons list in a successful envelope.
func sdPersons(persons string) string { return `{"persons":[` + persons + `],"isError":false}` }

// sdZeroIndicators returns indicators that report 0 for every documented and
// observed kind.
func sdZeroIndicators() PersonSearchIndicators {
	return PersonSearchIndicators{
		HasBankruptcyRecords: new(0), HasBusinessRecords: new(0), HasDivorceRecords: new(0), HasDomainsRecords: new(0),
		HasEvictionsRecords: new(0), HasFEINRecords: new(0), HasForeclosuresRecords: new(0), HasJudgmentRecords: new(0),
		HasLienRecords: new(0), HasMarriageRecords: new(0), HasProfessionalLicenseRecords: new(0),
		HasPropertyRecords: new(0), HasVehicleRegistrationsRecords: new(0), HasWorkplaceRecords: new(0),
		HasDEARecords: new(0), HasPropertyV2Records: new(0), HasForeclosuresV2Records: new(0), HasUCCRecords: new(0),
		HasDebtRecords: new(0),
	}
}

// sdPage is the result the provider-shaped page fixture decodes to.
func sdPage() *PersonSearchResult {
	first := sdZeroIndicators()
	first.HasBusinessRecords, first.HasVehicleRegistrationsRecords, first.HasMarriageRecords = new(1), new(2), new(1)
	noData := map[string]json.RawMessage{"driversLicenseDetail": json.RawMessage(`[]`)}
	return &PersonSearchResult{
		Persons: []PersonSearchPerson{
			{
				TahoeID: "G-1000000000000000101",
				Name: PersonSearchName{
					FirstName: "Riley", MiddleName: "Q", LastName: sqLast, RawNames: []string{"RILEY Q SAMPLE"},
					PublicFirstSeenDate: "7/26/2016", TotalFirstSeenDate: "7/26/2016",
				},
				FullName: "Riley Q Sample",
				IsPublic: new(true), IsPremium: new(false), IsOptedOut: new(false), IsEnterpriseOptedOut: new(false),
				HasAdditionalData: new(true), SparseFlag: new(0),
				DOB: "6/1985", Age: new(41), IsCurrentPropertyOwner: "Not Available",
				DatesOfBirth: []PersonSearchDateOfBirth{{Age: new(41)}},
				DeathRecords: PersonSearchDeathRecord{IsDeceased: new(false)},
				AKAs:         []PersonSearchName{{FirstName: "Rye", LastName: sqLast, TotalFirstSeenDate: "3/2/2011"}},
				MergedNames: []PersonSearchName{{
					FirstName: "Riley", MiddleName: "Q", LastName: sqLast,
					PublicFirstSeenDate: "7/26/2016", TotalFirstSeenDate: "7/26/2016",
				}},
				Locations: []PersonSearchLocation{{City: "Exampleton", State: "OH"}},
				Addresses: []PersonSearchAddress{{
					FullAddress: "100 Sample Way; Exampleton, OH 45000-0001", HouseNumber: "100", StreetName: sqLast,
					StreetType: "Way", City: "Exampleton", State: "OH", County: sqLast, Zip: "45000", Zip4: "0001", DPC: "01",
					Latitude: "39.000100", Longitude: "-84.000200", AddressHash: "-1000000000000000001",
					FirstReportedDate: "3/2/2011", LastReportedDate: "8/14/2025",
					PublicFirstSeenDate: "3/2/2011", TotalFirstSeenDate: "3/2/2011",
					IsPublic: new(true), IsDeliverable: new(true), IsMergedAddress: new(false),
					UnitCount: new(1), AddressOrder: new(1),
					HighRiskMarker:      PersonSearchHighRiskMarker{IsHighRisk: new(false)},
					PhoneNumbers:        []string{sqPhoneNumber},
					AddressQualityCodes: []string{"AA"},
				}},
				PhoneNumbers: []PersonSearchPhone{
					{
						PhoneNumber: sqPhoneNumber, Company: "Example Wireless", PhoneType: "Wireless",
						Latitude: "39.000100", Longitude: "-84.000200",
						FirstReportedDate: "5/5/2015", LastReportedDate: "9/30/2025", TotalFirstSeenDate: "5/5/2015",
						IsConnected: new(true), IsPublic: new(false), PhoneOrder: new(1),
					},
					{
						PhoneNumber: "(312) 555-0199", Company: "Example Telephone", PhoneType: "LandLine/Services",
						FirstReportedDate: "7/1/2006", LastReportedDate: "12/31/2012",
						PublicFirstSeenDate: "7/1/2006", TotalFirstSeenDate: "7/1/2006",
						IsConnected: new(false), IsPublic: new(true), PhoneOrder: new(2),
					},
				},
				EmailAddresses: []PersonSearchEmail{{
					EmailAddress: "riley.sample@example.com", EmailOrdinal: new(1), IsPremium: new(false), NonBusiness: new(1),
					EmailEngagement: PersonSearchEmailEngagement{
						IsBounce: new(false), IsMatched: new(true), IsGoodDomain: new(true), SendTime: new(9),
						EngagementScore: new(3), Frequency: "Weekly", BestDayOfTheWeek: "Tuesday",
						BestTimeOfTheDay: "Morning", LastCheckDate: "2025-08-14T00:00:00Z",
					},
				}},
				RelativesSummary: []PersonSearchRelative{{
					TahoeID: "G-1000000000000000102", FirstName: "Avery", LastName: dcLast, DOB: "9/XX/1987",
					RelativeLevel: "ab", RelativeType: "Spouse", City: "Exampleton", State: "OH",
					Spouse: new(1), Score: new(850), SharedHouseholdIDs: []string{"H-0000000000000001"},
					IsPrivate: new(false), IsOptedOut: new(false), IsDeceased: new(false),
				}},
				AssociatesSummary: []PersonSearchAssociate{{
					TahoeID: "G-1000000000000000103", FirstName: "Jordan", LastName: "Placeholder", Score: new(450),
				}},
				Indicators: first,
				Extra:      noData,
			},
			{
				TahoeID:  "G-1000000000000000104",
				Name:     PersonSearchName{FirstName: "Riley", LastName: sqLast, Suffix: "Jr", TotalFirstSeenDate: "1/9/2004"},
				FullName: "Riley Sample Jr",
				IsPublic: new(true), IsPremium: new(false), IsOptedOut: new(false), IsEnterpriseOptedOut: new(false),
				HasAdditionalData: new(false), SparseFlag: new(1), Age: new(0), IsCurrentPropertyOwner: "Not Available",
				DeathRecords: PersonSearchDeathRecord{IsDeceased: new(false)},
				Locations:    []PersonSearchLocation{{City: "Testville", State: "OH"}},
				Addresses: []PersonSearchAddress{{
					FullAddress: "200 Placeholder Rd Apt 4B; Testville, OH 45001", HouseNumber: "200",
					StreetName: "Placeholder", StreetType: "Rd", Unit: "4B", UnitType: "Apt", City: "Testville", State: "OH",
					County: sqLast, Zip: "45001", AddressHash: "1000000000000000002",
					FirstReportedDate: "1/9/2004", LastReportedDate: "2/27/2011", TotalFirstSeenDate: "1/9/2004",
					IsPublic: new(true), IsDeliverable: new(false), IsMergedAddress: new(false),
					UnitCount: new(12), AddressOrder: new(1),
					HighRiskMarker: PersonSearchHighRiskMarker{IsHighRisk: new(false)},
				}},
				Indicators: sdZeroIndicators(),
				Extra:      noData,
			},
		},
		Counts: PersonSearchCounts{
			Names: new(3), Addresses: new(2), Relatives: new(1), Associates: new(1), DebtRecords: new(0),
			DatesOfBirth: new(1), DatesOfDeath: new(0), DeathRecords: new(0), PhoneNumbers: new(2),
			ExpectedCount: new(2), SearchResults: new(2), EmailAddresses: new(1), BusinessRecords: new(0),
			EvictionRecords: new(0), ForeclosureRecords: new(0), ForeclosureV2Records: new(0),
			SocialSecurityNumbers: new(0), ProfessionalLicenseRecords: new(0), SearchResultsOverflow: new(false),
		},
		RequestID:             "synthetic-request-0101",
		CriteriaType:          json.RawMessage(`{}`),
		SmartSearchStatistics: json.RawMessage(`{}`),
	}
}

func TestSearchDecodeProviderPage(t *testing.T) {
	body := testutil.Fixture(t, "provider/person-search-page.json")
	for _, diag := range []bool{false, true} {
		res, err := sdDecode(body, diag)
		require.NoError(t, err)
		assert.Equal(t, sdPage(), res)
	}
}

func TestSearchDecodeProviderRejected400(t *testing.T) {
	_, err := decodeSearchPersons(&exchange{
		status: http.StatusBadRequest, body: testutil.Fixture(t, "provider/person-search-rejected-400.json"),
	}, true)
	ce := asCallError(t, err)
	assert.Equal(t, FailureRejected, ce.Kind)
	assert.Equal(t, http.StatusBadRequest, ce.StatusCode)
	assert.Empty(t, ce.RequestID)
	assert.Empty(t, ce.Field)
	assert.Equal(t, ProviderDiagnostics{}, ce.Diagnostics())
	require.ErrorIs(t, err, errStatus)
	assert.Equal(t, "enformion: rejected (HTTP 400)", err.Error())
}

func TestSearchDecodeStatusKinds(t *testing.T) {
	page := testutil.Fixture(t, "provider/person-search-page.json")
	for _, status := range []int{100, 201, 204, 302, 400, 401, 403, 404, 429, 500, 503, 600} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			res, err := decodeSearchPersons(&exchange{status: status, body: page}, true)
			assert.Nil(t, res, "a page body never rescues a non-200 status")
			ce := asCallError(t, err)
			assert.Equal(t, statusKind(status), ce.Kind)
			assert.Equal(t, status, ce.StatusCode)
			assert.Equal(t, "synthetic-request-0101", ce.RequestID)
		})
	}
}

// sdAssertFilled fails for every exported field of v, at any depth, that holds
// its zero value. It walks into structs and into the first element of slices
// of structs; skip names paths it leaves alone.
func sdAssertFilled(t *testing.T, v reflect.Value, at string, skip map[string]bool) {
	t.Helper()
	for i := range v.NumField() {
		f, fv := v.Type().Field(i), v.Field(i)
		name := at + "." + f.Name
		if skip[name] {
			continue
		}
		if !assert.False(t, fv.IsZero(), "%s is zero: a slot is missing or unbound", name) {
			continue
		}
		if fv.Kind() == reflect.Struct {
			sdAssertFilled(t, fv, name, skip)
		}
		if elem := name + "[0]"; fv.Kind() == reflect.Slice && fv.Type().Elem().Kind() == reflect.Struct && !skip[elem] {
			sdAssertFilled(t, fv.Index(0), elem, skip)
		}
	}
}

// TestSearchDecodeEveryField decodes a body that fills every member and
// proves no exported field is left at zero, so that no slot is unbound.
func TestSearchDecodeEveryField(t *testing.T) {
	res, err := sdDecode(testutil.Fixture(t, "synthetic/person-search-every-field.json"), false)
	require.NoError(t, err)
	sdAssertFilled(t, reflect.ValueOf(*res), "result", map[string]bool{
		"result.Persons[0].Associates[0]": true, // one level deep; checked below
	})

	p := res.Persons[0]
	assert.Equal(t, map[string]json.RawMessage{
		"driversLicenseDetail": json.RawMessage(`[{"state":"OH"}]`),
		"propensityToPayScore": json.RawMessage(`{"transactionId":"T-0000000001","measures":[]}`),
		"relatives":            json.RawMessage(`[{"tahoeId":"G-1000000000000000111"}]`),
	}, p.Extra, "undocumented members stay reachable")
	assert.Equal(t, []PersonSearchPerson{{
		TahoeID:  "G-1000000000000000112",
		Name:     PersonSearchName{FirstName: "Jordan", LastName: "Placeholder"},
		FullName: "Jordan Placeholder",
		Age:      new(55),
		Extra:    map[string]json.RawMessage{"associates": json.RawMessage(`[{"tahoeId":"G-1000000000000000113"}]`)},
	}}, p.Associates, "an associate's own associates are kept raw")
	assert.Equal(t, json.RawMessage(`{"sources":["synthetic-source-f"]}`), p.Addresses[0].SourceSummary) //nolint:testifylint // byte-exact, not just equivalent JSON
	assert.Equal(t, []json.RawMessage{json.RawMessage(`{"name":"Synthetic Neighbor"}`)}, p.Addresses[0].Neighbors)
	assert.Equal(t, []json.RawMessage{json.RawMessage(`{"date":"9/30/2025"}`)}, p.DatesOfDeath)
	assert.Equal(t, ProviderDiagnostics{
		Message:     "Synthetic message: every field.",
		InputErrors: []json.RawMessage{json.RawMessage(`{"field":"Synthetic"}`)},
		Warnings:    []json.RawMessage{json.RawMessage(`"Synthetic warning."`)},
	}, res.Diagnostics)
	assert.Equal(t, PersonSearchPagination{
		CurrentPageNumber: new(2), ResultsPerPage: new(1), TotalPages: new(3), TotalResults: new(3),
	}, res.Pagination)
}

func TestSearchDecodePersonsPresence(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		for _, body := range []string{`{"isError":false}`, `{"isError":false,"person":{"age":"41"},"requestId":"rid-1"}`} {
			res, err := sdDecode(body, false)
			assert.Nil(t, res)
			ce := asCallError(t, err)
			assert.Equal(t, FailureUnexpectedResponse, ce.Kind)
			assert.Equal(t, "persons", ce.Field)
			require.ErrorIs(t, err, errMissingMember)
			require.ErrorIs(t, err, ErrUnexpectedResponse)
		}
	})
	tests := []struct {
		name    string
		persons string
		want    []PersonSearchPerson
	}{
		{name: "null", persons: jsonNull},
		{name: "empty", persons: `[]`},
		{name: "only null elements", persons: `[null,null]`},
		{name: "an empty object is an entry", persons: `[{}]`, want: []PersonSearchPerson{{}}},
		{name: "defaults are an entry", persons: `[{"tahoeId":"","age":0,"isPublic":false}]`, want: []PersonSearchPerson{{Age: new(0), IsPublic: new(false)}}},
		{
			name: "order and duplicates are the provider's", persons: `[{"tahoeId":"G-2"},null,{"tahoeId":"G-1"},{"tahoeId":"G-2"}]`,
			want: []PersonSearchPerson{{TahoeID: "G-2"}, {TahoeID: "G-1"}, {TahoeID: "G-2"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := sdDecode(`{"isError":false,"requestId":"rid-1","persons":`+tt.persons+`}`, false)
			require.NoError(t, err)
			assert.Equal(t, tt.want, res.Persons)
			assert.Equal(t, "rid-1", res.RequestID)
		})
	}
}

func TestSearchDecodeSyntheticFixtures(t *testing.T) {
	t.Run("no persons", func(t *testing.T) {
		res, err := sdDecode(testutil.Fixture(t, "synthetic/person-search-no-persons.json"), false)
		require.NoError(t, err)
		assert.Nil(t, res.Persons)
		assert.Equal(t, "synthetic-request-0102", res.RequestID)
		require.NotNil(t, res.Counts.SearchResults)
		assert.Zero(t, *res.Counts.SearchResults)
		assert.Equal(t, []json.RawMessage{json.RawMessage(`"Synthetic warning: no persons were returned."`)}, res.Diagnostics.Warnings)
		assert.Equal(t, PersonSearchPagination{}, res.Pagination)
	})
	t.Run("is error", func(t *testing.T) {
		body := testutil.Fixture(t, "synthetic/person-search-is-error.json")
		for _, diag := range []bool{false, true} {
			res, err := sdDecode(body, diag)
			assert.Nil(t, res, "a refusal never returns its candidates")
			require.ErrorIs(t, err, ErrRejected)
			ce := asCallError(t, err)
			assert.Equal(t, http.StatusOK, ce.StatusCode)
			assert.Equal(t, "synthetic-request-0103", ce.RequestID)
			want := ProviderDiagnostics{}
			if diag {
				want = ProviderDiagnostics{
					Message: "Synthetic message: the search was refused.",
					InputErrors: []json.RawMessage{
						json.RawMessage(`{"field":"Addresses","message":"Synthetic input error."}`),
						json.RawMessage(`"Synthetic input error as a string."`),
					},
					Warnings: []json.RawMessage{json.RawMessage(`7`)},
				}
			}
			assert.Equal(t, want, ce.Diagnostics())
		}
	})
	t.Run("missing persons", func(t *testing.T) {
		_, err := sdDecode(testutil.Fixture(t, "synthetic/person-search-missing-persons.json"), false)
		ce := asCallError(t, err)
		assert.Equal(t, FailureUnexpectedResponse, ce.Kind)
		assert.Equal(t, "persons", ce.Field)
		assert.Equal(t, "synthetic-request-0104", ce.RequestID)
	})
	t.Run("defaults person", func(t *testing.T) {
		res, err := sdDecode(testutil.Fixture(t, "synthetic/person-search-defaults-person.json"), false)
		require.NoError(t, err)
		assert.Equal(t, []PersonSearchPerson{{
			IsPublic: new(false), IsPremium: new(false), IsOptedOut: new(false), IsEnterpriseOptedOut: new(false),
			HasAdditionalData: new(false), SparseFlag: new(0), Age: new(0),
			DeathRecords: PersonSearchDeathRecord{IsDeceased: new(false)},
			Indicators:   PersonSearchIndicators{HasBankruptcyRecords: new(0), HasDEARecords: new(0)},
		}}, res.Persons, "an entry of defaults is kept, without data")
	})
	t.Run("unknown keys person", func(t *testing.T) {
		_, err := sdDecode(testutil.Fixture(t, "synthetic/person-search-unknown-keys-person.json"), false)
		ce := asCallError(t, err)
		assert.Equal(t, FailureUnexpectedResponse, ce.Kind)
		assert.Equal(t, "persons[0]", ce.Field)
		require.ErrorIs(t, err, errUnrecognizedData)
		assert.Equal(t, "synthetic-request-0106", ce.RequestID)
	})
	t.Run("duplicates", func(t *testing.T) {
		res, err := sdDecode(testutil.Fixture(t, "synthetic/person-search-duplicates.json"), false)
		require.NoError(t, err)
		phone := PersonSearchPhone{PhoneNumber: sqPhoneNumber, PhoneOrder: new(1)}
		assert.Equal(t, []PersonSearchPerson{
			{TahoeID: "G-1000000000000000107", FullName: "Riley Sample", PhoneNumbers: []PersonSearchPhone{phone, phone}},
			{TahoeID: "G-1000000000000000107", FullName: "Riley Sample", PhoneNumbers: []PersonSearchPhone{phone}},
			{TahoeID: "G-1000000000000000108", FullName: "Avery Example"},
		}, res.Persons, "candidates and entries are never merged or deduplicated")
	})
	t.Run("teaser masked", func(t *testing.T) {
		res, err := sdDecode(testutil.Fixture(t, "synthetic/person-search-teaser-masked.json"), false)
		require.NoError(t, err)
		require.Len(t, res.Persons, 1)
		p := res.Persons[0]
		assert.Equal(t, "Riley S*****", p.FullName, "masked text is kept verbatim")
		assert.Equal(t, "(312) 555-XXXX", p.PhoneNumbers[0].PhoneNumber)
		assert.Nil(t, p.PhoneNumbers[0].IsConnected)
		assert.Equal(t, "XX/XXXX", p.RelativesSummary[0].DOB)
		assert.Empty(t, p.DOB)
		assert.Equal(t, PersonSearchPagination{
			CurrentPageNumber: new(1), ResultsPerPage: new(1), TotalPages: new(1), TotalResults: new(1),
		}, res.Pagination)
	})
}

func TestSearchDecodeFailures(t *testing.T) {
	p := func(person string) string { return sdPersons(`{"tahoeId":"G-1"},` + person) }
	tests := []struct {
		name, body, field string
		kind              FailureKind
		cause             error
	}{
		{name: "persons string", body: `{"isError":false,"persons":"x"}`, field: "persons", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "persons object", body: `{"isError":false,"persons":{}}`, field: "persons", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "person string", body: sdPersons(`null,"x"`), field: "persons[1]", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "person array", body: sdPersons(`[]`), field: "persons[0]", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "person unknown keys only", body: p(`{"other":"x"}`), field: "persons[1]", kind: FailureUnexpectedResponse, cause: errUnrecognizedData},
		{name: "person defaults and unknown data", body: p(`{"age":0,"isPublic":false,"x":{"y":1}}`), field: "persons[1]", kind: FailureUnexpectedResponse, cause: errUnrecognizedData},
		{name: "tahoe ID number", body: p(`{"tahoeId":1}`), field: "persons[1].tahoeId", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "age string", body: p(`{"age":"41"}`), field: "persons[1].age", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "age fraction", body: p(`{"age":41.5}`), field: "persons[1].age", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "age exponent", body: p(`{"age":4e1}`), field: "persons[1].age", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "flag string", body: p(`{"isOptedOut":"false"}`), field: "persons[1].isOptedOut", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "owner boolean", body: p(`{"isCurrentPropertyOwner":true}`), field: "persons[1].isCurrentPropertyOwner", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "name string", body: p(`{"name":"Riley"}`), field: "persons[1].name", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "name unknown keys only", body: p(`{"name":{"first":"Riley"}}`), field: "persons[1].name", kind: FailureUnexpectedResponse, cause: errUnrecognizedData},
		{name: "raw name entry", body: p(`{"name":{"rawNames":["a",5]}}`), field: "persons[1].name.rawNames[1]", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "aka first name", body: p(`{"akas":[{},{"firstName":5}]}`), field: "persons[1].akas[1].firstName", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "merged name unknown keys only", body: p(`{"mergedNames":[{"given":"R"}]}`), field: "persons[1].mergedNames[0]", kind: FailureUnexpectedResponse, cause: errUnrecognizedData},
		{name: "date of birth unknown keys only", body: p(`{"datesOfBirth":[{"age":0,"dob":"1/1980"}]}`), field: "persons[1].datesOfBirth[0]", kind: FailureUnexpectedResponse, cause: errUnrecognizedData},
		{name: "date of birth age", body: p(`{"datesOfBirth":[{"age":"41"}]}`), field: "persons[1].datesOfBirth[0].age", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "dates of death object", body: p(`{"datesOfDeath":{}}`), field: "persons[1].datesOfDeath", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "death records list", body: p(`{"deathRecords":[]}`), field: "persons[1].deathRecords", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "death records unknown keys only", body: p(`{"deathRecords":{"date":"x"}}`), field: "persons[1].deathRecords", kind: FailureUnexpectedResponse, cause: errUnrecognizedData},
		{name: "location unknown keys only", body: p(`{"locations":[{"town":"Exampleton"}]}`), field: "persons[1].locations[0]", kind: FailureUnexpectedResponse, cause: errUnrecognizedData},
		{name: "addresses object", body: p(`{"addresses":{}}`), field: "persons[1].addresses", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "address entry string", body: p(`{"addresses":[{},"x"]}`), field: "persons[1].addresses[1]", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "address zip number", body: p(`{"addresses":[{"zip":45000}]}`), field: "persons[1].addresses[0].zip", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "address latitude number", body: p(`{"addresses":[{"latitude":39.1}]}`), field: "persons[1].addresses[0].latitude", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "address unit count fraction", body: p(`{"addresses":[{"unitCount":1.5}]}`), field: "persons[1].addresses[0].unitCount", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "high risk marker string", body: p(`{"addresses":[{"highRiskMarker":"no"}]}`), field: "persons[1].addresses[0].highRiskMarker", kind: FailureMalformedResponse, cause: errWrongType},
		{
			name: "high risk flag string", body: p(`{"addresses":[{"highRiskMarker":{"isHighRisk":"no"}}]}`),
			field: "persons[1].addresses[0].highRiskMarker.isHighRisk", kind: FailureMalformedResponse, cause: errWrongType,
		},
		{name: "address phone number", body: p(`{"addresses":[{"phoneNumbers":[3125550142]}]}`), field: "persons[1].addresses[0].phoneNumbers[0]", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "neighbors object", body: p(`{"addresses":[{"neighbors":{}}]}`), field: "persons[1].addresses[0].neighbors", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "address unknown keys only", body: p(`{"addresses":[{"street":"100 Sample Way"}]}`), field: "persons[1].addresses[0]", kind: FailureUnexpectedResponse, cause: errUnrecognizedData},
		{
			name: "folded duplicate past the inline storage", body: p(`{"addresses":[{"sourceSummary":null,"SourceSummary":null}]}`),
			field: "persons[1].addresses[0].sourceSummary", kind: FailureMalformedResponse, cause: errDuplicateKey,
		},
		{name: "folded duplicate person member", body: p(`{"indicators":{},"Indicators":{}}`), field: "persons[1].indicators", kind: FailureMalformedResponse, cause: errDuplicateKey},
		{name: "phone connected string", body: p(`{"phoneNumbers":[{"isConnected":"yes"}]}`), field: "persons[1].phoneNumbers[0].isConnected", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "phone order string", body: p(`{"phoneNumbers":[{"phoneOrder":"1"}]}`), field: "persons[1].phoneNumbers[0].phoneOrder", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "email non-business boolean", body: p(`{"emailAddresses":[{"nonBusiness":true}]}`), field: "persons[1].emailAddresses[0].nonBusiness", kind: FailureMalformedResponse, cause: errWrongType},
		{
			name: "engagement send time string", body: p(`{"emailAddresses":[{"emailEngagementData":{"sendTime":"9"}}]}`),
			field: "persons[1].emailAddresses[0].emailEngagementData.sendTime", kind: FailureMalformedResponse, cause: errWrongType,
		},
		{
			name: "engagement naics string", body: p(`{"emailAddresses":[{"emailEngagementData":{"naics":"111111"}}]}`),
			field: "persons[1].emailAddresses[0].emailEngagementData.naics", kind: FailureMalformedResponse, cause: errWrongType,
		},
		{name: "relative score fraction", body: p(`{"relativesSummary":[{"score":850.5}]}`), field: "persons[1].relativesSummary[0].score", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "relative unknown keys only", body: p(`{"relativesSummary":[{"oldSpouse":1}]}`), field: "persons[1].relativesSummary[0]", kind: FailureUnexpectedResponse, cause: errUnrecognizedData},
		{name: "relative household ID", body: p(`{"relativesSummary":[{"sharedHouseholdIds":[1]}]}`), field: "persons[1].relativesSummary[0].sharedHouseholdIds[0]", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "associate summary score", body: p(`{"associatesSummary":[{"score":"450"}]}`), field: "persons[1].associatesSummary[0].score", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "indicators list", body: p(`{"indicators":[]}`), field: "persons[1].indicators", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "indicator boolean", body: p(`{"indicators":{"hasDeaRecords":true}}`), field: "persons[1].indicators.hasDeaRecords", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "associate age string", body: p(`{"associates":[{"age":"55"}]}`), field: "persons[1].associates[0].age", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "associate unknown keys only", body: p(`{"associates":[{"other":"x"}]}`), field: "persons[1].associates[0]", kind: FailureUnexpectedResponse, cause: errUnrecognizedData},
		{
			name: "associate address zip", body: p(`{"associates":[{"addresses":[{},{"zip":1}]}]}`),
			field: "persons[1].associates[0].addresses[1].zip", kind: FailureMalformedResponse, cause: errWrongType,
		},
		{
			name: "deepest path", body: p(`{"associates":[{},{"emailAddresses":[{"emailEngagementData":{"naics":["1",5]}}]}]}`),
			field: "persons[1].associates[1].emailAddresses[0].emailEngagementData.naics[1]", kind: FailureMalformedResponse, cause: errWrongType,
		},
		{name: "pagination list", body: `{"isError":false,"persons":[],"pagination":[]}`, field: "pagination", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "pagination string member", body: `{"isError":false,"persons":[],"pagination":{"totalPages":"3"}}`, field: "pagination.totalPages", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "counts fraction", body: `{"isError":false,"persons":[],"counts":{"searchResults":1.5}}`, field: "counts.searchResults", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "counts overflow string", body: `{"isError":false,"persons":[],"counts":{"searchResultsOverflow":"no"}}`, field: "counts.searchResultsOverflow", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "message number", body: `{"isError":false,"persons":[],"message":5}`, field: "message", kind: FailureMalformedResponse, cause: errWrongType},
		{name: "warnings string", body: `{"isError":false,"persons":[],"error":{"warnings":"w"}}`, field: "error.warnings", kind: FailureMalformedResponse, cause: errWrongType},
		{
			name: "folded duplicate envelope member past the inline storage", body: `{"isError":false,"persons":[],"smartSearchStatistics":{},"SmartSearchStatistics":{}}`,
			field: "smartSearchStatistics", kind: FailureMalformedResponse, cause: errDuplicateKey,
		},
		{name: "folded duplicate persons", body: `{"isError":false,"persons":[],"Persons":[]}`, field: "persons", kind: FailureMalformedResponse, cause: errDuplicateKey},
		{name: "isError missing", body: `{"persons":[]}`, field: isErrorKey, kind: FailureUnexpectedResponse, cause: errIsErrorFlag},
		{name: "isError string", body: `{"persons":[],"isError":"false"}`, field: isErrorKey, kind: FailureUnexpectedResponse, cause: errIsErrorFlag},
		{name: "isError true with malformed persons", body: `{"persons":5,"isError":true}`, kind: FailureRejected, cause: errProviderError},
		{name: "duplicate key", body: `{"persons":[],"isError":false,"isError":false}`, kind: FailureMalformedResponse, cause: errDuplicateKey},
		{name: "not an object", body: `[]`, kind: FailureUnexpectedResponse, cause: errNotObject},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := sdDecode(tt.body, true)
			assert.Nil(t, res)
			ce := asCallError(t, err)
			assert.Equal(t, tt.kind, ce.Kind)
			assert.Equal(t, tt.field, ce.Field)
			assert.Equal(t, http.StatusOK, ce.StatusCode)
			require.ErrorIs(t, err, tt.cause)
			assert.Equal(t, ProviderDiagnostics{}, ce.Diagnostics(), "a decoding failure keeps no diagnostics")
		})
	}
}

// TestSearchDecodeMetadataIgnoresUnknown proves that flag and metadata objects
// never fail on members the SDK does not know, even when every known member
// is zero.
func TestSearchDecodeMetadataIgnoresUnknown(t *testing.T) {
	body := `{"isError":false,"pagination":{"pageCursor":"c-2","totalPages":0},"counts":{"newRecords":7,"names":0},` +
		`"persons":[{"tahoeId":"G-1","indicators":{"hasNewRecords":1,"hasDeaRecords":0},` +
		`"addresses":[{"city":"Exampleton","highRiskMarker":{"reason":"synthetic","isHighRisk":false}}],` +
		`"emailAddresses":[{"emailAddress":"a@example.com","emailEngagementData":{"newScore":3,"sendTime":0}}]}]}`
	res, err := sdDecode(body, false)
	require.NoError(t, err)
	assert.Equal(t, PersonSearchPagination{TotalPages: new(0)}, res.Pagination)
	assert.Equal(t, PersonSearchCounts{Names: new(0)}, res.Counts)
	p := res.Persons[0]
	assert.Equal(t, PersonSearchIndicators{HasDEARecords: new(0)}, p.Indicators)
	assert.Equal(t, PersonSearchHighRiskMarker{IsHighRisk: new(false)}, p.Addresses[0].HighRiskMarker)
	assert.Equal(t, PersonSearchEmailEngagement{SendTime: new(0)}, p.EmailAddresses[0].EmailEngagement)
	assert.Nil(t, p.Extra, "nested members are never person-level extras")
}

func TestSearchDecodeIgnoredEnvelopeMembers(t *testing.T) {
	body := `{"isError":false,"persons":[],"searchCriteria":{"echo":"Riley"},"databaseQueryInfo":7,` +
		`"requestType":[],"requestTime":null,"totalRequestExecutionTimeMs":"slow","newMember":{"x":[1]},"person":5}`
	res, err := sdDecode(body, false)
	require.NoError(t, err)
	assert.Equal(t, &PersonSearchResult{}, res)
}

func TestSearchDecodeCaseVariants(t *testing.T) {
	body := `{"IsError":false,"RequestId":"rid-case","Persons":[{"TahoeId":"G-1","Name":{"FirstName":"Riley"},` +
		`"PhoneNumbers":[{"PhoneNumber":"(312) 555-0142","IsConnected":true}],"Indicators":{"HasDeaRecords":1}}]}`
	res, err := sdDecode(body, false)
	require.NoError(t, err)
	assert.Equal(t, "rid-case", res.RequestID)
	assert.Equal(t, []PersonSearchPerson{{
		TahoeID:      "G-1",
		Name:         PersonSearchName{FirstName: "Riley"},
		PhoneNumbers: []PersonSearchPhone{{PhoneNumber: sqPhoneNumber, IsConnected: new(true)}},
		Indicators:   PersonSearchIndicators{HasDEARecords: new(1)},
	}}, res.Persons, "the documentation's PascalCase spelling decodes too")
}

func TestSearchDecodeRawValues(t *testing.T) {
	t.Run("null and absent are nil", func(t *testing.T) {
		res, err := sdDecode(`{"isError":false,"criteriaType":null,"persons":[{"tahoeId":"G-1","name":{"sourceSummary":null},`+
			`"datesOfDeath":[null],"addresses":[{"city":"x","neighbors":[]}]}]}`, false)
		require.NoError(t, err)
		assert.Nil(t, res.CriteriaType)
		assert.Nil(t, res.SmartSearchStatistics)
		p := res.Persons[0]
		assert.Nil(t, p.Name.SourceSummary)
		assert.Nil(t, p.DatesOfDeath)
		assert.Nil(t, p.Addresses[0].Neighbors)
	})
	t.Run("values are normalized", func(t *testing.T) {
		body := testutil.Esc(`{"isError":false,"smartSearchStatistics": { "steps" : [ 1 , "<a>&" ] },` +
			`"persons":[{"tahoeId":"G-1","phoneNumbers":[{"phoneNumber":"1","sourceSummary":"~ud800"}]}]}`)
		res, err := sdDecode(body, false)
		require.NoError(t, err)
		assert.Equal(t, testutil.Esc(`{"steps":[1,"~u003ca~u003e~u0026"]}`), string(res.SmartSearchStatistics))
		assert.Equal(t, testutil.Esc(`"~ufffd"`), string(res.Persons[0].PhoneNumbers[0].SourceSummary))
	})
	t.Run("a raw value with data is the entry's data", func(t *testing.T) {
		res, err := sdDecode(sdPersons(`{"datesOfBirth":[{"age":0,"sourceSummary":{"s":"x"}}]}`), false)
		require.NoError(t, err)
		assert.Equal(t, json.RawMessage(`{"s":"x"}`), res.Persons[0].DatesOfBirth[0].SourceSummary) //nolint:testifylint // byte-exact, not just equivalent JSON

		_, err = sdDecode(sdPersons(`{"deathRecords":{"sourceSummary":{"s":0},"date":"x"}}`), false)
		require.ErrorIs(t, err, errUnrecognizedData, "a raw value without data is no data")
	})
}

func TestSearchDecodeExtra(t *testing.T) {
	body := testutil.Esc(`{"isError":false,"persons":[{"tahoeId":"G-1","driversLicenseDetail":[{"state":"OH"}],` +
		`"PropensityToPayScore":{ "measures" : [] },"newFlag":false,"nothing":null,"html":"<b>",` +
		`"~u0065scaped":1,"relatives":[{"tahoeId":"G-2"}]}]}`)
	res, err := sdDecode(body, false)
	require.NoError(t, err)
	assert.Equal(t, map[string]json.RawMessage{
		"driversLicenseDetail": json.RawMessage(`[{"state":"OH"}]`),
		"PropensityToPayScore": json.RawMessage(`{"measures":[]}`),
		"newFlag":              json.RawMessage(`false`),
		"html":                 json.RawMessage(testutil.Esc(`"~u003cb~u003e"`)),
		"escaped":              json.RawMessage(`1`),
		"relatives":            json.RawMessage(`[{"tahoeId":"G-2"}]`),
	}, res.Persons[0].Extra, "keys as sent, escapes decoded; values normalized; null skipped")

	res, err = sdDecode(sdPersons(`{"tahoeId":"G-1","nothing":null}`), false)
	require.NoError(t, err)
	assert.Nil(t, res.Persons[0].Extra, "no extras is nil, not empty")

	res, err = sdDecode(sdPersons(`{"":{}}`), false)
	require.NoError(t, err)
	assert.Equal(t, []PersonSearchPerson{{Extra: map[string]json.RawMessage{"": json.RawMessage(`{}`)}}}, res.Persons,
		"an empty key is kept as sent, and members without data are no data")

	_, err = sdDecode(sdPersons(`{"driversLicenseDetail":[{"state":"OH"}]}`), false)
	ce := asCallError(t, err)
	assert.Equal(t, FailureUnexpectedResponse, ce.Kind, "a person with nothing recognized still fails")
	assert.Equal(t, "persons[0]", ce.Field)
}

func TestSearchDecodeAssociatesOneLevel(t *testing.T) {
	res, err := sdDecode(sdPersons(`{"tahoeId":"G-1","associates":[{"tahoeId":"G-2","associates":[{"tahoeId":"G-3",`+
		`"associates":[{"tahoeId":"G-4"}]}]}]}`), false)
	require.NoError(t, err)
	require.Len(t, res.Persons[0].Associates, 1)
	a := res.Persons[0].Associates[0]
	assert.Equal(t, "G-2", a.TahoeID)
	assert.Nil(t, a.Associates)
	assert.Equal(t, map[string]json.RawMessage{
		"associates": json.RawMessage(`[{"tahoeId":"G-3","associates":[{"tahoeId":"G-4"}]}]`),
	}, a.Extra)

	_, err = sdDecode(sdPersons(`{"tahoeId":"G-1","associates":[{"associates":[{"tahoeId":"G-3"}]}]}`), false)
	ce := asCallError(t, err)
	assert.Equal(t, "persons[0].associates[0]", ce.Field, "an associate with only its own associates has nothing recognized")
}

// TestSearchDecodeBudgetEdge proves the budget at its edge: a list entry costs
// its size, an Extra member its key, value and entry overhead.
func TestSearchDecodeBudgetEdge(t *testing.T) {
	person := int(reflect.TypeFor[PersonSearchPerson]().Size())
	raw := []byte(`[{"tahoeId":"G-1"},{"tahoeId":"G-2"}]`)
	d := psDecoder{budget: budget(2 * person)}
	got, _, derr := decodeList(raw, at("persons", -1), &d.budget, d.person)
	require.Nil(t, derr)
	assert.Len(t, got, 2)
	assert.Zero(t, d.budget)

	d = psDecoder{budget: budget(2*person - 1)}
	_, _, derr = decodeList(raw, at("persons", -1), &d.budget, d.person)
	require.NotNil(t, derr)
	assert.Equal(t, FailureResponseTooLarge, derr.kind)
	assert.Equal(t, errDecodeBudget, derr.cause)
	assert.Equal(t, "persons[1]", derr.field)

	extra := []byte(`[{"tahoeId":"G-1","xy":"<"}]`)
	cost := person + len("xy") + len(testutil.Esc(`"~u003c"`)) + extraEntryBytes
	d = psDecoder{budget: budget(cost)}
	_, _, derr = decodeList(extra, at("persons", -1), &d.budget, d.person)
	require.Nil(t, derr)
	assert.Zero(t, d.budget)
	d = psDecoder{budget: budget(cost - 1)}
	_, _, derr = decodeList(extra, at("persons", -1), &d.budget, d.person)
	require.NotNil(t, derr)
	assert.Equal(t, "persons[0]", derr.field, "an Extra failure names the person, never the key")

	assert.Equal(t, newBudget(1000), budget(4*1000+1<<20), "the budget of a whole response")
}

func TestSearchDecodeErrorsNeverQuoteValues(t *testing.T) {
	tests := []struct {
		body   string
		values []string
	}{
		{body: sdPersons(`{"age":"31337"}`), values: []string{"31337"}},
		{body: sdPersons(`{"tahoeId":98765432109876543210}`), values: []string{"98765", "43210"}},
		{body: sdPersons(`{"addresses":[{"zip":45099}]}`), values: []string{"45099"}},
		{body: sdPersons(`{"fullNameX":"Zephyrine Quill"}`), values: []string{"Zephyrine", "Quill", "fullNameX"}},
		{body: sdPersons(`{"tahoeId":"G-1","Zephyrine":"x","age":"Quill"}`), values: []string{"Zephyrine", "Quill"}},
		{body: `{"isError":false,"persons":[],"counts":{"names":7.25e3}}`, values: []string{"7.25", "e3"}},
		{body: `{"isError":"Zephyrine","persons":[]}`, values: []string{"Zephyrine"}},
	}
	for _, tt := range tests {
		_, err := sdDecode(tt.body, true)
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

func TestSearchDecodeNeverAliasesBody(t *testing.T) {
	src := string(testutil.Fixture(t, "synthetic/person-search-every-field.json"))
	body := []byte(src)
	res, err := sdDecode(body, false)
	require.NoError(t, err)
	want, err := sdDecode(src, false)
	require.NoError(t, err)
	for i := range body {
		body[i] = 'Z'
	}
	assert.Equal(t, want, res, "no string, raw value or Extra member aliases the body")
}

func TestSearchDecodeJSONRoundTrip(t *testing.T) {
	bodies := map[string]string{
		"page":          string(testutil.Fixture(t, "provider/person-search-page.json")),
		"every field":   string(testutil.Fixture(t, "synthetic/person-search-every-field.json")),
		"no persons":    string(testutil.Fixture(t, "synthetic/person-search-no-persons.json")),
		"defaults":      string(testutil.Fixture(t, "synthetic/person-search-defaults-person.json")),
		"duplicates":    string(testutil.Fixture(t, "synthetic/person-search-duplicates.json")),
		"teaser masked": string(testutil.Fixture(t, "synthetic/person-search-teaser-masked.json")),
		"zeros and false": sdPersons(`{"age":0,"isPublic":false,"indicators":{"hasDeaRecords":0},` +
			`"emailAddresses":[{"nonBusiness":0,"isPremium":false}]}`),
		"html and separators": "{\"isError\":false,\"persons\":[{\"fullName\":\"<b>&\u2028\",\"extraMember\":[\"<i>\u2029\"]}]}",
		"escapes":             testutil.Esc(`{"isError":false,"persons":[{"fullName":"~ud800~u0000","x":"~ud83d~ude00"}]}`),
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			res, err := sdDecode(body, false)
			require.NoError(t, err)
			data, err := json.Marshal(res)
			require.NoError(t, err)
			var back PersonSearchResult
			require.NoError(t, json.Unmarshal(data, &back))
			assert.True(t, reflect.DeepEqual(*res, back), "a decoded result round-trips through JSON")
		})
	}
	t.Run("pointer semantics survive", func(t *testing.T) {
		res, err := sdDecode(bodies["zeros and false"], false)
		require.NoError(t, err)
		data, err := json.Marshal(res.Persons[0])
		require.NoError(t, err)
		assert.JSONEq(t, `{"isPublic":false,"age":0,"emailAddresses":[{"isPremium":false,"nonBusiness":0}],`+
			`"indicators":{"hasDeaRecords":0}}`, string(data))
	})
	t.Run("the extra map keeps its members apart", func(t *testing.T) {
		data, err := json.Marshal(PersonSearchPerson{TahoeID: "G-1", Extra: map[string]json.RawMessage{"tahoeId": json.RawMessage(`"G-2"`)}})
		require.NoError(t, err)
		assert.JSONEq(t, `{"tahoeId":"G-1","extra":{"tahoeId":"G-2"}}`, string(data))
		assert.True(t, strings.HasPrefix(string(data), `{"tahoeId":"G-1"`))
	})
}
