package enformion

import "encoding/json"

// EnrichContactResult is the provider's answer to one EnrichContact call.
//
// It holds personal data the caller asked for: access control, logging,
// retention and permitted use are the caller's responsibility (see the package
// documentation). It has no redacting formatter.
//
// The JSON tags give this SDK's own serialization, which round-trips: a result
// the SDK returned, marshaled and unmarshaled again, is reflect.DeepEqual to the
// original. It is not the provider's wire format.
type EnrichContactResult struct {
	// Person is the provider's top person match. It is nil when the response
	// carried no person data. The meaning of that is undocumented: it is not
	// proof that no match exists, and the SDK makes no claim about billing.
	Person *Person `json:"person,omitempty"`

	// IdentityScore is the provider's identityScore, an uninterpreted integer.
	// It is not a probability or a confidence. nil means the response had none;
	// a reported 0 is a pointer to 0.
	IdentityScore *int `json:"identityScore,omitempty"`

	// RequestID is the provider's requestId, kept only when present and 1 to 128
	// bytes of visible ASCII.
	RequestID string `json:"requestId,omitempty"`

	// Diagnostics holds the provider's message, inputErrors and warnings, if
	// any. They may echo the request's personal data.
	Diagnostics ProviderDiagnostics `json:"diagnostics,omitzero"`
}

// ProviderDiagnostics holds the provider's free-form notes about a call: the
// envelope's message and its error.inputErrors and error.warnings lists.
//
// The structure of list entries is undocumented, so each entry is kept as
// JSON, whatever its type, normalized as encoding/json marshals a
// json.RawMessage: compact, with <, >, &, U+2028 and U+2029 escaped, and any
// unpaired UTF-16 surrogate escape replaced by \ufffd. Bounds: Message is cut
// to 1 KiB on a UTF-8 boundary; each list keeps at most 32 entries, each at most
// 4 KiB once normalized. Longer entries, entries past the 32nd and JSON null
// entries are dropped. Empty lists are nil.
//
// Provider text may echo the request's personal data.
type ProviderDiagnostics struct {
	Message     string            `json:"message,omitempty"`
	InputErrors []json.RawMessage `json:"inputErrors,omitempty"`
	Warnings    []json.RawMessage `json:"warnings,omitempty"`
}

// Person is the provider's top person match.
//
// Every field is optional: the documentation's own examples disagree on which
// fields the standard tier returns. Strings are provider text, kept verbatim and
// never parsed: Age is text on the wire, and the documentation's examples show
// dates as M/D/YYYY without zero padding.
//
// Collections keep the provider's order and every entry it returned, except
// JSON null elements, which are not entries. A collection of more than 1,000
// entries fails the call with ErrResponseTooLarge, an SDK safety bound (the
// provider documents at most five). The SDK never deduplicates, truncates or
// ranks entries, picks a "preferred" one, or infers relationships. This is the
// provider's observation, not identity verification.
type Person struct {
	PersonID   string    `json:"personId,omitempty"`
	Name       Name      `json:"name,omitzero"`
	Age        string    `json:"age,omitempty"`
	PartialDOB string    `json:"partialDob,omitempty"`
	Addresses  []Address `json:"addresses,omitempty"`
	Phones     []Phone   `json:"phones,omitempty"`
	Emails     []Email   `json:"emails,omitempty"`
}

// Name is a person's name as the provider reports it.
type Name struct {
	FirstName  string `json:"firstName,omitempty"`
	MiddleName string `json:"middleName,omitempty"`
	LastName   string `json:"lastName,omitempty"`
}

// Address is one address the provider associates with the person. Zip and the
// dates are text.
type Address struct {
	Street            string `json:"street,omitempty"`
	Unit              string `json:"unit,omitempty"`
	City              string `json:"city,omitempty"`
	State             string `json:"state,omitempty"`
	Zip               string `json:"zip,omitempty"`
	FirstReportedDate string `json:"firstReportedDate,omitempty"`
	LastReportedDate  string `json:"lastReportedDate,omitempty"`
}

// Phone is one phone number the provider associates with the person.
//
// IsConnected is the provider's flag: nil means not reported. It is not consent
// to contact the number, and the provider does not scrub numbers against any
// Do Not Call registry.
type Phone struct {
	Number            string `json:"number,omitempty"`
	Type              string `json:"type,omitempty"`
	IsConnected       *bool  `json:"isConnected,omitempty"`
	FirstReportedDate string `json:"firstReportedDate,omitempty"`
	LastReportedDate  string `json:"lastReportedDate,omitempty"`
}

// Email is one email address the provider associates with the person.
//
// IsValidated and IsBusiness are the provider's flags: nil means not reported.
// Neither is consent to contact the address.
type Email struct {
	Address     string `json:"email,omitempty"`
	IsValidated *bool  `json:"isValidated,omitempty"`
	IsBusiness  *bool  `json:"isBusiness,omitempty"`
}
