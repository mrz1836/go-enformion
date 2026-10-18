package enformion

import (
	"bytes"
	"encoding/json"
	"math"
	"net/netip"
	"strconv"
	"strings"
)

// Person Search SDK bounds. The provider documents none; these are this SDK's
// own.
const (
	// maxSearchListEntries bounds each request list: AKAs, Addresses,
	// Relatives, TahoeIDs, Includes and FilterOptions.
	maxSearchListEntries = 32

	// maxTokenBytes bounds each Includes and FilterOptions token.
	maxTokenBytes = 128

	// maxCharOffset bounds FirstNameCharOffset and LastNameCharOffset: no edit
	// distance is larger than the longest text field.
	maxCharOffset = maxTextBytes

	// maxYears bounds MaxAddressYears and MaxPhoneYears.
	maxYears = 150

	// maxResultsPerPage bounds ResultsPerPage: a page of more persons than a
	// response list may hold could never decode.
	maxResultsPerPage = maxListEntries
)

// PersonSearchMode is the Person Search search type, sent as the
// galaxy-search-type header. The provider documents four, quoted below; it
// documents no response difference between them beyond that text.
type PersonSearchMode string

// Person Search modes.
const (
	// PersonSearchModePerson is documented "for more detailed information
	// pertaining to the subjects", mainly for "logged in or paying" users.
	PersonSearchModePerson PersonSearchMode = "Person"

	// PersonSearchModeTeaser is documented for "logged out" users: "Most of the
	// data elements in a teaser search are masked and/or limited by the
	// quantity returned."
	PersonSearchModeTeaser PersonSearchMode = "Teaser"

	// PersonSearchModeReversePhonePerson is documented with the same text as
	// PersonSearchModePerson. It is a mode of Person Search, not the separate
	// Reverse Phone Search API.
	PersonSearchModeReversePhonePerson PersonSearchMode = "ReversePhonePerson"

	// PersonSearchModeReversePhonePersonTeaser is documented with the same text
	// as PersonSearchModeTeaser.
	PersonSearchModeReversePhonePersonTeaser PersonSearchMode = "ReversePhonePersonTeaser"
)

// PersonSearchInclude is a token of the request's Includes list, which asks
// for parts of each person. The provider enables includes per account. Its
// includes page lists labels, such as "Phone Numbers", rather than tokens:
// only Addresses and PhoneNumbers appear as tokens in its documentation. Any
// other token of 1 to 128 ASCII letters and digits may be sent as
// PersonSearchInclude("...").
type PersonSearchInclude string

// Person Search include tokens.
const (
	// PersonSearchIncludeAddresses is a documented token.
	PersonSearchIncludeAddresses PersonSearchInclude = "Addresses"

	// PersonSearchIncludePhoneNumbers is a documented token.
	PersonSearchIncludePhoneNumbers PersonSearchInclude = "PhoneNumbers"

	// PersonSearchIncludeEmailAddresses is not in the documentation; it has
	// been observed to be accepted.
	PersonSearchIncludeEmailAddresses PersonSearchInclude = "EmailAddresses"

	// PersonSearchIncludeDatesOfBirth is not in the documentation; it has been
	// observed to be accepted.
	PersonSearchIncludeDatesOfBirth PersonSearchInclude = "DatesOfBirth"

	// PersonSearchIncludeRelatives is not in the documentation; it has been
	// observed to be accepted.
	PersonSearchIncludeRelatives PersonSearchInclude = "Relatives"

	// PersonSearchIncludeRelativesSummary is not in the documentation; it has
	// been observed to be accepted.
	PersonSearchIncludeRelativesSummary PersonSearchInclude = "RelativesSummary"

	// PersonSearchIncludeAssociates is not in the documentation; it has been
	// observed to be accepted. It returns each associate as a full person,
	// which makes a page much larger.
	PersonSearchIncludeAssociates PersonSearchInclude = "Associates"
)

// PersonSearchFilter is a token of the request's FilterOptions list. The
// provider enables filters per account. Each constant's comment quotes the
// provider's description, which for several filters contradicts the token's
// name: the SDK sends the token verbatim and does not interpret it. Any other
// token of 1 to 128 ASCII letters and digits may be sent as
// PersonSearchFilter("...").
type PersonSearchFilter string

// Person Search filter tokens, each documented.
const (
	// PersonSearchFilterDisableFirstNameSynonymMatching is documented as "This
	// filter enables exact name match on first name."
	PersonSearchFilterDisableFirstNameSynonymMatching PersonSearchFilter = "DisableFirstNameSynonymMatching"

	// PersonSearchFilterDisableSmartSearch is documented as "If smart search is
	// enabled when we get a not hit we begin dropping off low.* value criteria
	// in an attempt to get a hit."
	PersonSearchFilterDisableSmartSearch PersonSearchFilter = "DisableSmartSearch"

	// PersonSearchFilterIncludeEmptyFirstNameResults is documented as "This
	// filter will remove results where a first name is not present."
	PersonSearchFilterIncludeEmptyFirstNameResults PersonSearchFilter = "IncludeEmptyFirstNameResults"

	// PersonSearchFilterIncludeSevenDigitPhoneNumbers is documented as "This
	// filter will include phone numbers with no area code."
	PersonSearchFilterIncludeSevenDigitPhoneNumbers PersonSearchFilter = "IncludeSevenDigitPhoneNumbers"

	// PersonSearchFilterIncludeLowQualityAddresses is documented as "This filter
	// will remove any addresses that do not come from a validated source".
	PersonSearchFilterIncludeLowQualityAddresses PersonSearchFilter = "IncludeLowQualityAddresses"

	// PersonSearchFilterIncludeLatestRecordsOnly is documented as "When enabled
	// the system will only provide the most recent phone/address record."
	PersonSearchFilterIncludeLatestRecordsOnly PersonSearchFilter = "IncludeLatestRecordsOnly"

	// PersonSearchFilterEnableRemoveSingleCharNames is documented as "Filters
	// out first names that are only one character long".
	PersonSearchFilterEnableRemoveSingleCharNames PersonSearchFilter = "EnableRemoveSingleCharNames"

	// PersonSearchFilterEnableExactFirstNameMatch is documented as "Filters out
	// records where the first name is not an exact match".
	PersonSearchFilterEnableExactFirstNameMatch PersonSearchFilter = "EnableExactFirstNameMatch"

	// PersonSearchFilterEnableExactLastNameMatch is documented as "Filters out
	// records where the last name is not an exact match".
	PersonSearchFilterEnableExactLastNameMatch PersonSearchFilter = "EnableExactLastNameMatch"

	// PersonSearchFilterDisableNoDobs is documented as "Returns records with
	// partial DOB and filters out any records with no DOB. Default behavior is
	// to return whatever is present in the data."
	PersonSearchFilterDisableNoDobs PersonSearchFilter = "DisableNoDobs"
)

// PersonSearchRequestName is one name of a request's AKAs or Relatives list.
// The provider types these lists as Name{} without defining it; the SDK sends
// each entry as an object with FirstName, MiddleName and LastName, the members
// of the response's names. That shape is unverified.
type PersonSearchRequestName struct {
	FirstName  string
	MiddleName string
	LastName   string
}

// PersonSearchRequestAddress is one entry of a request's Addresses list, sent
// as AddressLine1, AddressLine2 and County. AddressLine1 is the house number
// and street name, or a PO Box; AddressLine2 is the city and state, the state,
// or the ZIP code.
type PersonSearchRequestAddress struct {
	AddressLine1 string
	AddressLine2 string
	County       string
}

// PersonSearchRequest describes one Person Search page: the mode, the search
// criteria, the includes and filters, and the page.
//
// Mode is required, and at least one criterion must be set: a name part, an
// AKA, DOB, an age field, an address, Email, Phone, ClientIP, a relative or a
// TahoeID. Includes, FilterOptions, the offsets, the year limits and the page
// fields refine a search but are not criteria. The SDK checks only these
// rules and the formats below: it does not decide which criteria a mode
// needs, which age field wins, or whether a filter applies.
//
// Values are sent verbatim, in order, duplicates included: the SDK never
// trims, normalizes, deduplicates or rewrites them. Every non-empty text field
// must be valid UTF-8 of at most 256 bytes, with no control characters (tabs
// included) and not only whitespace, and each list holds at most 32 entries;
// these bounds are this SDK's own, as the provider documents none. A list
// entry whose fields are all empty is rejected as blank. The request is passed
// by value, and the SDK neither keeps nor modifies it or its slices.
//
// The request holds personal data: see the package documentation.
type PersonSearchRequest struct {
	// Mode is the search type, sent as the galaxy-search-type header. It is
	// required and must be one of the PersonSearchMode constants.
	Mode PersonSearchMode

	// FirstName, MiddleName and LastName are sent as FirstName, MiddleName and
	// LastName. MiddleName may be a middle initial.
	FirstName  string
	MiddleName string
	LastName   string

	// AKAs are alternative names, sent as Akas. Their shape is unverified; see
	// PersonSearchRequestName.
	AKAs []PersonSearchRequestName

	// DOB is the date of birth as mm/dd/yyyy, zero-padded, and must be a real
	// calendar date. It is sent as Dob.
	DOB string

	// Age, AgeRangeMinAge and AgeRangeMaxAge are each sent when from 1 to 150;
	// 0 means not sent. The SDK does not check them against each other or
	// against AgeRange.
	Age            int
	AgeRangeMinAge int
	AgeRangeMaxAge int

	// AgeRange is an age range such as "30-40": one to three digits, a hyphen
	// and one to three digits, each number at most 150.
	AgeRange string

	// Addresses are sent as the Addresses list.
	Addresses []PersonSearchRequestAddress

	// Email and Phone are sent verbatim. The phone format is not enforced: the
	// provider documents ###-###-#### and (###)###-####, but its examples
	// elsewhere disagree.
	Email string
	Phone string

	// ClientIP is an IP address to search by, sent as ClientIp. It must be a
	// canonical dotted IPv4 address, such as "192.0.2.10". The provider types
	// the member as an integer while documenting the dotted format; the SDK
	// sends a string, which is unverified.
	ClientIP string

	// Relatives are names of relatives, sent as Relatives. Their shape is
	// unverified; see PersonSearchRequestName.
	Relatives []PersonSearchRequestName

	// TahoeIDs are provider person IDs, such as a result's TahoeID, sent as the
	// TahoeIds list. Whether the provider expects this list or a single
	// TahoeId member is unverified.
	TahoeIDs []string

	// FirstNameCharOffset and LastNameCharOffset are documented as Levenshtein
	// distances that filter records by first and last name. nil means not
	// sent; a pointer to 0 is sent as 0. Each must be from 0 to 256.
	FirstNameCharOffset *int
	LastNameCharOffset  *int

	// DOBFormat is documented to override the date format of returned dates of
	// birth, sent as DobFormat. It is sent verbatim.
	DOBFormat string

	// MaxAddressYears and MaxPhoneYears are documented to filter out addresses
	// and phone numbers older than that many years. nil means not sent; a
	// pointer to 0 is sent as 0. Each must be from 0 to 150.
	MaxAddressYears *int
	MaxPhoneYears   *int

	// Includes and FilterOptions are sent as the Includes and FilterOptions
	// lists, verbatim and in order. Each token must be 1 to 128 ASCII letters
	// and digits. The provider enables both per account.
	Includes      []PersonSearchInclude
	FilterOptions []PersonSearchFilter

	// Page is the page to return, from 1, and ResultsPerPage the number of
	// persons per page, from 1 to 1,000 (an SDK bound). 0 means not sent, and
	// the provider's default applies. Each page is a separate, potentially
	// billable call.
	Page           int
	ResultsPerPage int

	// SessionID is optional and sent as the galaxy-client-session-id header,
	// which the provider documents as a session ID for logging. It must be empty
	// or 1 to 128 bytes of visible ASCII (no spaces). Use an opaque value, never
	// personal data.
	SessionID string
}

// wirePersonSearch is the request body, in the order of the provider's
// properties page. Empty values are omitted; a nil pointer is omitted and a
// pointer to 0 is sent.
type wirePersonSearch struct {
	FirstName           string                    `json:"FirstName,omitempty"`
	MiddleName          string                    `json:"MiddleName,omitempty"`
	LastName            string                    `json:"LastName,omitempty"`
	Akas                []wirePersonSearchName    `json:"Akas,omitempty"`
	Dob                 string                    `json:"Dob,omitempty"`
	Age                 int                       `json:"Age,omitempty"`
	AgeRangeMinAge      int                       `json:"AgeRangeMinAge,omitempty"`
	AgeRangeMaxAge      int                       `json:"AgeRangeMaxAge,omitempty"`
	AgeRange            string                    `json:"AgeRange,omitempty"`
	Addresses           []wirePersonSearchAddress `json:"Addresses,omitempty"`
	Email               string                    `json:"Email,omitempty"`
	ClientIP            string                    `json:"ClientIp,omitempty"`
	Phone               string                    `json:"Phone,omitempty"`
	Relatives           []wirePersonSearchName    `json:"Relatives,omitempty"`
	TahoeIDs            []string                  `json:"TahoeIds,omitempty"`
	FirstNameCharOffset *int                      `json:"FirstNameCharOffset,omitempty"`
	LastNameCharOffset  *int                      `json:"LastNameCharOffset,omitempty"`
	DobFormat           string                    `json:"DobFormat,omitempty"`
	MaxAddressYears     *int                      `json:"MaxAddressYears,omitempty"`
	MaxPhoneYears       *int                      `json:"MaxPhoneYears,omitempty"`
	Includes            []PersonSearchInclude     `json:"Includes,omitempty"`
	FilterOptions       []PersonSearchFilter      `json:"FilterOptions,omitempty"`
	Page                int                       `json:"Page,omitempty"`
	ResultsPerPage      int                       `json:"ResultsPerPage,omitempty"`
}

// wirePersonSearchName is one entry of the body's Akas or Relatives list.
type wirePersonSearchName struct {
	FirstName  string `json:"FirstName,omitempty"`
	MiddleName string `json:"MiddleName,omitempty"`
	LastName   string `json:"LastName,omitempty"`
}

// wirePersonSearchAddress is one entry of the body's Addresses list. The keys
// are PascalCase, as the provider's properties page and examples spell them.
type wirePersonSearchAddress struct {
	AddressLine1 string `json:"AddressLine1,omitempty"`
	AddressLine2 string `json:"AddressLine2,omitempty"`
	County       string `json:"County,omitempty"`
}

// validate checks every field in struct order and then the criteria, returning
// the first failure as a *ValidationError matching ErrInvalidRequest.
func (r *PersonSearchRequest) validate() error {
	if err := r.checkMode(); err != nil {
		return err
	}
	if err := checkFields(
		textField{"FirstName", r.FirstName}, textField{"MiddleName", r.MiddleName}, textField{"LastName", r.LastName},
	); err != nil {
		return err
	}
	if err := checkNameList("AKAs", r.AKAs); err != nil {
		return err
	}
	if err := r.checkAges(); err != nil {
		return err
	}
	if err := r.checkAddresses(); err != nil {
		return err
	}
	if err := r.checkContact(); err != nil {
		return err
	}
	if err := checkNameList("Relatives", r.Relatives); err != nil {
		return err
	}
	if err := r.checkTahoeIDs(); err != nil {
		return err
	}
	if err := r.checkRefinements(); err != nil {
		return err
	}
	if r.SessionID != "" && !validID(r.SessionID) {
		return invalidRequest("SessionID", reasonInvalidHeaderValue)
	}
	if !r.hasCriteria() {
		return invalidRequest("request", reasonNoCriteria)
	}
	return nil
}

// checkMode checks that Mode is one of the documented modes.
func (r *PersonSearchRequest) checkMode() error {
	switch r.Mode {
	case PersonSearchModePerson, PersonSearchModeTeaser,
		PersonSearchModeReversePhonePerson, PersonSearchModeReversePhonePersonTeaser:
		return nil
	case "":
		return invalidRequest("Mode", reasonRequired)
	default:
		return invalidRequest("Mode", reasonUnsupported)
	}
}

// checkAges checks DOB, the three age fields and AgeRange.
func (r *PersonSearchRequest) checkAges() error {
	if err := checkDOB(r.DOB); err != nil {
		return err
	}
	for _, a := range [...]struct {
		field string
		value int
	}{{"Age", r.Age}, {"AgeRangeMinAge", r.AgeRangeMinAge}, {"AgeRangeMaxAge", r.AgeRangeMaxAge}} {
		if a.value < 0 || a.value > maxAge {
			return invalidRequest(a.field, reasonOutOfRange)
		}
	}
	if err := checkText("AgeRange", r.AgeRange); err != nil {
		return err
	}
	if r.AgeRange != "" {
		return checkAgeRange(r.AgeRange)
	}
	return nil
}

// checkAddresses checks the Addresses list.
func (r *PersonSearchRequest) checkAddresses() error {
	if len(r.Addresses) > maxSearchListEntries {
		return invalidRequest("Addresses", reasonTooMany)
	}
	for i, a := range r.Addresses {
		if err := checkEntry("Addresses", i, [...]textField{
			{"AddressLine1", a.AddressLine1}, {"AddressLine2", a.AddressLine2}, {"County", a.County},
		}); err != nil {
			return err
		}
	}
	return nil
}

// checkContact checks Email, Phone and ClientIP.
func (r *PersonSearchRequest) checkContact() error {
	if err := checkFields(
		textField{"Email", r.Email}, textField{"Phone", r.Phone}, textField{"ClientIP", r.ClientIP},
	); err != nil {
		return err
	}
	if r.ClientIP != "" && !validIPv4(r.ClientIP) {
		return invalidRequest("ClientIP", reasonInvalidFormat)
	}
	return nil
}

// checkTahoeIDs checks the TahoeIDs list.
func (r *PersonSearchRequest) checkTahoeIDs() error {
	if len(r.TahoeIDs) > maxSearchListEntries {
		return invalidRequest("TahoeIDs", reasonTooMany)
	}
	for i, id := range r.TahoeIDs {
		reason := textReason(id)
		if id == "" {
			reason = reasonBlank
		}
		if reason != "" {
			return invalidRequest(entryField("TahoeIDs", i, ""), reason)
		}
	}
	return nil
}

// checkRefinements checks the fields that refine a search: the offsets,
// DOBFormat, the year limits, the tokens and the page.
func (r *PersonSearchRequest) checkRefinements() error {
	for _, o := range [...]struct {
		field string
		value *int
		max   int
	}{
		{"FirstNameCharOffset", r.FirstNameCharOffset, maxCharOffset},
		{"LastNameCharOffset", r.LastNameCharOffset, maxCharOffset},
	} {
		if o.value != nil && (*o.value < 0 || *o.value > o.max) {
			return invalidRequest(o.field, reasonOutOfRange)
		}
	}
	if err := checkText("DOBFormat", r.DOBFormat); err != nil {
		return err
	}
	for _, y := range [...]struct {
		field string
		value *int
	}{{"MaxAddressYears", r.MaxAddressYears}, {"MaxPhoneYears", r.MaxPhoneYears}} {
		if y.value != nil && (*y.value < 0 || *y.value > maxYears) {
			return invalidRequest(y.field, reasonOutOfRange)
		}
	}
	if err := checkTokens("Includes", r.Includes); err != nil {
		return err
	}
	if err := checkTokens("FilterOptions", r.FilterOptions); err != nil {
		return err
	}
	if r.Page < 0 || r.Page > math.MaxInt32 {
		return invalidRequest("Page", reasonOutOfRange)
	}
	if r.ResultsPerPage < 0 || r.ResultsPerPage > maxResultsPerPage {
		return invalidRequest("ResultsPerPage", reasonOutOfRange)
	}
	return nil
}

// hasCriteria reports whether the request sets at least one search criterion.
func (r *PersonSearchRequest) hasCriteria() bool {
	return r.FirstName != "" || r.MiddleName != "" || r.LastName != "" || len(r.AKAs) > 0 ||
		r.DOB != "" || r.Age != 0 || r.AgeRangeMinAge != 0 || r.AgeRangeMaxAge != 0 || r.AgeRange != "" ||
		len(r.Addresses) > 0 || r.Email != "" || r.Phone != "" || r.ClientIP != "" ||
		len(r.Relatives) > 0 || len(r.TahoeIDs) > 0
}

// encode validates the request and returns its JSON body: exactly one object,
// with no trailing newline and with <, > and & unescaped. Mode and SessionID
// are headers, never body content.
func (r *PersonSearchRequest) encode() ([]byte, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	w := wirePersonSearch{
		FirstName:           r.FirstName,
		MiddleName:          r.MiddleName,
		LastName:            r.LastName,
		Akas:                wireNames(r.AKAs),
		Dob:                 r.DOB,
		Age:                 r.Age,
		AgeRangeMinAge:      r.AgeRangeMinAge,
		AgeRangeMaxAge:      r.AgeRangeMaxAge,
		AgeRange:            r.AgeRange,
		Email:               r.Email,
		ClientIP:            r.ClientIP,
		Phone:               r.Phone,
		Relatives:           wireNames(r.Relatives),
		TahoeIDs:            r.TahoeIDs,
		FirstNameCharOffset: r.FirstNameCharOffset,
		LastNameCharOffset:  r.LastNameCharOffset,
		DobFormat:           r.DOBFormat,
		MaxAddressYears:     r.MaxAddressYears,
		MaxPhoneYears:       r.MaxPhoneYears,
		Includes:            r.Includes,
		FilterOptions:       r.FilterOptions,
		Page:                r.Page,
		ResultsPerPage:      r.ResultsPerPage,
	}
	if len(r.Addresses) > 0 {
		w.Addresses = make([]wirePersonSearchAddress, len(r.Addresses))
		for i, a := range r.Addresses {
			w.Addresses[i] = wirePersonSearchAddress(a)
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(&w); err != nil {
		// Validated strings and integers always encode; this is unreachable.
		return nil, invalidRequest("request", reasonInvalidFormat)
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte{'\n'}), nil
}

// wireNames copies a list of request names into the body's shape, or returns
// nil for an empty list.
func wireNames(names []PersonSearchRequestName) []wirePersonSearchName {
	if len(names) == 0 {
		return nil
	}
	out := make([]wirePersonSearchName, len(names))
	for i, n := range names {
		out[i] = wirePersonSearchName(n)
	}
	return out
}

// textField is one text value and the field name it is reported under.
type textField struct {
	name  string
	value string
}

// checkFields applies the text rules to each field, in order.
func checkFields(fields ...textField) error {
	for _, f := range fields {
		if err := checkText(f.name, f.value); err != nil {
			return err
		}
	}
	return nil
}

// checkNameList checks a list of request names reported as field.
func checkNameList(field string, names []PersonSearchRequestName) error {
	if len(names) > maxSearchListEntries {
		return invalidRequest(field, reasonTooMany)
	}
	for i, n := range names {
		if err := checkEntry(field, i, [...]textField{
			{"FirstName", n.FirstName}, {"MiddleName", n.MiddleName}, {"LastName", n.LastName},
		}); err != nil {
			return err
		}
	}
	return nil
}

// checkEntry applies the text rules to the fields of entry i of the list
// field, then rejects an entry whose fields are all empty. The indexed field
// name, such as "Addresses[1].County", is built only for a failure.
func checkEntry(field string, i int, parts [3]textField) error {
	blank := true
	for _, p := range parts {
		if reason := textReason(p.value); reason != "" {
			return invalidRequest(entryField(field, i, p.name), reason)
		}
		blank = blank && p.value == ""
	}
	if blank {
		return invalidRequest(entryField(field, i, ""), reasonBlank)
	}
	return nil
}

// entryField names entry i of the list field, or its member sub when set.
func entryField(field string, i int, sub string) string {
	s := field + "[" + strconv.Itoa(i) + "]"
	if sub != "" {
		s += "." + sub
	}
	return s
}

// checkTokens checks a list of Includes or FilterOptions tokens, each 1 to
// maxTokenBytes ASCII letters and digits.
func checkTokens[T ~string](field string, tokens []T) error {
	if len(tokens) > maxSearchListEntries {
		return invalidRequest(field, reasonTooMany)
	}
	for i, tok := range tokens {
		var reason string
		switch {
		case tok == "":
			reason = reasonBlank
		case len(tok) > maxTokenBytes:
			reason = reasonTooLong
		case !alphanumeric(string(tok)):
			reason = reasonInvalidFormat
		}
		if reason != "" {
			return invalidRequest(entryField(field, i, ""), reason)
		}
	}
	return nil
}

// alphanumeric reports whether every byte of s is an ASCII letter or digit.
func alphanumeric(s string) bool {
	for i := range len(s) {
		if c := s[i]; (c < '0' || c > '9') && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
			return false
		}
	}
	return true
}

// checkAgeRange checks a non-empty AgeRange: one to three digits, a hyphen and
// one to three digits, each number at most maxAge.
func checkAgeRange(s string) error {
	lo, hi, ok := strings.Cut(s, "-")
	if !ok || !ageDigits(lo) || !ageDigits(hi) {
		return invalidRequest("AgeRange", reasonInvalidFormat)
	}
	for _, part := range [...]string{lo, hi} {
		if n, _ := strconv.Atoi(part); n > maxAge {
			return invalidRequest("AgeRange", reasonOutOfRange)
		}
	}
	return nil
}

// ageDigits reports whether s is one to three ASCII digits.
func ageDigits(s string) bool {
	if s == "" || len(s) > 3 {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// validIPv4 reports whether s is an IPv4 address in canonical dotted form: no
// leading zeros, no IPv6 or IPv4-mapped form and no zone.
func validIPv4(s string) bool {
	ip, err := netip.ParseAddr(s)
	if err != nil || !ip.Is4() {
		return false
	}
	var buf [len("255.255.255.255")]byte
	return string(ip.AppendTo(buf[:0])) == s
}
