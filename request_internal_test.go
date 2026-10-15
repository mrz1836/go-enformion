package enformion

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-enformion/internal/testutil"
)

// rqMarker prefixes rejected values, so a test can prove none reaches an error.
const rqMarker = "RQMARKER"

// rqFirst is the synthetic first name most requests use.
const rqFirst = "Avery"

// rqBase returns a valid request with two categories, name and email.
func rqBase() EnrichContactRequest {
	return EnrichContactRequest{FirstName: rqFirst, LastName: "Example", Email: "avery.example@example.com"}
}

// rqFull returns a valid request that fills every field.
func rqFull() EnrichContactRequest {
	return EnrichContactRequest{
		FirstName: rqFirst, MiddleName: "Q", LastName: "Example", DOB: "06/01/1985", Age: 41,
		AddressLine1: "100 Sample Way", AddressLine2: "Exampleton, OH",
		Phone: "(312) 555-0142", Email: "avery.example@example.com", SessionID: "batch-7f3a9c-0001",
	}
}

// rqFields returns setters for every text field, in struct order, applied to
// rqFull, which fills all four categories, so that clearing or breaking any one
// field still leaves at least two.
func rqFields() []struct {
	name string
	set  func(*EnrichContactRequest, string)
} {
	return []struct {
		name string
		set  func(*EnrichContactRequest, string)
	}{
		{"FirstName", func(r *EnrichContactRequest, v string) { r.FirstName = v }},
		{"MiddleName", func(r *EnrichContactRequest, v string) { r.MiddleName = v }},
		{"LastName", func(r *EnrichContactRequest, v string) { r.LastName = v }},
		{"DOB", func(r *EnrichContactRequest, v string) { r.DOB = v }},
		{"AddressLine1", func(r *EnrichContactRequest, v string) { r.AddressLine1 = v }},
		{"AddressLine2", func(r *EnrichContactRequest, v string) { r.AddressLine2 = v }},
		{"Phone", func(r *EnrichContactRequest, v string) { r.Phone = v }},
		{"Email", func(r *EnrichContactRequest, v string) { r.Email = v }},
	}
}

func TestRequestTextRules(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		reason string
	}{
		{name: "invalid UTF-8", value: rqMarker + "\xff", reason: reasonInvalidUTF8},
		{name: "truncated UTF-8", value: rqMarker + "\xc3", reason: reasonInvalidUTF8},
		{name: "tab", value: rqMarker + "\tx", reason: reasonControlCharacter},
		{name: "NUL", value: rqMarker + "\x00x", reason: reasonControlCharacter},
		{name: "line feed", value: rqMarker + "\nx", reason: reasonControlCharacter},
		{name: "DEL", value: rqMarker + "\x7fx", reason: reasonControlCharacter},
		{name: "C1 next line", value: rqMarker + "\u0085x", reason: reasonControlCharacter},
		{name: "spaces only", value: "   ", reason: reasonBlank},
		{name: "ideographic space only", value: "\u3000", reason: reasonBlank},
		{name: "257 bytes", value: rqMarker + strings.Repeat("a", 257-len(rqMarker)), reason: reasonTooLong},
		{name: "too long beats invalid UTF-8", value: strings.Repeat("\xff", 300), reason: reasonTooLong},
	}
	for _, f := range rqFields() {
		for _, tt := range tests {
			t.Run(f.name+"/"+tt.name, func(t *testing.T) {
				r := rqFull()
				f.set(&r, tt.value)
				err := r.validate()
				assertValidation(t, err, ErrInvalidRequest, f.name, tt.reason)
				testutil.AssertRedacted(t, err.Error(), []string{rqMarker})

				body, encErr := r.encode()
				assert.Nil(t, body)
				assert.Equal(t, err, encErr)
			})
		}
	}
}

func TestRequestTextAccepted(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "256 bytes", value: strings.Repeat("a", 256)},
		{name: "256 bytes of multi-byte runes", value: strings.Repeat("é", 128)},
		{name: "unicode letters", value: "Zoë Ñúñez"},
		{name: "inner and edge spaces are sent verbatim", value: " Avery  Q "},
		{name: "format characters are not controls", value: "Avery\u200d"},
	}
	for _, f := range rqFields() {
		if f.name == "DOB" {
			continue // DOB also has a format; see TestRequestFormattedFields
		}
		for _, tt := range tests {
			t.Run(f.name+"/"+tt.name, func(t *testing.T) {
				r := rqFull()
				f.set(&r, tt.value)
				require.NoError(t, r.validate())
			})
		}
	}
	t.Run("DOB/257 bytes is too long", func(t *testing.T) {
		r := rqBase()
		r.DOB = strings.Repeat("1", 257)
		assertValidation(t, r.validate(), ErrInvalidRequest, "DOB", reasonTooLong)
	})
}

// TestRequestFormattedFields covers the fields with rules beyond the text
// rules: DOB, Age and SessionID.
func TestRequestFormattedFields(t *testing.T) {
	type row struct {
		name  string
		set   func(*EnrichContactRequest)
		valid bool
	}
	dob := func(v string, valid bool) row {
		return row{name: v, set: func(r *EnrichContactRequest) { r.DOB = v }, valid: valid}
	}
	age := func(v int, valid bool) row {
		return row{name: strconv.Itoa(v), set: func(r *EnrichContactRequest) { r.Age = v }, valid: valid}
	}
	sid := func(name, v string, valid bool) row {
		return row{name: name, set: func(r *EnrichContactRequest) { r.SessionID = v }, valid: valid}
	}
	tests := []struct {
		field, reason string
		rows          []row
	}{
		{field: "DOB", reason: reasonInvalidFormat, rows: []row{
			dob("01/02/1990", true), dob("12/31/1999", true), dob("02/29/2000", true), dob("06/01/1985", true),
			dob("02/29/1900", false), dob("02/30/2020", false), dob("13/01/2020", false), dob("00/10/2000", false),
			dob("01/00/2000", false), dob("1/2/1990", false), dob("2/3/1990", false), dob("01/2/1990", false),
			dob("01/02/90", false), dob("01/02/1990x", false), dob(" 01/02/1990", false), dob("01-02-1990", false),
			dob("1990-01-02", false), dob("01/02/1990 00:00", false), dob("Jan 2 1990", false),
		}},
		{field: "Age", reason: reasonOutOfRange, rows: []row{
			age(0, true), age(1, true), age(150, true),
			age(-1, false), age(151, false), age(-1<<31, false), age(1<<40, false),
		}},
		{field: "SessionID", reason: reasonInvalidHeaderValue, rows: []row{
			sid("empty", "", true),
			sid("opaque", "batch-7f3a9c-0001", true),
			sid("one byte", "x", true),
			sid("128 bytes", strings.Repeat("s", 128), true),
			sid("every visible ASCII byte", "!\"#$%&'()*+,-./09:;<=>?@AZ[\\]^_`az{|}~", true),
			sid("129 bytes", rqMarker+strings.Repeat("s", 129-len(rqMarker)), false),
			sid("space", rqMarker+" x", false),
			sid("non-ASCII", rqMarker+"séance", false),
			sid("NUL", rqMarker+"\x00", false),
			sid("tab", rqMarker+"\tx", false),
			sid("CR LF", rqMarker+"\r\nX-Injected: 1", false),
			sid("DEL", rqMarker+"\x7f", false),
		}},
	}
	for _, g := range tests {
		for _, tt := range g.rows {
			t.Run(g.field+"/"+tt.name, func(t *testing.T) {
				r := rqBase()
				tt.set(&r)
				err := r.validate()
				if tt.valid {
					require.NoError(t, err)
					return
				}
				assertValidation(t, err, ErrInvalidRequest, g.field, g.reason)
				testutil.AssertRedacted(t, err.Error(), []string{rqMarker})
			})
		}
	}
}

func TestRequestCategories(t *testing.T) {
	tests := []struct {
		name string
		req  EnrichContactRequest
		want int
	}{
		{name: "empty", req: EnrichContactRequest{}, want: 0},
		{name: "full name only", req: EnrichContactRequest{FirstName: rqFirst, LastName: "Example"}, want: 1},
		{name: "full name and phone", req: EnrichContactRequest{FirstName: rqFirst, LastName: "Example", Phone: "312-555-0142"}, want: 2},
		{name: "first name alone does not count", req: EnrichContactRequest{FirstName: rqFirst, Phone: "312-555-0142"}, want: 1},
		{name: "last name alone does not count", req: EnrichContactRequest{LastName: "Example", Email: "a@example.com"}, want: 1},
		{name: "middle name alone does not count", req: EnrichContactRequest{MiddleName: "Q", Phone: "312-555-0142"}, want: 1},
		{name: "first, middle and last are one category", req: EnrichContactRequest{FirstName: rqFirst, MiddleName: "Q", LastName: "Example"}, want: 1},
		{name: "phone and email", req: EnrichContactRequest{Phone: "312-555-0142", Email: "a@example.com"}, want: 2},
		{name: "address line 1 and phone", req: EnrichContactRequest{AddressLine1: "100 Sample Way", Phone: "312-555-0142"}, want: 2},
		{name: "address line 2 and email", req: EnrichContactRequest{AddressLine2: "Exampleton, OH", Email: "a@example.com"}, want: 2},
		{name: "both address lines are one category", req: EnrichContactRequest{AddressLine1: "100 Sample Way", AddressLine2: "Exampleton, OH"}, want: 1},
		{name: "DOB and age are not categories", req: EnrichContactRequest{DOB: "06/01/1985", Age: 41, Phone: "312-555-0142"}, want: 1},
		{name: "name with DOB and age", req: EnrichContactRequest{FirstName: rqFirst, LastName: "Example", DOB: "06/01/1985", Age: 41}, want: 1},
		{name: "session ID is not a category", req: EnrichContactRequest{Email: "a@example.com", SessionID: "s-1"}, want: 1},
		{name: "all four", req: EnrichContactRequest{FirstName: rqFirst, LastName: "Example", Phone: "312-555-0142", Email: "a@example.com", AddressLine2: "45000"}, want: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.req.categories())
			err := tt.req.validate()
			if tt.want >= 2 {
				require.NoError(t, err)
				return
			}
			assertValidation(t, err, ErrInvalidRequest, "request", reasonInsufficientCategories)
		})
	}
}

func TestRequestFirstFailureWins(t *testing.T) {
	tests := []struct {
		name   string
		req    EnrichContactRequest
		field  string
		reason string
	}{
		{
			name: "first name before last name", req: EnrichContactRequest{FirstName: "\x00", LastName: "\x00", Phone: "x", Email: "y"},
			field: "FirstName", reason: reasonControlCharacter,
		},
		{
			name: "last name before phone", req: EnrichContactRequest{FirstName: rqFirst, LastName: "\xff", Phone: "\x00", Email: "y"},
			field: "LastName", reason: reasonInvalidUTF8,
		},
		{
			name: "DOB format before age", req: EnrichContactRequest{Phone: "x", Email: "y", DOB: "1/2/1990", Age: -1},
			field: "DOB", reason: reasonInvalidFormat,
		},
		{
			name: "age before address", req: EnrichContactRequest{Phone: "x", Email: "y", Age: 151, AddressLine1: "\x00"},
			field: "Age", reason: reasonOutOfRange,
		},
		{
			name: "address line 1 before line 2", req: EnrichContactRequest{Phone: "x", Email: "y", AddressLine1: " ", AddressLine2: " "},
			field: "AddressLine1", reason: reasonBlank,
		},
		{
			name: "email before session ID", req: EnrichContactRequest{Phone: "x", Email: "\x00", SessionID: " "},
			field: "Email", reason: reasonControlCharacter,
		},
		{
			name: "session ID before categories", req: EnrichContactRequest{Phone: "x", SessionID: " "},
			field: "SessionID", reason: reasonInvalidHeaderValue,
		},
		{
			name: "fields before categories", req: EnrichContactRequest{MiddleName: "\x00"},
			field: "MiddleName", reason: reasonControlCharacter,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidation(t, tt.req.validate(), ErrInvalidRequest, tt.field, tt.reason)
		})
	}
}

func TestRequestWireEncoding(t *testing.T) {
	tests := []struct {
		name string
		req  EnrichContactRequest
		want string
	}{
		{
			name: "name and email",
			req:  rqBase(),
			want: `{"FirstName":"` + rqFirst + `","LastName":"Example","Email":"avery.example@example.com"}`,
		},
		{
			name: "every field",
			req:  rqFull(),
			want: `{"FirstName":"` + rqFirst + `","MiddleName":"Q","LastName":"Example","Dob":"06/01/1985","Age":41,` +
				`"Address":{"addressLine1":"100 Sample Way","addressLine2":"Exampleton, OH"},` +
				`"Phone":"(312) 555-0142","Email":"avery.example@example.com"}`,
		},
		{
			name: "address line 2 only",
			req:  EnrichContactRequest{AddressLine2: "Exampleton, OH 45000", Phone: "312-555-0199"},
			want: `{"Address":{"addressLine2":"Exampleton, OH 45000"},"Phone":"312-555-0199"}`,
		},
		{
			name: "address line 1 only",
			req:  EnrichContactRequest{AddressLine1: "PO Box 100", Email: "a@example.com"},
			want: `{"Address":{"addressLine1":"PO Box 100"},"Email":"a@example.com"}`,
		},
		{
			name: "age zero and empty strings are omitted",
			req:  EnrichContactRequest{FirstName: rqFirst, MiddleName: "", LastName: "Example", Age: 0, Phone: "312-555-0142"},
			want: `{"FirstName":"` + rqFirst + `","LastName":"Example","Phone":"312-555-0142"}`,
		},
		{
			name: "HTML characters are not escaped",
			req:  EnrichContactRequest{FirstName: "A<b>&c", LastName: "Ex>ample", Email: "x&y@example.com"},
			want: `{"FirstName":"A<b>&c","LastName":"Ex>ample","Email":"x&y@example.com"}`,
		},
		{
			name: "unicode is kept and quotes are escaped",
			req:  EnrichContactRequest{FirstName: "Zoë", LastName: `O"Ñúñez\`, Phone: "312-555-0142"},
			want: `{"FirstName":"Zoë","LastName":"O\"Ñúñez\\","Phone":"312-555-0142"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := tt.req.encode()
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(body))
			assert.False(t, bytes.HasSuffix(body, []byte("\n")), "no trailing newline")
			assert.True(t, json.Valid(body))

			dec := json.NewDecoder(bytes.NewReader(body))
			var obj map[string]json.RawMessage
			require.NoError(t, dec.Decode(&obj), "the body is one JSON object")
			_, err = dec.Token()
			require.ErrorIs(t, err, io.EOF, "nothing follows the object")
			if tt.req.SessionID != "" {
				assert.NotContains(t, string(body), tt.req.SessionID, "the session ID is a header, never body content")
			}
		})
	}
}
