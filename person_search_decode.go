package enformion

import (
	"encoding/json"
	"net/http"
)

// Person Search envelope members this SDK decodes, by index into
// searchEnvelopeNames. Everything else in the envelope (searchCriteria, which
// echoes the request's personal data; databaseQueryInfo; requestType;
// requestTime; totalRequestExecutionTimeMs; and any key added later) is never
// decoded or type-checked. The whole-body checks (valid UTF-8 and JSON, no
// repeated keys) still cover it.
const (
	searchEnvPersons = iota
	searchEnvPagination
	searchEnvCounts
	searchEnvRequestID
	searchEnvIsError
	searchEnvMessage
	searchEnvError
	searchEnvCriteriaType
	searchEnvSmartSearchStatistics
)

// searchEnvelopeNames returns the Person Search envelope member names, by
// searchEnv* index.
func searchEnvelopeNames() []string {
	return []string{
		searchEnvPersons: "persons", searchEnvPagination: "pagination", searchEnvCounts: "counts",
		searchEnvRequestID: "requestId", searchEnvIsError: "isError", searchEnvMessage: "message",
		searchEnvError: "error", searchEnvCriteriaType: "criteriaType",
		searchEnvSmartSearchStatistics: "smartSearchStatistics",
	}
}

// decodeSearchPersons turns a Person Search exchange into a result or a
// *CallError.
func decodeSearchPersons(ex *exchange, withDiagnostics bool) (*PersonSearchResult, error) {
	if ex.status != http.StatusOK {
		return nil, statusFailure(ex, withDiagnostics)
	}
	if kind, cause := checkStrict(ex.body); cause != nil {
		return nil, newCallError(kind, ex.status, cause)
	}
	names := searchEnvelopeNames()
	env, dup := readObject(ex.body, names, false)
	var requestID string
	if dup != searchEnvRequestID {
		requestID = providerRequestID(env.field(searchEnvRequestID))
	}
	if dup >= 0 {
		return nil, duplicated(names[dup]).callError(requestID)
	}

	switch string(env.field(searchEnvIsError)) {
	case "false":
	case "true":
		ce := newCallErrorWithDiagnostics(FailureRejected, ex.status, errProviderError,
			envelopeDiagnostics(env.field(searchEnvMessage), env.field(searchEnvError), withDiagnostics))
		ce.RequestID = requestID
		return nil, ce
	default:
		derr := &decodeError{kind: FailureUnexpectedResponse, cause: errIsErrorFlag, field: names[searchEnvIsError]}
		return nil, derr.callError(requestID)
	}

	d := psDecoder{budget: newBudget(len(ex.body))}
	res, derr := d.envelope(&env)
	if derr != nil {
		return nil, derr.callError(requestID)
	}
	res.RequestID = requestID
	return res, nil
}

// psDecoder decodes one Person Search response within its decode budget.
//
// Each object is a slot table. Record objects (persons, names, addresses,
// phones, emails, locations, relatives, associates, dates of birth and death
// records) whose only data is under keys the SDK does not decode fail as an
// unexpected response, so a renamed field never makes a billed page silently
// come back empty. A person keeps such members in Extra, but still fails when
// none of its members is recognized. Flag and metadata objects (pagination,
// counts, indicators, highRiskMarker, emailEngagementData) ignore members the
// SDK does not know.
type psDecoder struct {
	budget budget
}

// envelope decodes the members of a successful envelope. persons is
// required: an envelope without it is unexpected, while null or [] is a page
// without candidates.
func (d *psDecoder) envelope(env *object) (*PersonSearchResult, *decodeError) {
	raw := env.field(searchEnvPersons)
	if raw == nil {
		return nil, &decodeError{kind: FailureUnexpectedResponse, cause: errMissingMember, field: "persons"}
	}
	res := &PersonSearchResult{}
	var derr *decodeError
	if res.Persons, _, derr = decodeList(raw, at("persons", -1), &d.budget, d.person); derr != nil {
		return nil, derr
	}
	if _, derr = d.pagination(env.field(searchEnvPagination), at("pagination", -1), &res.Pagination); derr != nil {
		return nil, derr
	}
	if _, derr = d.counts(env.field(searchEnvCounts), at("counts", -1), &res.Counts); derr != nil {
		return nil, derr
	}
	if res.CriteriaType, _, derr = keepRaw(env.field(searchEnvCriteriaType), at("criteriaType", -1), &d.budget); derr != nil {
		return nil, derr
	}
	raw = env.field(searchEnvSmartSearchStatistics)
	if res.SmartSearchStatistics, _, derr = keepRaw(raw, at("smartSearchStatistics", -1), &d.budget); derr != nil {
		return nil, derr
	}
	if res.Diagnostics, derr = decodeDiagnostics(env.field(searchEnvMessage), env.field(searchEnvError), true); derr != nil {
		return nil, derr
	}
	return res, nil
}

// person decodes one candidate of the persons list.
func (d *psDecoder) person(raw []byte, p path) (PersonSearchPerson, bool, *decodeError) {
	return d.decodePerson(raw, p, true)
}

// associate decodes one entry of a person's associates list: a person whose
// own associates, if any, are kept in Extra rather than decoded.
func (d *psDecoder) associate(raw []byte, p path) (PersonSearchPerson, bool, *decodeError) {
	return d.decodePerson(raw, p, false)
}

// decodePerson decodes one person, with its associates when withAssociates
// is set, and keeps every other member in Extra.
func (d *psDecoder) decodePerson(raw []byte, p path, withAssociates bool) (PersonSearchPerson, bool, *decodeError) {
	var ps PersonSearchPerson
	b := &d.budget
	slots := []slot{
		{name: "tahoeId", str: &ps.TahoeID},
		{name: "name", sub: func(raw []byte, p path) (bool, *decodeError) { return d.name(raw, p, &ps.Name) }},
		{name: "fullName", str: &ps.FullName},
		{name: "isPublic", flag: &ps.IsPublic},
		{name: "isPremium", flag: &ps.IsPremium},
		{name: "isOptedOut", flag: &ps.IsOptedOut},
		{name: "isEnterpriseOptedOut", flag: &ps.IsEnterpriseOptedOut},
		{name: "hasAdditionalData", flag: &ps.HasAdditionalData},
		{name: "optOutDate", str: &ps.OptOutDate},
		{name: "enterpriseOptOutDate", str: &ps.EnterpriseOptOutDate},
		{name: "sparseFlag", num: &ps.SparseFlag},
		{name: "dob", str: &ps.DOB},
		{name: "dod", str: &ps.DOD},
		{name: "age", num: &ps.Age},
		{name: "isCurrentPropertyOwner", str: &ps.IsCurrentPropertyOwner},
		{name: "datesOfBirth", sub: listOf(&ps.DatesOfBirth, b, d.dateOfBirth)},
		{name: "datesOfDeath", sub: rawsOf(&ps.DatesOfDeath, b)},
		{name: "deathRecords", sub: func(raw []byte, p path) (bool, *decodeError) {
			return decodeObject(raw, p, b, true, []slot{
				{name: "isDeceased", flag: &ps.DeathRecords.IsDeceased},
				{name: "sourceSummary", raw: &ps.DeathRecords.SourceSummary},
			})
		}},
		{name: "akas", sub: listOf(&ps.AKAs, b, d.nameEntry)},
		{name: "mergedNames", sub: listOf(&ps.MergedNames, b, d.nameEntry)},
		{name: "locations", sub: listOf(&ps.Locations, b, d.location)},
		{name: "addresses", sub: listOf(&ps.Addresses, b, d.address)},
		{name: "phoneNumbers", sub: listOf(&ps.PhoneNumbers, b, d.phone)},
		{name: "emailAddresses", sub: listOf(&ps.EmailAddresses, b, d.email)},
		{name: "relativesSummary", sub: listOf(&ps.RelativesSummary, b, d.relative)},
		{name: "associatesSummary", sub: listOf(&ps.AssociatesSummary, b, d.associateSummary)},
		{name: "indicators", sub: func(raw []byte, p path) (bool, *decodeError) {
			return d.indicators(raw, p, &ps.Indicators)
		}},
		// associates stays last, so that an associate's table drops it.
		{name: "associates", sub: listOf(&ps.Associates, b, d.associate)},
	}
	if !withAssociates {
		slots = slots[:len(slots)-1]
	}
	data, derr := decodeSlots(raw, p, b, true, slots)
	if derr != nil {
		return PersonSearchPerson{}, false, derr
	}
	if ps.Extra, derr = d.extra(raw, p, slots); derr != nil {
		return PersonSearchPerson{}, false, derr
	}
	return ps, data, nil
}

// extra returns every member of the person raw, at path p, that matches none
// of slots and is not JSON null, normalized and charged against the budget.
// A failure names the person, never the member's key, which is response
// content.
func (d *psDecoder) extra(raw []byte, p path, slots []slot) (map[string]json.RawMessage, *decodeError) {
	var out map[string]json.RawMessage
	for key, value := range members(raw) {
		k := keyBytes(key)
		if isNull(value) || slotIndex(k, slots) >= 0 {
			continue
		}
		norm, err := normalizeJSON(value)
		if err != nil {
			// A value of a body that passed checkStrict always marshals.
			return nil, wrongType(p.String())
		}
		if derr := d.budget.charge(len(k)+len(norm)+extraEntryBytes, p); derr != nil {
			return nil, derr
		}
		if out == nil {
			out = make(map[string]json.RawMessage)
		}
		out[string(k)] = norm
	}
	return out, nil
}

// extraEntryBytes approximates what one Extra entry costs beyond its key and
// value bytes: the key and value headers and the map's own slot.
const extraEntryBytes = 64

// name decodes a name object into n.
func (d *psDecoder) name(raw []byte, p path, n *PersonSearchName) (bool, *decodeError) {
	return decodeObject(raw, p, &d.budget, true, []slot{
		{name: "prefix", str: &n.Prefix},
		{name: "firstName", str: &n.FirstName},
		{name: "middleName", str: &n.MiddleName},
		{name: "lastName", str: &n.LastName},
		{name: "suffix", str: &n.Suffix},
		{name: "rawNames", sub: stringsOf(&n.RawNames, &d.budget)},
		{name: "publicFirstSeenDate", str: &n.PublicFirstSeenDate},
		{name: "totalFirstSeenDate", str: &n.TotalFirstSeenDate},
		{name: "sourceSummary", raw: &n.SourceSummary},
	})
}

// nameEntry decodes one entry of a list of names.
func (d *psDecoder) nameEntry(raw []byte, p path) (PersonSearchName, bool, *decodeError) {
	var n PersonSearchName
	data, derr := d.name(raw, p, &n)
	return n, data, derr
}

func (d *psDecoder) dateOfBirth(raw []byte, p path) (PersonSearchDateOfBirth, bool, *decodeError) {
	var v PersonSearchDateOfBirth
	data, derr := decodeSlots(raw, p, &d.budget, true, []slot{
		{name: "age", num: &v.Age},
		{name: "sourceSummary", raw: &v.SourceSummary},
	})
	return v, data, derr
}

func (d *psDecoder) location(raw []byte, p path) (PersonSearchLocation, bool, *decodeError) {
	var v PersonSearchLocation
	data, derr := decodeSlots(raw, p, &d.budget, true, []slot{
		{name: "city", str: &v.City},
		{name: "state", str: &v.State},
	})
	return v, data, derr
}

func (d *psDecoder) address(raw []byte, p path) (PersonSearchAddress, bool, *decodeError) {
	var a PersonSearchAddress
	b := &d.budget
	data, derr := decodeSlots(raw, p, b, true, []slot{
		{name: "fullAddress", str: &a.FullAddress},
		{name: "houseNumber", str: &a.HouseNumber},
		{name: "streetPreDirection", str: &a.StreetPreDirection},
		{name: "streetName", str: &a.StreetName},
		{name: "streetPostDirection", str: &a.StreetPostDirection},
		{name: "streetType", str: &a.StreetType},
		{name: "unit", str: &a.Unit},
		{name: "unitType", str: &a.UnitType},
		{name: "city", str: &a.City},
		{name: "state", str: &a.State},
		{name: "county", str: &a.County},
		{name: "zip", str: &a.Zip},
		{name: "zip4", str: &a.Zip4},
		{name: "dpc", str: &a.DPC},
		{name: "latitude", str: &a.Latitude},
		{name: "longitude", str: &a.Longitude},
		{name: "addressHash", str: &a.AddressHash},
		{name: "bldgCode", str: &a.BldgCode},
		{name: "utilityCode", str: &a.UtilityCode},
		{name: "propertyIndicator", str: &a.PropertyIndicator},
		{name: "firstReportedDate", str: &a.FirstReportedDate},
		{name: "lastReportedDate", str: &a.LastReportedDate},
		{name: "publicFirstSeenDate", str: &a.PublicFirstSeenDate},
		{name: "totalFirstSeenDate", str: &a.TotalFirstSeenDate},
		{name: "isPublic", flag: &a.IsPublic},
		{name: "isDeliverable", flag: &a.IsDeliverable},
		{name: "isMergedAddress", flag: &a.IsMergedAddress},
		{name: "unitCount", num: &a.UnitCount},
		{name: "addressOrder", num: &a.AddressOrder},
		{name: "highRiskMarker", sub: func(raw []byte, p path) (bool, *decodeError) {
			return decodeObject(raw, p, b, false, []slot{
				{name: "isHighRisk", flag: &a.HighRiskMarker.IsHighRisk},
				{name: "sic", str: &a.HighRiskMarker.SIC},
				{name: "addressType", str: &a.HighRiskMarker.AddressType},
			})
		}},
		{name: "phoneNumbers", sub: stringsOf(&a.PhoneNumbers, b)},
		{name: "addressQualityCodes", sub: stringsOf(&a.AddressQualityCodes, b)},
		{name: "neighbors", sub: rawsOf(&a.Neighbors, b)},
		{name: "neighborSummaryRecords", sub: rawsOf(&a.NeighborSummaryRecords, b)},
		{name: "sourceSummary", raw: &a.SourceSummary},
	})
	return a, data, derr
}

func (d *psDecoder) phone(raw []byte, p path) (PersonSearchPhone, bool, *decodeError) {
	var ph PersonSearchPhone
	data, derr := decodeSlots(raw, p, &d.budget, true, []slot{
		{name: "phoneNumber", str: &ph.PhoneNumber},
		{name: "company", str: &ph.Company},
		{name: "location", str: &ph.Location},
		{name: "phoneType", str: &ph.PhoneType},
		{name: "latitude", str: &ph.Latitude},
		{name: "longitude", str: &ph.Longitude},
		{name: "firstReportedDate", str: &ph.FirstReportedDate},
		{name: "lastReportedDate", str: &ph.LastReportedDate},
		{name: "publicFirstSeenDate", str: &ph.PublicFirstSeenDate},
		{name: "totalFirstSeenDate", str: &ph.TotalFirstSeenDate},
		{name: "isConnected", flag: &ph.IsConnected},
		{name: "isPublic", flag: &ph.IsPublic},
		{name: "phoneOrder", num: &ph.PhoneOrder},
		{name: "sourceSummary", raw: &ph.SourceSummary},
	})
	return ph, data, derr
}

func (d *psDecoder) email(raw []byte, p path) (PersonSearchEmail, bool, *decodeError) {
	var e PersonSearchEmail
	b := &d.budget
	en := &e.EmailEngagement
	data, derr := decodeSlots(raw, p, b, true, []slot{
		{name: "emailAddress", str: &e.EmailAddress},
		{name: "emailOrdinal", num: &e.EmailOrdinal},
		{name: "isPremium", flag: &e.IsPremium},
		{name: "nonBusiness", num: &e.NonBusiness},
		{name: "emailEngagementData", sub: func(raw []byte, p path) (bool, *decodeError) {
			return decodeObject(raw, p, b, false, []slot{
				{name: "isBounce", flag: &en.IsBounce},
				{name: "isMatched", flag: &en.IsMatched},
				{name: "isGoodDomain", flag: &en.IsGoodDomain},
				{name: "sendTime", num: &en.SendTime},
				{name: "engagementScore", num: &en.EngagementScore},
				{name: "frequency", str: &en.Frequency},
				{name: "bestDayOfTheWeek", str: &en.BestDayOfTheWeek},
				{name: "bestTimeOfTheDay", str: &en.BestTimeOfTheDay},
				{name: "naics", sub: stringsOf(&en.NAICS, b)},
				{name: "lastCheckDate", str: &en.LastCheckDate},
				{name: "lastTouchDate", str: &en.LastTouchDate},
			})
		}},
		{name: "sourceSummary", raw: &e.SourceSummary},
	})
	return e, data, derr
}

func (d *psDecoder) relative(raw []byte, p path) (PersonSearchRelative, bool, *decodeError) {
	var r PersonSearchRelative
	data, derr := decodeSlots(raw, p, &d.budget, true, []slot{
		{name: "tahoeId", str: &r.TahoeID},
		{name: "prefix", str: &r.Prefix},
		{name: "firstName", str: &r.FirstName},
		{name: "middleName", str: &r.MiddleName},
		{name: "lastName", str: &r.LastName},
		{name: "suffix", str: &r.Suffix},
		{name: "dob", str: &r.DOB},
		{name: "relativeLevel", str: &r.RelativeLevel},
		{name: "relativeType", str: &r.RelativeType},
		{name: "city", str: &r.City},
		{name: "state", str: &r.State},
		{name: "spouse", num: &r.Spouse},
		{name: "score", num: &r.Score},
		{name: "sharedHouseholdIds", sub: stringsOf(&r.SharedHouseholdIDs, &d.budget)},
		{name: "isPrivate", flag: &r.IsPrivate},
		{name: "isOptedOut", flag: &r.IsOptedOut},
		{name: "isDeceased", flag: &r.IsDeceased},
	})
	return r, data, derr
}

func (d *psDecoder) associateSummary(raw []byte, p path) (PersonSearchAssociate, bool, *decodeError) {
	var a PersonSearchAssociate
	data, derr := decodeSlots(raw, p, &d.budget, true, []slot{
		{name: "tahoeId", str: &a.TahoeID},
		{name: "prefix", str: &a.Prefix},
		{name: "firstName", str: &a.FirstName},
		{name: "middleName", str: &a.MiddleName},
		{name: "lastName", str: &a.LastName},
		{name: "suffix", str: &a.Suffix},
		{name: "dob", str: &a.DOB},
		{name: "score", num: &a.Score},
	})
	return a, data, derr
}

// indicators decodes a person's indicators object into v.
func (d *psDecoder) indicators(raw []byte, p path, v *PersonSearchIndicators) (bool, *decodeError) {
	return decodeObject(raw, p, &d.budget, false, []slot{
		{name: "hasBankruptcyRecords", num: &v.HasBankruptcyRecords},
		{name: "hasBusinessRecords", num: &v.HasBusinessRecords},
		{name: "hasDivorceRecords", num: &v.HasDivorceRecords},
		{name: "hasDomainsRecords", num: &v.HasDomainsRecords},
		{name: "hasEvictionsRecords", num: &v.HasEvictionsRecords},
		{name: "hasFeinRecords", num: &v.HasFEINRecords},
		{name: "hasForeclosuresRecords", num: &v.HasForeclosuresRecords},
		{name: "hasJudgmentRecords", num: &v.HasJudgmentRecords},
		{name: "hasLienRecords", num: &v.HasLienRecords},
		{name: "hasMarriageRecords", num: &v.HasMarriageRecords},
		{name: "hasProfessionalLicenseRecords", num: &v.HasProfessionalLicenseRecords},
		{name: "hasPropertyRecords", num: &v.HasPropertyRecords},
		{name: "hasVehicleRegistrationsRecords", num: &v.HasVehicleRegistrationsRecords},
		{name: "hasWorkplaceRecords", num: &v.HasWorkplaceRecords},
		{name: "hasDeaRecords", num: &v.HasDEARecords},
		{name: "hasPropertyV2Records", num: &v.HasPropertyV2Records},
		{name: "hasForeclosuresV2Records", num: &v.HasForeclosuresV2Records},
		{name: "hasUccRecords", num: &v.HasUCCRecords},
		{name: "hasDebtRecords", num: &v.HasDebtRecords},
	})
}

// pagination decodes the envelope's pagination object into v.
func (d *psDecoder) pagination(raw []byte, p path, v *PersonSearchPagination) (bool, *decodeError) {
	return decodeObject(raw, p, &d.budget, false, []slot{
		{name: "currentPageNumber", num: &v.CurrentPageNumber},
		{name: "resultsPerPage", num: &v.ResultsPerPage},
		{name: "totalPages", num: &v.TotalPages},
		{name: "totalResults", num: &v.TotalResults},
	})
}

// counts decodes the envelope's counts object into v.
func (d *psDecoder) counts(raw []byte, p path, v *PersonSearchCounts) (bool, *decodeError) {
	return decodeObject(raw, p, &d.budget, false, []slot{
		{name: "names", num: &v.Names},
		{name: "addresses", num: &v.Addresses},
		{name: "relatives", num: &v.Relatives},
		{name: "associates", num: &v.Associates},
		{name: "debtRecords", num: &v.DebtRecords},
		{name: "datesOfBirth", num: &v.DatesOfBirth},
		{name: "datesOfDeath", num: &v.DatesOfDeath},
		{name: "deathRecords", num: &v.DeathRecords},
		{name: "phoneNumbers", num: &v.PhoneNumbers},
		{name: "expectedCount", num: &v.ExpectedCount},
		{name: "searchResults", num: &v.SearchResults},
		{name: "emailAddresses", num: &v.EmailAddresses},
		{name: "businessRecords", num: &v.BusinessRecords},
		{name: "evictionRecords", num: &v.EvictionRecords},
		{name: "foreclosureRecords", num: &v.ForeclosureRecords},
		{name: "foreclosureV2Records", num: &v.ForeclosureV2Records},
		{name: "socialSecurityNumbers", num: &v.SocialSecurityNumbers},
		{name: "professionalLicenseRecords", num: &v.ProfessionalLicenseRecords},
		{name: "searchResultsOverflow", flag: &v.SearchResultsOverflow},
	})
}

// listOf returns a slot decoder for a list of objects, kept in dst. It is
// small enough to inline, so the closure stays on its caller's stack.
func listOf[T any](dst *[]T, b *budget, decode func([]byte, path) (T, bool, *decodeError)) func([]byte, path) (bool, *decodeError) {
	return func(raw []byte, p path) (data bool, derr *decodeError) {
		*dst, data, derr = decodeList(raw, p, b, decode)
		return data, derr
	}
}

// stringsOf returns a slot decoder for a list of strings, kept in dst.
func stringsOf(dst *[]string, b *budget) func([]byte, path) (bool, *decodeError) {
	return func(raw []byte, p path) (data bool, derr *decodeError) {
		*dst, data, derr = decodeArray(raw, p, b, stringEntry)
		return data, derr
	}
}

// rawsOf returns a slot decoder for a list of raw JSON values, kept in dst.
func rawsOf(dst *[]json.RawMessage, b *budget) func([]byte, path) (bool, *decodeError) {
	return func(raw []byte, p path) (data bool, derr *decodeError) {
		*dst, data, derr = decodeArray(raw, p, b, b.rawEntry)
		return data, derr
	}
}
