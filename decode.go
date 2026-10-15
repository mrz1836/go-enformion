package enformion

import (
	"encoding/json"
	"net/http"
	"strconv"
	"unicode/utf8"
)

// Diagnostics and collection bounds.
const (
	maxDiagnosticMessageBytes = 1 << 10
	maxDiagnosticEntries      = 32
	maxDiagnosticEntryBytes   = 4 << 10

	// maxListEntries bounds each person collection (addresses, phones,
	// emails). The provider documents at most five; the bound keeps a hostile
	// response from costing many times its own size in memory.
	maxListEntries = 1000
)

// Envelope members this SDK decodes, by index into envelopeNames. Everything
// else in the envelope (searchCriteria, which echoes the request's personal
// data; databaseQueryInfo; pagination, whose sample contradicts a match;
// requestType; totalRequestExecutionTimeMs; and any key added later) is never
// decoded or type-checked. The whole-body checks (valid UTF-8 and JSON, no
// repeated keys) still cover it.
const (
	envPerson = iota
	envMessage
	envIdentityScore
	envRequestID
	envIsError
	envError
)

// Person members this SDK decodes, by index into the names decodePerson reads
// with.
const (
	personID = iota
	personName
	personAge
	personPartialDOB
	personAddresses
	personPhones
	personEmails
)

// maxFields is the most members any decoded object has.
const maxFields = 7

// decodeError is a decoding failure at a JSON path, such as
// "person.phones[1].isConnected". It becomes a CallError once the request ID is
// known.
type decodeError struct {
	kind  FailureKind
	cause error
	field string
}

// callError returns the CallError for e on an HTTP 200 response.
func (e *decodeError) callError(requestID string) *CallError {
	ce := newCallError(e.kind, http.StatusOK, e.cause)
	ce.Field = e.field
	ce.RequestID = requestID
	return ce
}

func wrongType(field string) *decodeError {
	return &decodeError{kind: FailureMalformedResponse, cause: errWrongType, field: field}
}

func duplicated(field string) *decodeError {
	return &decodeError{kind: FailureMalformedResponse, cause: errDuplicateKey, field: field}
}

func unrecognized(field string) *decodeError {
	return &decodeError{kind: FailureUnexpectedResponse, cause: errUnrecognizedData, field: field}
}

func tooManyEntries(field string) *decodeError {
	return &decodeError{kind: FailureResponseTooLarge, cause: errTooManyEntries, field: field}
}

// path names an object for error messages, rendered only when an error needs
// it: a base such as "person.phones" and, for a list entry, its index.
type path struct {
	base  string
	index int
}

// at returns the path of the entry at index i of the list at base.
func at(base string, i int) path { return path{base: base, index: i} }

// String renders the path, such as "person.phones[1]".
func (p path) String() string {
	if p.index < 0 {
		return p.base
	}
	return p.base + "[" + strconv.Itoa(p.index) + "]"
}

// member renders the path of the member name inside p.
func (p path) member(name string) string { return p.String() + "." + name }

// object is the recognized members of one JSON object, by index into the names
// it was read with, and whether any other member holds data.
type object struct {
	fields      [maxFields][]byte
	ignoredData bool
}

// readObject reads the JSON object raw against names, matching keys without
// regard to ASCII case. dup is the index of a name two members match, or -1.
// ignoredData is computed only when track is set.
func readObject(raw []byte, names []string, track bool) (o object, dup int) {
	dup = -1
	for key, value := range members(raw) {
		i := foldIndex(keyBytes(key), names)
		switch {
		case i < 0:
			if track && !o.ignoredData {
				o.ignoredData = hasData(value)
			}
		case o.fields[i] != nil:
			if dup < 0 {
				dup = i
			}
		default:
			o.fields[i] = value
		}
	}
	return o, dup
}

// envelopeNames returns the envelope member names, by env* index.
func envelopeNames() []string {
	return []string{
		envPerson: "person", envMessage: "message", envIdentityScore: "identityScore",
		envRequestID: "requestId", envIsError: "isError", envError: "error",
	}
}

// decodeEnrichContact turns an exchange into a result or a *CallError.
func decodeEnrichContact(ex *exchange, withDiagnostics bool) (*EnrichContactResult, error) {
	if ex.status != http.StatusOK {
		return nil, statusFailure(ex, withDiagnostics)
	}
	if kind, cause := checkStrict(ex.body); cause != nil {
		return nil, newCallError(kind, ex.status, cause)
	}
	names := envelopeNames()
	env, dup := readObject(ex.body, names, false)
	var requestID string
	if dup != envRequestID {
		requestID = providerRequestID(env.fields[envRequestID])
	}
	if dup >= 0 {
		return nil, duplicated(names[dup]).callError(requestID)
	}

	switch string(env.fields[envIsError]) {
	case "false":
	case "true":
		ce := newCallErrorWithDiagnostics(FailureRejected, ex.status, errProviderError,
			envelopeDiagnostics(&env, withDiagnostics))
		ce.RequestID = requestID
		return nil, ce
	default:
		derr := &decodeError{kind: FailureUnexpectedResponse, cause: errIsErrorFlag, field: names[envIsError]}
		return nil, derr.callError(requestID)
	}

	person, derr := decodePerson(env.fields[envPerson])
	if derr != nil {
		return nil, derr.callError(requestID)
	}
	score, ok := decodeInt(env.fields[envIdentityScore])
	if !ok {
		return nil, wrongType(names[envIdentityScore]).callError(requestID)
	}
	diag, derr := decodeDiagnostics(env.fields[envMessage], env.fields[envError], true)
	if derr != nil {
		return nil, derr.callError(requestID)
	}
	return &EnrichContactResult{Person: person, IdentityScore: score, RequestID: requestID, Diagnostics: diag}, nil
}

// statusFailure returns the CallError for a status other than 200. The body is
// read best-effort for the request ID and, when enabled, the diagnostics: a
// body that cannot be parsed never changes the kind.
func statusFailure(ex *exchange, withDiagnostics bool) *CallError {
	requestID, diag := failureEnvelope(ex.body, withDiagnostics)
	ce := newCallErrorWithDiagnostics(statusKind(ex.status), ex.status, errStatus, diag)
	ce.RequestID = requestID
	return ce
}

// failureEnvelope reads the request ID and, when enabled, the diagnostics from
// the body of a failed call. A body that is absent or cannot be parsed yields
// neither.
func failureEnvelope(body []byte, withDiagnostics bool) (string, ProviderDiagnostics) {
	if body == nil {
		return "", ProviderDiagnostics{}
	}
	if kind, _ := checkStrict(body); kind != "" {
		return "", ProviderDiagnostics{}
	}
	env, dup := readObject(body, envelopeNames(), false)
	if dup >= 0 {
		return "", ProviderDiagnostics{}
	}
	return providerRequestID(env.fields[envRequestID]), envelopeDiagnostics(&env, withDiagnostics)
}

// envelopeDiagnostics returns the diagnostics of a failed call's envelope,
// skipping any member of the wrong type, when enabled, and none otherwise.
func envelopeDiagnostics(env *object, enabled bool) ProviderDiagnostics {
	if !enabled {
		return ProviderDiagnostics{}
	}
	diag, _ := decodeDiagnostics(env.fields[envMessage], env.fields[envError], false)
	return diag
}

// statusKind classifies an HTTP status other than 200.
func statusKind(code int) FailureKind {
	switch {
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return FailureUnauthorized
	case code == http.StatusTooManyRequests:
		return FailureRateLimited
	case code >= 400 && code < 500:
		return FailureRejected
	case code >= 500 && code < 600:
		return FailureUpstream
	}
	return FailureUnexpectedResponse
}

// providerRequestID returns the envelope's requestId when it is a string of 1
// to 128 visible ASCII bytes, and "" otherwise. It never fails a call.
func providerRequestID(raw []byte) string {
	if len(raw) < 2 || raw[0] != '"' {
		return ""
	}
	if s := decodeKey(raw); validID(s) {
		return s
	}
	return ""
}

// decodePerson decodes the envelope's person member. It returns nil when the
// member is absent, null, or an object without data. A person, name or entry
// whose only data is under keys this SDK does not decode, such as a renamed
// field, fails as an unexpected response, so a renamed field never makes a
// billed match silently come back empty.
func decodePerson(raw []byte) (*Person, *decodeError) {
	if isNull(raw) {
		return nil, nil
	}
	if raw[0] != '{' {
		return nil, wrongType("person")
	}
	names := [...]string{
		personID: "personId", personName: "name", personAge: "age", personPartialDOB: "partialDob",
		personAddresses: "addresses", personPhones: "phones", personEmails: "emails",
	}
	o, dup := readObject(raw, names[:], true)
	if dup >= 0 {
		return nil, duplicated("person." + names[dup])
	}
	p := &Person{}
	texts := [...]struct {
		i   int
		dst *string
	}{{personID, &p.PersonID}, {personAge, &p.Age}, {personPartialDOB, &p.PartialDOB}}
	for _, t := range texts {
		var ok bool
		if *t.dst, ok = decodeString(o.fields[t.i]); !ok {
			return nil, wrongType("person." + names[t.i])
		}
	}
	nameData, derr := decodeName(o.fields[personName], &p.Name)
	if derr != nil {
		return nil, derr
	}
	var addrData, phoneData, emailData bool
	if p.Addresses, addrData, derr = decodeList(o.fields[personAddresses], "person.addresses", decodeAddress); derr != nil {
		return nil, derr
	}
	if p.Phones, phoneData, derr = decodeList(o.fields[personPhones], "person.phones", decodePhone); derr != nil {
		return nil, derr
	}
	if p.Emails, emailData, derr = decodeList(o.fields[personEmails], "person.emails", decodeEmail); derr != nil {
		return nil, derr
	}
	if p.PersonID != "" || p.Age != "" || p.PartialDOB != "" || nameData || addrData || phoneData || emailData {
		return p, nil
	}
	if o.ignoredData {
		return nil, unrecognized("person")
	}
	return nil, nil
}

// slot is one member of a flat object: a string or an optional boolean.
type slot struct {
	name string
	str  *string
	flag **bool
}

// decodeFlat decodes the flat JSON object raw, at path p, into slots and
// reports whether any recognized member holds data (a non-empty string, or
// true). An object whose only data is under other keys fails as an unexpected
// response.
func decodeFlat(raw []byte, p path, slots []slot) (bool, *decodeError) {
	var names [maxFields]string
	for i, s := range slots {
		names[i] = s.name
	}
	o, dup := readObject(raw, names[:len(slots)], true)
	if dup >= 0 {
		return false, duplicated(p.member(names[dup]))
	}
	data := false
	for i, s := range slots {
		var ok bool
		if s.str != nil {
			if *s.str, ok = decodeString(o.fields[i]); !ok {
				return false, wrongType(p.member(s.name))
			}
			data = data || *s.str != ""
			continue
		}
		if *s.flag, ok = decodeBool(o.fields[i]); !ok {
			return false, wrongType(p.member(s.name))
		}
		data = data || (*s.flag != nil && **s.flag)
	}
	if !data && o.ignoredData {
		return false, unrecognized(p.String())
	}
	return data, nil
}

// decodeName decodes person.name into n.
func decodeName(raw []byte, n *Name) (bool, *decodeError) {
	const field = "person.name"
	if isNull(raw) {
		return false, nil
	}
	if raw[0] != '{' {
		return false, wrongType(field)
	}
	return decodeFlat(raw, at(field, -1), []slot{
		{name: "firstName", str: &n.FirstName},
		{name: "middleName", str: &n.MiddleName},
		{name: "lastName", str: &n.LastName},
	})
}

func decodeAddress(raw []byte, p path) (Address, bool, *decodeError) {
	var a Address
	data, derr := decodeFlat(raw, p, []slot{
		{name: "street", str: &a.Street},
		{name: "unit", str: &a.Unit},
		{name: "city", str: &a.City},
		{name: "state", str: &a.State},
		{name: "zip", str: &a.Zip},
		{name: "firstReportedDate", str: &a.FirstReportedDate},
		{name: "lastReportedDate", str: &a.LastReportedDate},
	})
	return a, data, derr
}

func decodePhone(raw []byte, p path) (Phone, bool, *decodeError) {
	var ph Phone
	data, derr := decodeFlat(raw, p, []slot{
		{name: "number", str: &ph.Number},
		{name: "type", str: &ph.Type},
		{name: "isConnected", flag: &ph.IsConnected},
		{name: "firstReportedDate", str: &ph.FirstReportedDate},
		{name: "lastReportedDate", str: &ph.LastReportedDate},
	})
	return ph, data, derr
}

func decodeEmail(raw []byte, p path) (Email, bool, *decodeError) {
	var e Email
	data, derr := decodeFlat(raw, p, []slot{
		{name: "email", str: &e.Address},
		{name: "isValidated", flag: &e.IsValidated},
		{name: "isBusiness", flag: &e.IsBusiness},
	})
	return e, data, derr
}

// decodeList decodes a JSON array of objects. Every entry is kept, in the
// provider's order, except JSON null elements, which are not entries; more
// than maxListEntries entries fail the call. data reports whether any entry
// holds recognized data. An empty list is nil.
func decodeList[T any](raw []byte, field string, decode func([]byte, path) (T, bool, *decodeError)) ([]T, bool, *decodeError) {
	if isNull(raw) {
		return nil, false, nil
	}
	if raw[0] != '[' {
		return nil, false, wrongType(field)
	}
	var (
		out  []T
		data bool
		i    = -1
	)
	for el := range elements(raw) {
		i++
		if isNull(el) {
			continue
		}
		if len(out) == maxListEntries {
			return nil, false, tooManyEntries(field)
		}
		if el[0] != '{' {
			return nil, false, wrongType(at(field, i).String())
		}
		v, d, derr := decode(el, at(field, i))
		if derr != nil {
			return nil, false, derr
		}
		out = append(out, v)
		data = data || d
	}
	return out, data, nil
}

// decodeString decodes a JSON string or null. ok is false for any other type.
func decodeString(raw []byte) (string, bool) {
	switch {
	case isNull(raw):
		return "", true
	case raw[0] == '"':
		return decodeKey(raw), true
	}
	return "", false
}

// decodeBool decodes a JSON boolean or null; null is nil. ok is false for any
// other type.
func decodeBool(raw []byte) (*bool, bool) {
	switch string(raw) {
	case "", "null":
		return nil, true
	case "true":
		return new(true), true
	case "false":
		return new(false), true
	}
	return nil, false
}

// decodeInt decodes a JSON integer that fits an int, or null; null is nil. ok
// is false for a fraction, an exponent, an out-of-range value or another type.
// The range of int depends on the platform: 32 bits on 386 and arm.
func decodeInt(raw []byte) (*int, bool) {
	if isNull(raw) {
		return nil, true
	}
	n, err := strconv.Atoi(string(raw))
	if err != nil {
		return nil, false
	}
	return &n, true
}

// decodeDiagnostics decodes the envelope's message and error members within the
// bounds documented on ProviderDiagnostics. When strict is false, as for a
// failed call, a member of the wrong type is skipped instead of failing.
func decodeDiagnostics(msgRaw, errRaw []byte, strict bool) (ProviderDiagnostics, *decodeError) {
	var d ProviderDiagnostics
	msg, ok := decodeString(msgRaw)
	if !ok && strict {
		return ProviderDiagnostics{}, wrongType("message")
	}
	d.Message = truncateUTF8(msg, maxDiagnosticMessageBytes)

	if isNull(errRaw) {
		return d, nil
	}
	if errRaw[0] != '{' {
		if strict {
			return ProviderDiagnostics{}, wrongType("error")
		}
		return d, nil
	}
	names := [...]string{"inputErrors", "warnings"}
	o, dup := readObject(errRaw, names[:], false)
	if dup >= 0 {
		if strict {
			return ProviderDiagnostics{}, duplicated("error." + names[dup])
		}
		return d, nil
	}
	for i, dst := range [...]*[]json.RawMessage{&d.InputErrors, &d.Warnings} {
		raw := o.fields[i]
		if isNull(raw) {
			continue
		}
		if raw[0] != '[' {
			if strict {
				return ProviderDiagnostics{}, wrongType("error." + names[i])
			}
			continue
		}
		*dst = diagnosticEntries(raw)
	}
	return d, nil
}

// diagnosticEntries keeps the first maxDiagnosticEntries usable entries of the
// JSON array raw, and stops reading there. Each entry is normalized as
// encoding/json marshals a json.RawMessage (compact, with <, >, & and U+2028
// and U+2029 escaped), after any lone UTF-16 surrogate escape is replaced by
// \ufffd, so that a result round-trips through either JSON package unchanged.
// Null entries, and entries longer than maxDiagnosticEntryBytes once
// normalized, are dropped. The entries never alias raw.
func diagnosticEntries(raw []byte) []json.RawMessage {
	var out []json.RawMessage
	for el := range elements(raw) {
		if isNull(el) {
			continue
		}
		norm, err := json.Marshal(json.RawMessage(replaceLoneSurrogates(el)))
		if err != nil || len(norm) > maxDiagnosticEntryBytes {
			continue
		}
		out = append(out, norm)
		if len(out) == maxDiagnosticEntries {
			break
		}
	}
	return out
}

// truncateUTF8 cuts s to at most n bytes without splitting a UTF-8 sequence.
func truncateUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
