package enformion

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// SDK safety bounds. The provider documents no length limits; these are this
// SDK's own.
const (
	// maxTextBytes bounds every request text field.
	maxTextBytes = 256

	// maxIDBytes bounds SessionID, which is sent as a header, and the
	// provider's requestId kept on results and errors.
	maxIDBytes = 128

	// maxAge bounds Age.
	maxAge = 150

	// dobLayout is the documented "mm/dd/yyyy" date of birth format.
	dobLayout = "01/02/2006"
)

// Validation reasons, reported in ValidationError.Reason.
const (
	reasonInvalidUTF8            = "invalid_utf8"
	reasonControlCharacter       = "control_character"
	reasonBlank                  = "blank"
	reasonTooLong                = "too_long"
	reasonInvalidFormat          = "invalid_format"
	reasonOutOfRange             = "out_of_range"
	reasonInvalidHeaderValue     = "invalid_header_value"
	reasonInsufficientCategories = "insufficient_categories"
	reasonNil                    = "nil"
	reasonRequired               = "required"
	reasonInvalidURL             = "invalid_url"
	reasonNotInitialized         = "not_initialized"
	reasonUnsupported            = "unsupported"
	reasonTooMany                = "too_many"
	reasonNoCriteria             = "no_criteria"
)

// EnrichContactRequest describes one subject to enrich.
//
// The provider requires at least two of four search categories, and this SDK
// counts them as follows:
//   - Name: FirstName and LastName both set (MiddleName alone, or a first or
//     last name alone, is sent but does not count)
//   - Phone
//   - Email
//   - Address: AddressLine1 or AddressLine2 set
//
// DOB and Age are sent when set but are not categories.
//
// Values are sent verbatim: the SDK never trims, normalizes or rewrites them.
// Every non-empty text field must be valid UTF-8 of at most 256 bytes, with no
// control characters (tabs included) and not only whitespace; these bounds are
// this SDK's own, as the provider documents none. The request is passed by
// value, and the SDK neither keeps nor modifies it.
//
// The request holds personal data: see the package documentation.
type EnrichContactRequest struct {
	// FirstName, MiddleName and LastName are sent as FirstName, MiddleName and
	// LastName. MiddleName may be a middle initial.
	FirstName  string
	MiddleName string
	LastName   string

	// DOB is the date of birth as mm/dd/yyyy, zero-padded, and must be a real
	// calendar date. It is sent as Dob.
	DOB string

	// Age is sent as Age when it is from 1 to 150; 0 means not sent.
	Age int

	// AddressLine1 is the house number and street name, or a PO Box.
	// AddressLine2 is the city and state, the state, or the ZIP code. They are
	// sent as Address.addressLine1 and Address.addressLine2.
	AddressLine1 string
	AddressLine2 string

	// Phone and Email are sent verbatim. The phone format is not enforced: the
	// provider's own examples disagree with its documented formats.
	Phone string
	Email string

	// SessionID is optional and sent as the galaxy-client-session-id header,
	// which the provider documents as a session ID for logging. It must be empty
	// or 1 to 128 bytes of visible ASCII (no spaces). Use an opaque value, never
	// personal data.
	SessionID string
}

// wireRequest is the request body. Empty values are omitted, as the provider's
// own examples do.
type wireRequest struct {
	FirstName  string       `json:"FirstName,omitempty"`
	MiddleName string       `json:"MiddleName,omitempty"`
	LastName   string       `json:"LastName,omitempty"`
	Dob        string       `json:"Dob,omitempty"`
	Age        int          `json:"Age,omitempty"`
	Address    *wireAddress `json:"Address,omitempty"`
	Phone      string       `json:"Phone,omitempty"`
	Email      string       `json:"Email,omitempty"`
}

// wireAddress is the request's Address object. Every valid JSON example in the
// provider's documentation spells its keys addressLine1 and addressLine2,
// although its properties table says AddressLine1 and AddressLine2.
type wireAddress struct {
	AddressLine1 string `json:"addressLine1,omitempty"`
	AddressLine2 string `json:"addressLine2,omitempty"`
}

// validate checks every field in struct order and then the search categories,
// returning the first failure as a *ValidationError matching ErrInvalidRequest.
func (r *EnrichContactRequest) validate() error {
	texts := [...]struct{ field, value string }{
		{"FirstName", r.FirstName},
		{"MiddleName", r.MiddleName},
		{"LastName", r.LastName},
	}
	for _, t := range texts {
		if err := checkText(t.field, t.value); err != nil {
			return err
		}
	}
	if err := checkDOB(r.DOB); err != nil {
		return err
	}
	if r.Age < 0 || r.Age > maxAge {
		return invalidRequest("Age", reasonOutOfRange)
	}
	rest := [...]struct{ field, value string }{
		{"AddressLine1", r.AddressLine1},
		{"AddressLine2", r.AddressLine2},
		{"Phone", r.Phone},
		{"Email", r.Email},
	}
	for _, t := range rest {
		if err := checkText(t.field, t.value); err != nil {
			return err
		}
	}
	if r.SessionID != "" && !validID(r.SessionID) {
		return invalidRequest("SessionID", reasonInvalidHeaderValue)
	}
	if r.categories() < 2 {
		return invalidRequest("request", reasonInsufficientCategories)
	}
	return nil
}

// categories counts the documented search categories the request fills.
func (r *EnrichContactRequest) categories() int {
	n := 0
	for _, filled := range [...]bool{
		r.FirstName != "" && r.LastName != "",
		r.Phone != "",
		r.Email != "",
		r.AddressLine1 != "" || r.AddressLine2 != "",
	} {
		if filled {
			n++
		}
	}
	return n
}

// encode validates the request and returns its JSON body: exactly one object,
// with no trailing newline and with <, > and & unescaped.
func (r *EnrichContactRequest) encode() ([]byte, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	w := wireRequest{
		FirstName:  r.FirstName,
		MiddleName: r.MiddleName,
		LastName:   r.LastName,
		Dob:        r.DOB,
		Age:        r.Age,
		Phone:      r.Phone,
		Email:      r.Email,
	}
	if r.AddressLine1 != "" || r.AddressLine2 != "" {
		w.Address = &wireAddress{AddressLine1: r.AddressLine1, AddressLine2: r.AddressLine2}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(&w); err != nil {
		// Validated strings and an int always encode; this is unreachable.
		return nil, invalidRequest("request", reasonInvalidFormat)
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte{'\n'}), nil
}

// checkText applies the rules every request text field shares. An empty value
// is valid: it is simply not sent.
func checkText(field, s string) error {
	if reason := textReason(s); reason != "" {
		return invalidRequest(field, reason)
	}
	return nil
}

// textReason returns the reason s breaks the rules every request text field
// shares, or "" when it keeps them. It never allocates, so a caller can build
// an indexed field name only once a value fails.
func textReason(s string) string {
	switch {
	case s == "":
		return ""
	case len(s) > maxTextBytes:
		return reasonTooLong
	case !utf8.ValidString(s):
		return reasonInvalidUTF8
	case strings.ContainsFunc(s, unicode.IsControl):
		return reasonControlCharacter
	case strings.TrimSpace(s) == "":
		return reasonBlank
	}
	return ""
}

// checkDOB applies the text rules and then the date format to a DOB field.
func checkDOB(s string) error {
	if err := checkText("DOB", s); err != nil {
		return err
	}
	if s != "" && !validDOB(s) {
		return invalidRequest("DOB", reasonInvalidFormat)
	}
	return nil
}

// validDOB reports whether s is a zero-padded mm/dd/yyyy calendar date.
func validDOB(s string) bool {
	t, err := time.Parse(dobLayout, s)
	return err == nil && t.Format(dobLayout) == s
}

// validID reports whether s is 1 to maxIDBytes bytes, each visible ASCII (0x21
// to 0x7E): no spaces, controls or non-ASCII bytes.
func validID(s string) bool {
	return s != "" && len(s) <= maxIDBytes && asciiRange(s, 0x21, 0x7e)
}

// asciiRange reports whether every byte of s is from lo to hi.
func asciiRange(s string, lo, hi byte) bool {
	for i := range len(s) {
		if c := s[i]; c < lo || c > hi {
			return false
		}
	}
	return true
}
