package enformion

import (
	"encoding/json"
	"net/http"
	"reflect"
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

	// budgetPerBodyByte and budgetSlack size a decode budget: what decoding a
	// body may keep, in approximate bytes, is budgetPerBodyByte times the
	// body's length plus budgetSlack. Real responses decode to less than their
	// own size; a body of nested empty entries would otherwise decode to more
	// than a hundred times it.
	budgetPerBodyByte = 4
	budgetSlack       = 1 << 20
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

// Object and path storage.
const (
	// inlineFields is how many members an object holds without allocating:
	// every Contact Enrichment object fits.
	inlineFields = 8

	// maxSlots is the widest slot table that decodes without allocating: a
	// Person Search address has 35 members.
	maxSlots = 40

	// maxPathDepth is how many segments a JSON path holds, such as the five of
	// "persons[0].associates[1].emailAddresses[0].emailEngagementData.naics[2]".
	maxPathDepth = 6
)

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

// pathSeg is one segment of a path: a member name, or a dotted base such as
// "person.phones", and its list index, or -1 when it is not a list entry.
type pathSeg struct {
	name  string
	index int
}

// path names a JSON location for error messages, rendered only when an error
// needs it. It is a small value: building one never allocates. A path holds at
// most maxPathDepth segments; deeper ones stay at their deepest ancestor.
type path struct {
	segs [maxPathDepth]pathSeg
	n    int
}

// at returns the path of the entry at index i of the list at base, or of base
// itself when i is -1.
func at(base string, i int) path {
	return path{segs: [maxPathDepth]pathSeg{{name: base, index: i}}, n: 1}
}

// relative returns the root of a path relative to a member, which renders as
// "": an entry of a list there renders as "[2]", and a member inside it as
// ".zip", ready to follow the member's own path.
func relative() path { return at("", -1) }

// String renders the path, such as "person.phones[1]" or
// "persons[0].addresses[2].highRiskMarker".
func (p path) String() string {
	var b []byte
	for i, s := range p.segs[:p.n] {
		if i > 0 {
			b = append(b, '.')
		}
		b = append(b, s.name...)
		if s.index >= 0 {
			b = append(b, '[')
			b = strconv.AppendInt(b, int64(s.index), 10)
			b = append(b, ']')
		}
	}
	return string(b)
}

// child returns the path of the member name inside p. Past maxPathDepth it
// returns p unchanged.
func (p path) child(name string) path {
	if p.n < len(p.segs) {
		p.segs[p.n] = pathSeg{name: name, index: -1}
		p.n++
	}
	return p
}

// entry returns the path of the entry at index i of the list at p.
func (p path) entry(i int) path {
	if p.n > 0 {
		p.segs[p.n-1].index = i
	}
	return p
}

// member renders the path of the member name inside p. The result is always a
// new string, so that name, often a slot's, never escapes through it.
func (p path) member(name string) string { return p.child(name).String() }

// object is the recognized members of one JSON object, by index into the names
// it was read with, and whether any other member holds data. The first
// inlineFields members are held inline; more is allocated only for an object
// read with more names.
type object struct {
	inline      [inlineFields][]byte
	more        [][]byte
	ignoredData bool
}

// field returns member i, or nil when it is absent or i is out of range.
func (o *object) field(i int) []byte {
	switch {
	case i >= 0 && i < inlineFields:
		return o.inline[i]
	case i >= inlineFields && i-inlineFields < len(o.more):
		return o.more[i-inlineFields]
	}
	return nil
}

// keep records the member value whose key matched index i, or no name when i
// is -1: inline or, past the inline storage, in more; unmatched, into
// ignoredData when track is set. A member matched twice records its index in
// dup, once. An index past more is ignored.
func (o *object) keep(i int, value []byte, more [][]byte, track bool, dup *int) {
	var dst *[]byte
	switch {
	case i < 0:
		if track && !o.ignoredData {
			o.ignoredData = hasData(value)
		}
		return
	case i < inlineFields:
		dst = &o.inline[i]
	case i-inlineFields < len(more):
		dst = &more[i-inlineFields]
	default:
		return
	}
	if *dst != nil {
		if *dup < 0 {
			*dup = i
		}
		return
	}
	*dst = value
}

// readObject reads the JSON object raw against names, matching keys without
// regard to ASCII case. dup is the index of a name two members match, or -1.
// ignoredData is computed only when track is set.
func readObject(raw []byte, names []string, track bool) (o object, dup int) {
	if n := len(names) - inlineFields; n > 0 {
		o.more = make([][]byte, n)
	}
	dup = -1
	for key, value := range members(raw) {
		o.keep(foldIndex(keyBytes(key), names), value, o.more, track, &dup)
	}
	return o, dup
}

// readSlots is readObject against the names of slots, except that members
// past the inline storage are kept in more, which needs len(slots) -
// inlineFields entries, and o.more stays nil. Matching the slots in place,
// rather than copying their names out, and never holding more in o, keep every
// slot's destination, and more itself, on the caller's stack.
func readSlots(raw []byte, slots []slot, track bool, more [][]byte) (o object, dup int) {
	dup = -1
	for key, value := range members(raw) {
		o.keep(slotIndex(keyBytes(key), slots), value, more, track, &dup)
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
		requestID = providerRequestID(env.field(envRequestID))
	}
	if dup >= 0 {
		return nil, duplicated(names[dup]).callError(requestID)
	}

	switch string(env.field(envIsError)) {
	case "false":
	case "true":
		ce := newCallErrorWithDiagnostics(FailureRejected, ex.status, errProviderError,
			envelopeDiagnostics(env.field(envMessage), env.field(envError), withDiagnostics))
		ce.RequestID = requestID
		return nil, ce
	default:
		derr := &decodeError{kind: FailureUnexpectedResponse, cause: errIsErrorFlag, field: names[envIsError]}
		return nil, derr.callError(requestID)
	}

	person, derr := decodePerson(env.field(envPerson))
	if derr != nil {
		return nil, derr.callError(requestID)
	}
	score, ok := decodeInt(env.field(envIdentityScore))
	if !ok {
		return nil, wrongType(names[envIdentityScore]).callError(requestID)
	}
	diag, derr := decodeDiagnostics(env.field(envMessage), env.field(envError), true)
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
	return providerRequestID(env.field(envRequestID)), envelopeDiagnostics(env.field(envMessage), env.field(envError), withDiagnostics)
}

// envelopeDiagnostics returns the diagnostics of a failed call's envelope
// message and error members, skipping any member of the wrong type, when
// enabled, and none otherwise.
func envelopeDiagnostics(msgRaw, errRaw []byte, enabled bool) ProviderDiagnostics {
	if !enabled {
		return ProviderDiagnostics{}
	}
	diag, _ := decodeDiagnostics(msgRaw, errRaw, false)
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
		if *t.dst, ok = decodeString(o.field(t.i)); !ok {
			return nil, wrongType("person." + names[t.i])
		}
	}
	nameData, derr := decodeName(o.field(personName), &p.Name)
	if derr != nil {
		return nil, derr
	}
	var addrData, phoneData, emailData bool
	if p.Addresses, addrData, derr = decodeList(o.field(personAddresses), at("person.addresses", -1), nil, decodeAddress); derr != nil {
		return nil, derr
	}
	if p.Phones, phoneData, derr = decodeList(o.field(personPhones), at("person.phones", -1), nil, decodePhone); derr != nil {
		return nil, derr
	}
	if p.Emails, emailData, derr = decodeList(o.field(personEmails), at("person.emails", -1), nil, decodeEmail); derr != nil {
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

// slot is one member of an object and where it is decoded to. Exactly one
// destination is set:
//   - str: a string; data when non-empty
//   - flag: an optional boolean; data when true
//   - num: an optional integer (see decodeInt); data when non-zero, while a
//     reported 0 is still kept as a pointer to 0
//   - raw: any JSON value, kept normalized (see keepRaw); data when it holds
//     any (see hasData)
//   - sub: a nested object or list, decoded by the function, which reports
//     whether it holds data. It decodes at a path relative to the member (see
//     relative), and decode adds the member's own path to any failure. A
//     slot's name therefore never reaches the function: as an argument of an
//     indirect call, it would move every slot's destination to the heap.
type slot struct {
	name string
	str  *string
	flag **bool
	num  **int
	raw  *json.RawMessage
	sub  func(raw []byte, p path) (bool, *decodeError)
}

// decode decodes the member raw (nil when absent) of the object at p into the
// slot's destination, charging b for a raw value, and reports whether it holds
// data.
func (s *slot) decode(raw []byte, p path, b *budget) (bool, *decodeError) {
	var ok bool
	switch {
	case s.str != nil:
		*s.str, ok = decodeString(raw)
		return *s.str != "", s.check(ok, p)
	case s.flag != nil:
		*s.flag, ok = decodeBool(raw)
		return *s.flag != nil && **s.flag, s.check(ok, p)
	case s.num != nil:
		*s.num, ok = decodeInt(raw)
		return *s.num != nil && **s.num != 0, s.check(ok, p)
	case s.raw != nil:
		var data bool
		var derr *decodeError
		*s.raw, data, derr = keepRaw(raw, p.child(s.name), b)
		return data, derr
	case s.sub != nil:
		data, derr := s.sub(raw, relative())
		if derr != nil {
			derr.field = p.member(s.name) + derr.field
		}
		return data, derr
	}
	return false, nil
}

// check returns the wrong-type failure of the slot's member inside p unless ok.
func (s *slot) check(ok bool, p path) *decodeError {
	if ok {
		return nil
	}
	return wrongType(p.member(s.name))
}

// decodeFlat decodes the flat JSON object raw, at path p, into slots and
// reports whether any recognized member holds data (a non-empty string, or
// true). An object whose only data is under other keys fails as an unexpected
// response.
func decodeFlat(raw []byte, p path, slots []slot) (bool, *decodeError) {
	return decodeSlots(raw, p, nil, true, slots)
}

// decodeSlots decodes the JSON object raw, at path p, into slots, charging b
// (nil for no budget) for raw values, and reports whether any recognized member
// holds data. When track is set, an object whose only data is under other keys
// fails as an unexpected response; otherwise other members are ignored.
func decodeSlots(raw []byte, p path, b *budget, track bool, slots []slot) (bool, *decodeError) {
	switch n := len(slots) - inlineFields; {
	case n <= 0:
		return decodeSlotsIn(raw, p, b, track, slots, nil)
	case n <= maxSlots-inlineFields:
		// A wider object keeps its other members on this stack rather than
		// the heap; only wide objects pay for zeroing it.
		var more [maxSlots - inlineFields][]byte
		return decodeSlotsIn(raw, p, b, track, slots, more[:n])
	default:
		return decodeSlotsIn(raw, p, b, track, slots, make([][]byte, n))
	}
}

// decodeSlotsIn is decodeSlots with more, the storage for members past the
// inline storage (see readSlots).
func decodeSlotsIn(raw []byte, p path, b *budget, track bool, slots []slot, more [][]byte) (bool, *decodeError) {
	o, dup := readSlots(raw, slots, track, more)
	if dup >= 0 {
		return false, duplicated(p.member(slots[dup].name))
	}
	data := false
	for i := range slots {
		var member []byte
		if i < inlineFields {
			member = o.inline[i]
		} else if k := i - inlineFields; k < len(more) {
			member = more[k]
		}
		d, derr := slots[i].decode(member, p, b)
		if derr != nil {
			return false, derr
		}
		data = data || d
	}
	if !data && o.ignoredData {
		return false, unrecognized(p.String())
	}
	return data, nil
}

// decodeObject decodes the member raw, at path p, as a JSON object of slots
// (see decodeSlots). Absent or null is no data; any other type fails.
func decodeObject(raw []byte, p path, b *budget, track bool, slots []slot) (bool, *decodeError) {
	if isNull(raw) {
		return false, nil
	}
	if raw[0] != '{' {
		return false, wrongType(p.String())
	}
	return decodeSlots(raw, p, b, track, slots)
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

// decodeList decodes the JSON array of objects at path p (see decodeArray),
// failing on an element that is not an object.
func decodeList[T any](raw []byte, p path, b *budget, decode func([]byte, path) (T, bool, *decodeError)) ([]T, bool, *decodeError) {
	return decodeArray(raw, p, b, func(el []byte, ep path) (T, bool, *decodeError) {
		if el[0] != '{' {
			var zero T
			return zero, false, wrongType(ep.String())
		}
		return decode(el, ep)
	})
}

// decodeArray decodes the JSON array raw at path p, each element with decode at
// its own path. Every entry is kept, in the provider's order, except JSON null
// elements, which are not entries but still count in the index; more than
// maxListEntries entries fail the call. Each entry is charged its size against
// b, when it is not nil, before it is decoded. data reports whether any entry
// holds data. An empty list is nil.
func decodeArray[T any](raw []byte, p path, b *budget, decode func([]byte, path) (T, bool, *decodeError)) ([]T, bool, *decodeError) {
	if isNull(raw) {
		return nil, false, nil
	}
	if raw[0] != '[' {
		return nil, false, wrongType(p.String())
	}
	if b != nil {
		return decodeBudgeted(raw, p, b, decode)
	}
	// Grown from nil, so that the compiler may grow a short list on the stack
	// and allocate it once, at its size, when it is returned.
	var (
		out  []T
		data bool
		i    = -1
	)
	for el := range elements(raw) {
		i++
		v, d, skip, derr := arrayEntry(el, p.entry(i), len(out), nil, 0, decode)
		if derr != nil {
			return nil, false, derr
		}
		if !skip {
			out = append(out, v)
			data = data || d
		}
	}
	return out, data, nil
}

// decodeBudgeted is decodeArray for a non-empty array with a budget. The list
// is allocated once, at its size, so that growing it leaves no garbage beyond
// what the budget counts.
func decodeBudgeted[T any](raw []byte, p path, b *budget, decode func([]byte, path) (T, bool, *decodeError)) ([]T, bool, *decodeError) {
	n := countEntries(raw)
	if n == 0 {
		return nil, false, nil
	}
	var (
		out  = make([]T, 0, n)
		size = int(reflect.TypeFor[T]().Size())
		data bool
		i    = -1
	)
	for el := range elements(raw) {
		i++
		v, d, skip, derr := arrayEntry(el, p.entry(i), len(out), b, size, decode)
		if derr != nil {
			return nil, false, derr
		}
		if !skip {
			out = append(out, v)
			data = data || d
		}
	}
	return out, data, nil
}

// arrayEntry decodes the array element el at path ep, after n entries,
// charging size against b. skip reports a JSON null element, which is not an
// entry. More than maxListEntries entries fail at the list's path.
func arrayEntry[T any](el []byte, ep path, n int, b *budget, size int,
	decode func([]byte, path) (T, bool, *decodeError),
) (v T, data, skip bool, derr *decodeError) {
	switch {
	case isNull(el):
		return v, false, true, nil
	case n == maxListEntries:
		return v, false, false, tooManyEntries(ep.entry(-1).String())
	}
	if derr = b.charge(size, ep); derr != nil {
		return v, false, false, derr
	}
	v, data, derr = decode(el, ep)
	return v, data, false, derr
}

// countEntries returns how many elements of the JSON array raw are not null,
// at most maxListEntries.
func countEntries(raw []byte) int {
	n := 0
	for el := range elements(raw) {
		if isNull(el) {
			continue
		}
		if n++; n == maxListEntries {
			break
		}
	}
	return n
}

// stringEntry decodes a list element that must be a JSON string. An empty
// string is kept, as an entry without data.
func stringEntry(el []byte, p path) (string, bool, *decodeError) {
	if el[0] != '"' {
		return "", false, wrongType(p.String())
	}
	s := decodeKey(el)
	return s, s != "", nil
}

// budget is what one decode may still keep, in approximate bytes: list entries
// at their size, and raw values at their normalized length. A nil *budget is
// unlimited.
type budget int

// newBudget returns the budget for decoding a body of n bytes.
func newBudget(n int) budget { return budget(budgetPerBodyByte*n + budgetSlack) }

// charge spends n bytes, or fails at path p when fewer are left.
func (b *budget) charge(n int, p path) *decodeError {
	if b == nil {
		return nil
	}
	if n > int(*b) {
		return &decodeError{kind: FailureResponseTooLarge, cause: errDecodeBudget, field: p.String()}
	}
	*b -= budget(n)
	return nil
}

// rawEntry keeps a list element of any JSON type (see keepRaw).
func (b *budget) rawEntry(el []byte, p path) (json.RawMessage, bool, *decodeError) {
	return keepRaw(el, p, b)
}

// keepRaw returns the JSON value raw, at path p, normalized (see
// normalizeJSON) and charged against b, and whether it holds data. Absent and
// null are nil.
func keepRaw(raw []byte, p path, b *budget) (json.RawMessage, bool, *decodeError) {
	if isNull(raw) {
		return nil, false, nil
	}
	norm, err := normalizeJSON(raw)
	if err != nil {
		// A value of a body that passed checkStrict always marshals.
		return nil, false, wrongType(p.String())
	}
	if derr := b.charge(len(norm), p); derr != nil {
		return nil, false, derr
	}
	return norm, hasData(raw), nil
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
		raw := o.field(i)
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
// JSON array raw, and stops reading there. Each entry is normalized (see
// normalizeJSON). Null entries, and entries longer than maxDiagnosticEntryBytes
// once normalized, are dropped. The entries never alias raw.
func diagnosticEntries(raw []byte) []json.RawMessage {
	var out []json.RawMessage
	for el := range elements(raw) {
		if isNull(el) {
			continue
		}
		norm, err := normalizeJSON(el)
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

// normalizeJSON returns a copy of the JSON value raw, normalized as
// encoding/json marshals a json.RawMessage (compact, with <, >, & and U+2028
// and U+2029 escaped), after any lone UTF-16 surrogate escape is replaced by
// �, so that a result round-trips through either JSON package unchanged.
// It never aliases raw.
func normalizeJSON(raw []byte) (json.RawMessage, error) {
	return json.Marshal(json.RawMessage(replaceLoneSurrogates(raw)))
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
