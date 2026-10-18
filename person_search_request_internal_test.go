package enformion

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion/internal/testutil"
)

// Literals the Person Search request tests repeat. sqMarker prefixes rejected
// values, so a test can prove none reaches an error.
const (
	sqMarker = "SQMARKER"
	sqLast   = "Sample"
	sqPhone  = "312-555-0142"
	sqDOB    = "DOB"
)

// sqBase returns a valid Teaser request with one criterion, a name.
func sqBase() PersonSearchRequest {
	return PersonSearchRequest{Mode: PersonSearchModeTeaser, FirstName: "Riley", LastName: sqLast}
}

// sqFull returns a valid request that fills every field, each list with two
// entries.
func sqFull() PersonSearchRequest {
	return PersonSearchRequest{
		Mode:      PersonSearchModePerson,
		FirstName: "Riley", MiddleName: "Q", LastName: sqLast,
		AKAs: []PersonSearchRequestName{
			{FirstName: "Rye", LastName: sqLast},
			{FirstName: "Riley", MiddleName: "J", LastName: dcLast},
		},
		DOB: "06/01/1985", Age: 41, AgeRangeMinAge: 35, AgeRangeMaxAge: 45, AgeRange: "35-45",
		Addresses: []PersonSearchRequestAddress{
			{AddressLine1: "100 Sample Way", AddressLine2: "Exampleton, OH", County: sqLast},
			{AddressLine2: "45000"},
		},
		Email:    "riley.sample@example.com",
		Phone:    "(312) 555-0142",
		ClientIP: "192.0.2.10",
		Relatives: []PersonSearchRequestName{
			{FirstName: "Avery", LastName: dcLast},
			{LastName: sqLast},
		},
		TahoeIDs:            []string{"G-1000000000000000042", "G-1000000000000000043"},
		FirstNameCharOffset: new(0),
		LastNameCharOffset:  new(2),
		DOBFormat:           "mm-dd-yyyy",
		MaxAddressYears:     new(10),
		MaxPhoneYears:       new(0),
		Includes:            []PersonSearchInclude{PersonSearchIncludeAddresses, PersonSearchIncludePhoneNumbers},
		FilterOptions:       []PersonSearchFilter{PersonSearchFilterIncludeLowQualityAddresses, PersonSearchFilterDisableNoDobs},
		Page:                2,
		ResultsPerPage:      10,
		SessionID:           "batch-7f3a9c-0001",
	}
}

// sqFullBody is the exact body sqFull encodes to.
const sqFullBody = `{"FirstName":"Riley","MiddleName":"Q","LastName":"Sample",` +
	`"Akas":[{"FirstName":"Rye","LastName":"Sample"},{"FirstName":"Riley","MiddleName":"J","LastName":"Example"}],` +
	`"Dob":"06/01/1985","Age":41,"AgeRangeMinAge":35,"AgeRangeMaxAge":45,"AgeRange":"35-45",` +
	`"Addresses":[{"AddressLine1":"100 Sample Way","AddressLine2":"Exampleton, OH","County":"Sample"},{"AddressLine2":"45000"}],` +
	`"Email":"riley.sample@example.com","ClientIp":"192.0.2.10","Phone":"(312) 555-0142",` +
	`"Relatives":[{"FirstName":"Avery","LastName":"Example"},{"LastName":"Sample"}],` +
	`"TahoeIds":["G-1000000000000000042","G-1000000000000000043"],` +
	`"FirstNameCharOffset":0,"LastNameCharOffset":2,"DobFormat":"mm-dd-yyyy","MaxAddressYears":10,"MaxPhoneYears":0,` +
	`"Includes":["Addresses","PhoneNumbers"],"FilterOptions":["IncludeLowQualityAddresses","DisableNoDobs"],` +
	`"Page":2,"ResultsPerPage":10}`

// sqTextField is a request text field, by the name errors report, and how to
// set it on sqFull.
type sqTextField struct {
	name string
	set  func(*PersonSearchRequest, string)
}

// sqTextFields returns setters for every text field, list entries included,
// in validation order.
func sqTextFields() []sqTextField {
	return []sqTextField{
		{"FirstName", func(r *PersonSearchRequest, v string) { r.FirstName = v }},
		{"MiddleName", func(r *PersonSearchRequest, v string) { r.MiddleName = v }},
		{"LastName", func(r *PersonSearchRequest, v string) { r.LastName = v }},
		{"AKAs[0].FirstName", func(r *PersonSearchRequest, v string) { r.AKAs[0].FirstName = v }},
		{"AKAs[1].MiddleName", func(r *PersonSearchRequest, v string) { r.AKAs[1].MiddleName = v }},
		{"AKAs[1].LastName", func(r *PersonSearchRequest, v string) { r.AKAs[1].LastName = v }},
		{sqDOB, func(r *PersonSearchRequest, v string) { r.DOB = v }},
		{"AgeRange", func(r *PersonSearchRequest, v string) { r.AgeRange = v }},
		{"Addresses[0].AddressLine1", func(r *PersonSearchRequest, v string) { r.Addresses[0].AddressLine1 = v }},
		{"Addresses[0].AddressLine2", func(r *PersonSearchRequest, v string) { r.Addresses[0].AddressLine2 = v }},
		{"Addresses[1].County", func(r *PersonSearchRequest, v string) { r.Addresses[1].County = v }},
		{"Email", func(r *PersonSearchRequest, v string) { r.Email = v }},
		{"Phone", func(r *PersonSearchRequest, v string) { r.Phone = v }},
		{"ClientIP", func(r *PersonSearchRequest, v string) { r.ClientIP = v }},
		{"Relatives[0].FirstName", func(r *PersonSearchRequest, v string) { r.Relatives[0].FirstName = v }},
		{"Relatives[1].MiddleName", func(r *PersonSearchRequest, v string) { r.Relatives[1].MiddleName = v }},
		{"TahoeIDs[1]", func(r *PersonSearchRequest, v string) { r.TahoeIDs[1] = v }},
		{"DOBFormat", func(r *PersonSearchRequest, v string) { r.DOBFormat = v }},
	}
}

func TestSearchRequestWireEncoding(t *testing.T) {
	tests := []struct {
		name string
		req  PersonSearchRequest
		want string
	}{
		{name: "every field", req: sqFull(), want: sqFullBody},
		{name: "a name", req: sqBase(), want: `{"FirstName":"Riley","LastName":"Sample"}`},
		{
			name: "zero values and nil pointers are omitted",
			req: PersonSearchRequest{
				Mode: PersonSearchModePerson, Phone: sqPhone, Age: 0, Page: 0, ResultsPerPage: 0,
				AKAs: []PersonSearchRequestName{}, Includes: []PersonSearchInclude{},
			},
			want: `{"Phone":"312-555-0142"}`,
		},
		{
			name: "pointers to 0 are sent",
			req: PersonSearchRequest{
				Mode: PersonSearchModeReversePhonePerson, Phone: sqPhone,
				FirstNameCharOffset: new(0), LastNameCharOffset: new(0), MaxAddressYears: new(0), MaxPhoneYears: new(0),
			},
			want: `{"Phone":"312-555-0142","FirstNameCharOffset":0,"LastNameCharOffset":0,"MaxAddressYears":0,"MaxPhoneYears":0}`,
		},
		{
			name: "tokens keep their order and duplicates",
			req: PersonSearchRequest{
				Mode: PersonSearchModeTeaser, LastName: sqLast,
				Includes: []PersonSearchInclude{"PhoneNumbers", "Addresses", "PhoneNumbers", "Workplace"},
				FilterOptions: []PersonSearchFilter{
					PersonSearchFilterEnableExactLastNameMatch, PersonSearchFilterEnableExactLastNameMatch,
				},
			},
			want: `{"LastName":"Sample","Includes":["PhoneNumbers","Addresses","PhoneNumbers","Workplace"],` +
				`"FilterOptions":["EnableExactLastNameMatch","EnableExactLastNameMatch"]}`,
		},
		{
			name: "list entries keep their order and duplicates",
			req: PersonSearchRequest{
				Mode: PersonSearchModePerson, TahoeIDs: []string{"G-2", "G-1", "G-2"},
				Addresses: []PersonSearchRequestAddress{{County: sqLast}, {County: sqLast}},
			},
			want: `{"Addresses":[{"County":"Sample"},{"County":"Sample"}],"TahoeIds":["G-2","G-1","G-2"]}`,
		},
		{
			name: "HTML characters are not escaped",
			req:  PersonSearchRequest{Mode: PersonSearchModeTeaser, FirstName: "A<b>&c", Email: "x&y@example.com"},
			want: `{"FirstName":"A<b>&c","Email":"x&y@example.com"}`,
		},
		{
			name: "unicode is kept and quotes are escaped",
			req:  PersonSearchRequest{Mode: PersonSearchModeTeaser, LastName: `O"Ñúñez\`, AKAs: []PersonSearchRequestName{{FirstName: "Zoë"}}},
			want: `{"LastName":"O\"Ñúñez\\","Akas":[{"FirstName":"Zoë"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := tt.req.encode()
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(body), "exact body bytes")
			assert.False(t, bytes.HasSuffix(body, []byte("\n")), "no trailing newline")

			dec := json.NewDecoder(bytes.NewReader(body))
			var obj map[string]json.RawMessage
			require.NoError(t, dec.Decode(&obj), "the body is one JSON object")
			_, err = dec.Token()
			require.ErrorIs(t, err, io.EOF, "nothing follows the object")
			for _, header := range []string{"Mode", "SessionID", "SessionId"} {
				assert.NotContains(t, obj, header, "a header, never body content")
			}
			assert.NotContains(t, string(body), string(tt.req.Mode), "the mode is the search type header")
			if tt.req.SessionID != "" {
				assert.NotContains(t, string(body), tt.req.SessionID)
			}
		})
	}
}

func TestSearchRequestMode(t *testing.T) {
	for _, mode := range []PersonSearchMode{
		PersonSearchModePerson, PersonSearchModeTeaser, PersonSearchModeReversePhonePerson, PersonSearchModeReversePhonePersonTeaser,
	} {
		r := sqBase()
		r.Mode = mode
		require.NoError(t, r.validate(), mode)
	}
	tests := []struct {
		mode   PersonSearchMode
		reason string
	}{
		{"", reasonRequired},
		{"person", reasonUnsupported},
		{"PERSON", reasonUnsupported},
		{" Person", reasonUnsupported},
		{"ReversePhone", reasonUnsupported},
		{"DevAPIContactEnrich", reasonUnsupported},
		{sqMarker + "Teaser", reasonUnsupported},
	}
	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			r := sqBase()
			r.Mode = tt.mode
			err := r.validate()
			assertValidation(t, err, ErrInvalidRequest, "Mode", tt.reason)
			testutil.AssertRedacted(t, err.Error(), []string{sqMarker})
		})
	}
}

func TestSearchRequestTextRules(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		reason string
	}{
		{name: "invalid UTF-8", value: sqMarker + "\xff", reason: reasonInvalidUTF8},
		{name: "tab", value: sqMarker + "\tx", reason: reasonControlCharacter},
		{name: "NUL", value: sqMarker + "\x00x", reason: reasonControlCharacter},
		{name: "C1 next line", value: sqMarker + "\u0085x", reason: reasonControlCharacter},
		{name: "spaces only", value: "   ", reason: reasonBlank},
		{name: "ideographic space only", value: "\u3000", reason: reasonBlank},
		{name: "257 bytes", value: sqMarker + strings.Repeat("a", 257-len(sqMarker)), reason: reasonTooLong},
	}
	for _, f := range sqTextFields() {
		for _, tt := range tests {
			t.Run(f.name+"/"+tt.name, func(t *testing.T) {
				r := sqFull()
				f.set(&r, tt.value)
				err := r.validate()
				assertValidation(t, err, ErrInvalidRequest, f.name, tt.reason)
				testutil.AssertRedacted(t, err.Error(), []string{sqMarker})

				body, encErr := r.encode()
				assert.Nil(t, body)
				assert.Equal(t, err, encErr)
			})
		}
	}
	t.Run("free text is accepted", func(t *testing.T) {
		for _, f := range sqTextFields() {
			switch f.name {
			case sqDOB, "AgeRange", "ClientIP":
				continue // formatted; see TestSearchRequestFormattedFields
			}
			for _, v := range []string{strings.Repeat("a", 256), strings.Repeat("é", 128), " Zoë  Ñúñez ", "Avery\u200d"} {
				r := sqFull()
				f.set(&r, v)
				require.NoError(t, r.validate(), "%s %q", f.name, v)
			}
		}
	})
}

// TestSearchRequestFormattedFields covers the fields with rules beyond the text
// rules.
func TestSearchRequestFormattedFields(t *testing.T) {
	type row struct {
		name  string
		set   func(*PersonSearchRequest)
		valid bool
	}
	str := func(dst func(*PersonSearchRequest) *string, v string, valid bool) row {
		return row{name: v, set: func(r *PersonSearchRequest) { *dst(r) = v }, valid: valid}
	}
	num := func(dst func(*PersonSearchRequest) *int, v int, valid bool) row {
		return row{name: strconv.Itoa(v), set: func(r *PersonSearchRequest) { *dst(r) = v }, valid: valid}
	}
	ptr := func(dst func(*PersonSearchRequest) **int, v int, valid bool) row {
		return row{name: strconv.Itoa(v), set: func(r *PersonSearchRequest) { *dst(r) = new(v) }, valid: valid}
	}
	dob := func(r *PersonSearchRequest) *string { return &r.DOB }
	ageRange := func(r *PersonSearchRequest) *string { return &r.AgeRange }
	clientIP := func(r *PersonSearchRequest) *string { return &r.ClientIP }
	session := func(r *PersonSearchRequest) *string { return &r.SessionID }
	ages := func(dst func(*PersonSearchRequest) *int) []row {
		return []row{
			num(dst, 0, true), num(dst, 1, true), num(dst, 150, true),
			num(dst, -1, false), num(dst, 151, false), num(dst, math.MinInt32, false),
		}
	}
	offsets := func(dst func(*PersonSearchRequest) **int) []row {
		return []row{ptr(dst, 0, true), ptr(dst, 256, true), ptr(dst, -1, false), ptr(dst, 257, false)}
	}
	years := func(dst func(*PersonSearchRequest) **int) []row {
		return []row{ptr(dst, 0, true), ptr(dst, 150, true), ptr(dst, -1, false), ptr(dst, 151, false)}
	}
	tests := []struct {
		field, reason string
		rows          []row
	}{
		{field: sqDOB, reason: reasonInvalidFormat, rows: []row{
			str(dob, "02/29/2000", true), str(dob, "12/31/1999", true),
			str(dob, "02/30/2020", false), str(dob, "1/2/1990", false), str(dob, "1990-01-02", false), str(dob, sqMarker, false),
		}},
		{field: "Age", reason: reasonOutOfRange, rows: ages(func(r *PersonSearchRequest) *int { return &r.Age })},
		{field: "AgeRangeMinAge", reason: reasonOutOfRange, rows: ages(func(r *PersonSearchRequest) *int { return &r.AgeRangeMinAge })},
		{field: "AgeRangeMaxAge", reason: reasonOutOfRange, rows: ages(func(r *PersonSearchRequest) *int { return &r.AgeRangeMaxAge })},
		{field: "AgeRange", reason: reasonInvalidFormat, rows: []row{
			str(ageRange, "30-40", true), str(ageRange, "0-150", true), str(ageRange, "150-150", true),
			str(ageRange, "40-30", true), str(ageRange, "007-9", true),
			str(ageRange, "30", false), str(ageRange, "30-", false), str(ageRange, "-40", false), str(ageRange, "30--40", false),
			str(ageRange, "30-40-50", false), str(ageRange, "30 - 40", false), str(ageRange, "1000-1", false),
			str(ageRange, "+3-40", false), str(ageRange, "\u0663-40", false), str(ageRange, sqMarker, false),
		}},
		{field: "AgeRange", reason: reasonOutOfRange, rows: []row{
			str(ageRange, "151-160", false), str(ageRange, "30-151", false), str(ageRange, "999-999", false),
		}},
		{field: "ClientIP", reason: reasonInvalidFormat, rows: []row{
			str(clientIP, "192.0.2.10", true), str(clientIP, "0.0.0.0", true), str(clientIP, "255.255.255.255", true),
			str(clientIP, "192.0.2.010", false), str(clientIP, "192.0.2", false), str(clientIP, "192.0.2.256", false),
			str(clientIP, "2001:db8::1", false), str(clientIP, "::ffff:192.0.2.10", false), str(clientIP, "192.0.2.10:80", false),
			str(clientIP, "example.com", false), str(clientIP, sqMarker, false),
		}},
		{field: "FirstNameCharOffset", reason: reasonOutOfRange, rows: offsets(func(r *PersonSearchRequest) **int { return &r.FirstNameCharOffset })},
		{field: "LastNameCharOffset", reason: reasonOutOfRange, rows: offsets(func(r *PersonSearchRequest) **int { return &r.LastNameCharOffset })},
		{field: "MaxAddressYears", reason: reasonOutOfRange, rows: years(func(r *PersonSearchRequest) **int { return &r.MaxAddressYears })},
		{field: "MaxPhoneYears", reason: reasonOutOfRange, rows: years(func(r *PersonSearchRequest) **int { return &r.MaxPhoneYears })},
		{field: "Page", reason: reasonOutOfRange, rows: []row{
			num(func(r *PersonSearchRequest) *int { return &r.Page }, 0, true),
			num(func(r *PersonSearchRequest) *int { return &r.Page }, 1, true),
			num(func(r *PersonSearchRequest) *int { return &r.Page }, math.MaxInt32, true),
			num(func(r *PersonSearchRequest) *int { return &r.Page }, -1, false),
		}},
		{field: "ResultsPerPage", reason: reasonOutOfRange, rows: []row{
			num(func(r *PersonSearchRequest) *int { return &r.ResultsPerPage }, 0, true),
			num(func(r *PersonSearchRequest) *int { return &r.ResultsPerPage }, 1, true),
			num(func(r *PersonSearchRequest) *int { return &r.ResultsPerPage }, 1000, true),
			num(func(r *PersonSearchRequest) *int { return &r.ResultsPerPage }, 1001, false),
			num(func(r *PersonSearchRequest) *int { return &r.ResultsPerPage }, -1, false),
		}},
		{field: "SessionID", reason: reasonInvalidHeaderValue, rows: []row{
			str(session, "", true), str(session, "batch-7f3a9c-0001", true), str(session, strings.Repeat("s", 128), true),
			str(session, sqMarker+strings.Repeat("s", 129-len(sqMarker)), false), str(session, sqMarker+" x", false),
			str(session, sqMarker+"\r\nX-Injected: 1", false), str(session, sqMarker+"é", false),
		}},
	}
	for _, g := range tests {
		for _, tt := range g.rows {
			t.Run(g.field+"/"+tt.name, func(t *testing.T) {
				r := sqFull()
				tt.set(&r)
				err := r.validate()
				if tt.valid {
					require.NoError(t, err)
					return
				}
				assertValidation(t, err, ErrInvalidRequest, g.field, g.reason)
				testutil.AssertRedacted(t, err.Error(), []string{sqMarker})
			})
		}
	}
	t.Run("Page above 2^31-1", func(t *testing.T) {
		if math.MaxInt == math.MaxInt32 {
			t.Skip("int is 32 bits")
		}
		r := sqBase()
		r.Page = math.MaxInt32 + 1
		assertValidation(t, r.validate(), ErrInvalidRequest, "Page", reasonOutOfRange)
	})
}

func TestSearchRequestTokens(t *testing.T) {
	tests := []struct {
		name, token, reason string
	}{
		{name: "documented", token: "Addresses"},
		{name: "any alphanumeric token", token: "Workplace2"},
		{name: "128 bytes", token: strings.Repeat("A", 128)},
		{name: "empty", token: "", reason: reasonBlank},
		{name: "129 bytes", token: sqMarker + strings.Repeat("A", 129-len(sqMarker)), reason: reasonTooLong},
		{name: "space", token: sqMarker + " Names", reason: reasonInvalidFormat},
		{name: "spaced label", token: "Phone Numbers", reason: reasonInvalidFormat},
		{name: "punctuation", token: sqMarker + "-x", reason: reasonInvalidFormat},
		{name: "non-ASCII letter", token: sqMarker + "é", reason: reasonInvalidFormat},
		{name: "control", token: sqMarker + "\x00", reason: reasonInvalidFormat},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inc := sqBase()
			inc.Includes = []PersonSearchInclude{PersonSearchIncludePhoneNumbers, PersonSearchInclude(tt.token)}
			flt := sqBase()
			flt.FilterOptions = []PersonSearchFilter{PersonSearchFilterDisableNoDobs, PersonSearchFilter(tt.token)}
			for field, r := range map[string]PersonSearchRequest{"Includes[1]": inc, "FilterOptions[1]": flt} {
				err := r.validate()
				if tt.reason == "" {
					require.NoError(t, err)
					continue
				}
				assertValidation(t, err, ErrInvalidRequest, field, tt.reason)
				testutil.AssertRedacted(t, err.Error(), []string{sqMarker})
			}
		})
	}
}

func TestSearchRequestLists(t *testing.T) {
	type list struct {
		field string
		fill  func(*PersonSearchRequest, int)
	}
	lists := []list{
		{"AKAs", func(r *PersonSearchRequest, n int) {
			r.AKAs = make([]PersonSearchRequestName, n)
			for i := range r.AKAs {
				r.AKAs[i].LastName = sqLast
			}
		}},
		{"Addresses", func(r *PersonSearchRequest, n int) {
			r.Addresses = make([]PersonSearchRequestAddress, n)
			for i := range r.Addresses {
				r.Addresses[i].AddressLine2 = "OH"
			}
		}},
		{"Relatives", func(r *PersonSearchRequest, n int) {
			r.Relatives = make([]PersonSearchRequestName, n)
			for i := range r.Relatives {
				r.Relatives[i].FirstName = "Avery"
			}
		}},
		{"TahoeIDs", func(r *PersonSearchRequest, n int) {
			r.TahoeIDs = make([]string, n)
			for i := range r.TahoeIDs {
				r.TahoeIDs[i] = "G-" + strconv.Itoa(i)
			}
		}},
		{"Includes", func(r *PersonSearchRequest, n int) {
			r.Includes = make([]PersonSearchInclude, n)
			for i := range r.Includes {
				r.Includes[i] = PersonSearchIncludeAddresses
			}
		}},
		{"FilterOptions", func(r *PersonSearchRequest, n int) {
			r.FilterOptions = make([]PersonSearchFilter, n)
			for i := range r.FilterOptions {
				r.FilterOptions[i] = PersonSearchFilterDisableNoDobs
			}
		}},
	}
	for _, l := range lists {
		t.Run(l.field, func(t *testing.T) {
			r := sqBase()
			l.fill(&r, maxSearchListEntries)
			require.NoError(t, r.validate(), "32 entries")
			l.fill(&r, maxSearchListEntries+1)
			assertValidation(t, r.validate(), ErrInvalidRequest, l.field, reasonTooMany)
		})
	}
	t.Run("blank entries", func(t *testing.T) {
		tests := []struct {
			field string
			set   func(*PersonSearchRequest)
		}{
			{"AKAs[1]", func(r *PersonSearchRequest) { r.AKAs[1] = PersonSearchRequestName{} }},
			{"Addresses[0]", func(r *PersonSearchRequest) { r.Addresses[0] = PersonSearchRequestAddress{} }},
			{"Relatives[1]", func(r *PersonSearchRequest) { r.Relatives[1] = PersonSearchRequestName{} }},
			{"TahoeIDs[0]", func(r *PersonSearchRequest) { r.TahoeIDs[0] = "" }},
			{"Includes[0]", func(r *PersonSearchRequest) { r.Includes[0] = "" }},
			{"FilterOptions[1]", func(r *PersonSearchRequest) { r.FilterOptions[1] = "" }},
		}
		for _, tt := range tests {
			r := sqFull()
			tt.set(&r)
			assertValidation(t, r.validate(), ErrInvalidRequest, tt.field, reasonBlank)
		}
	})
}

func TestSearchRequestCriteria(t *testing.T) {
	criteria := map[string]func(*PersonSearchRequest){
		"first name":   func(r *PersonSearchRequest) { r.FirstName = "Riley" },
		"middle name":  func(r *PersonSearchRequest) { r.MiddleName = "Q" },
		"last name":    func(r *PersonSearchRequest) { r.LastName = sqLast },
		"AKA":          func(r *PersonSearchRequest) { r.AKAs = []PersonSearchRequestName{{LastName: sqLast}} },
		"DOB":          func(r *PersonSearchRequest) { r.DOB = "06/01/1985" },
		"age":          func(r *PersonSearchRequest) { r.Age = 41 },
		"minimum age":  func(r *PersonSearchRequest) { r.AgeRangeMinAge = 30 },
		"maximum age":  func(r *PersonSearchRequest) { r.AgeRangeMaxAge = 50 },
		"age range":    func(r *PersonSearchRequest) { r.AgeRange = "30-50" },
		"address":      func(r *PersonSearchRequest) { r.Addresses = []PersonSearchRequestAddress{{County: sqLast}} },
		"email":        func(r *PersonSearchRequest) { r.Email = "riley.sample@example.com" },
		"phone":        func(r *PersonSearchRequest) { r.Phone = "(312) 555-0142" },
		"client IP":    func(r *PersonSearchRequest) { r.ClientIP = "192.0.2.10" },
		"relative":     func(r *PersonSearchRequest) { r.Relatives = []PersonSearchRequestName{{FirstName: "Avery"}} },
		"Tahoe ID":     func(r *PersonSearchRequest) { r.TahoeIDs = []string{"G-1000000000000000042"} },
		"reverse name": func(r *PersonSearchRequest) { r.Mode, r.LastName = PersonSearchModeReversePhonePersonTeaser, sqLast },
	}
	for name, set := range criteria {
		t.Run(name, func(t *testing.T) {
			r := PersonSearchRequest{Mode: PersonSearchModePerson}
			set(&r)
			require.NoError(t, r.validate(), "one criterion is enough, whatever the mode")
		})
	}
	refinements := PersonSearchRequest{
		Mode: PersonSearchModePerson, FirstNameCharOffset: new(1), LastNameCharOffset: new(1), DOBFormat: "mm-dd-yyyy",
		MaxAddressYears: new(5), MaxPhoneYears: new(5), Includes: []PersonSearchInclude{PersonSearchIncludeAddresses},
		FilterOptions: []PersonSearchFilter{PersonSearchFilterDisableSmartSearch}, Page: 1, ResultsPerPage: 10,
		SessionID: "s-1",
	}
	for name, r := range map[string]PersonSearchRequest{
		"empty":                         {Mode: PersonSearchModePerson},
		"refinements are not criteria":  refinements,
		"reverse phone without a phone": {Mode: PersonSearchModeReversePhonePerson},
	} {
		t.Run(name, func(t *testing.T) {
			assertValidation(t, r.validate(), ErrInvalidRequest, "request", reasonNoCriteria)
		})
	}
}

func TestSearchRequestFirstFailureWins(t *testing.T) {
	tests := []struct {
		name   string
		req    PersonSearchRequest
		field  string
		reason string
	}{
		{
			name: "mode before fields", req: PersonSearchRequest{FirstName: "\x00"},
			field: "Mode", reason: reasonRequired,
		},
		{
			name: "first name before AKAs", req: PersonSearchRequest{
				Mode: PersonSearchModeTeaser, FirstName: "\x00", AKAs: []PersonSearchRequestName{{}},
			},
			field: "FirstName", reason: reasonControlCharacter,
		},
		{
			name: "an entry's fields in order", req: PersonSearchRequest{
				Mode: PersonSearchModeTeaser, AKAs: []PersonSearchRequestName{{FirstName: "x", MiddleName: " ", LastName: "\x00"}},
			},
			field: "AKAs[0].MiddleName", reason: reasonBlank,
		},
		{
			name: "entries in order", req: PersonSearchRequest{
				Mode: PersonSearchModeTeaser, Addresses: []PersonSearchRequestAddress{{County: "x"}, {}, {County: "\x00"}},
			},
			field: "Addresses[1]", reason: reasonBlank,
		},
		{
			name: "too many before an entry", req: PersonSearchRequest{
				Mode: PersonSearchModeTeaser, Relatives: make([]PersonSearchRequestName, maxSearchListEntries+1),
			},
			field: "Relatives", reason: reasonTooMany,
		},
		{
			name: "DOB before age", req: PersonSearchRequest{Mode: PersonSearchModeTeaser, DOB: "1/2/1990", Age: -1},
			field: sqDOB, reason: reasonInvalidFormat,
		},
		{
			name: "ages before the range", req: PersonSearchRequest{Mode: PersonSearchModeTeaser, AgeRangeMaxAge: 151, AgeRange: "x"},
			field: "AgeRangeMaxAge", reason: reasonOutOfRange,
		},
		{
			name: "addresses before contact", req: PersonSearchRequest{
				Mode: PersonSearchModeTeaser, Addresses: []PersonSearchRequestAddress{{}}, Email: "\x00",
			},
			field: "Addresses[0]", reason: reasonBlank,
		},
		{
			name: "phone before client IP", req: PersonSearchRequest{Mode: PersonSearchModeTeaser, Phone: " ", ClientIP: "x"},
			field: "Phone", reason: reasonBlank,
		},
		{
			name: "client IP before relatives", req: PersonSearchRequest{
				Mode: PersonSearchModeTeaser, ClientIP: "x", Relatives: []PersonSearchRequestName{{}},
			},
			field: "ClientIP", reason: reasonInvalidFormat,
		},
		{
			name: "Tahoe IDs before offsets", req: PersonSearchRequest{
				Mode: PersonSearchModeTeaser, TahoeIDs: []string{""}, FirstNameCharOffset: new(-1),
			},
			field: "TahoeIDs[0]", reason: reasonBlank,
		},
		{
			name: "tokens before the page", req: PersonSearchRequest{
				Mode: PersonSearchModeTeaser, LastName: "x", FilterOptions: []PersonSearchFilter{"a b"}, Page: -1,
			},
			field: "FilterOptions[0]", reason: reasonInvalidFormat,
		},
		{
			name: "session ID before criteria", req: PersonSearchRequest{Mode: PersonSearchModeTeaser, SessionID: " "},
			field: "SessionID", reason: reasonInvalidHeaderValue,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidation(t, tt.req.validate(), ErrInvalidRequest, tt.field, tt.reason)
		})
	}
}

// TestSearchRequestUnchanged proves that encoding never modifies the request,
// its slices or the values its pointers hold.
func TestSearchRequestUnchanged(t *testing.T) {
	req := sqFull()
	before := sqFull()
	aka0, addr0, tahoe0, inc0 := &req.AKAs[0], &req.Addresses[0], &req.TahoeIDs[0], &req.Includes[0]
	_, err := req.encode()
	require.NoError(t, err)
	assert.True(t, reflect.DeepEqual(before, req), "the request and everything it points to are unchanged")
	assert.Same(t, aka0, &req.AKAs[0])
	assert.Same(t, addr0, &req.Addresses[0])
	assert.Same(t, tahoe0, &req.TahoeIDs[0])
	assert.Same(t, inc0, &req.Includes[0])

	invalid := sqFull()
	invalid.Addresses[1].County = "\x00"
	snapshot := sqFull()
	snapshot.Addresses[1].County = "\x00"
	_, err = invalid.encode()
	require.Error(t, err)
	assert.True(t, reflect.DeepEqual(snapshot, invalid))
}
