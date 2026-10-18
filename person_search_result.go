package enformion

import "encoding/json"

// PersonSearchResult is one page of the provider's answer to a SearchPersons
// call.
//
// It holds personal data the caller asked for, and often data about other
// people (relatives, associates, neighbors): access control, logging,
// retention and permitted use are the caller's responsibility (see the package
// documentation). It has no redacting formatter.
//
// Persons are the provider's candidates, in its order and with any duplicates
// it sent. The SDK never selects, ranks, merges or deduplicates candidates, and
// a candidate is not identity verification. An empty page means only that the
// response carried no candidates.
//
// The JSON tags give this SDK's own serialization, which round-trips: a result
// the SDK returned, marshaled and unmarshaled again, is reflect.DeepEqual to the
// original. It is not the provider's wire format.
type PersonSearchResult struct {
	// Persons are the page's candidates. nil means the response's persons
	// list was null or empty.
	Persons []PersonSearchPerson `json:"persons,omitempty"`

	// Pagination is the provider's pagination object. Its member names are
	// inferred from the provider's other endpoints and unverified.
	Pagination PersonSearchPagination `json:"pagination,omitzero"`

	// Counts is the provider's counts object.
	Counts PersonSearchCounts `json:"counts,omitzero"`

	// RequestID is the provider's requestId, kept only when present and 1 to 128
	// bytes of visible ASCII.
	RequestID string `json:"requestId,omitempty"`

	// CriteriaType and SmartSearchStatistics are the provider's criteriaType
	// and smartSearchStatistics members, whose structure is undocumented, kept
	// as raw JSON (see PersonSearchPerson.Extra for the normalization). nil
	// means absent or null. They may echo the request's personal data.
	CriteriaType          json.RawMessage `json:"criteriaType,omitempty"`
	SmartSearchStatistics json.RawMessage `json:"smartSearchStatistics,omitempty"`

	// Diagnostics holds the provider's message, inputErrors and warnings, if
	// any. They may echo the request's personal data.
	Diagnostics ProviderDiagnostics `json:"diagnostics,omitzero"`
}

// PersonSearchPagination is the response's pagination object. Each member is
// nil when not reported; a reported 0 is a pointer to 0.
type PersonSearchPagination struct {
	CurrentPageNumber *int `json:"currentPageNumber,omitempty"`
	ResultsPerPage    *int `json:"resultsPerPage,omitempty"`
	TotalPages        *int `json:"totalPages,omitempty"`
	TotalResults      *int `json:"totalResults,omitempty"`
}

// PersonSearchCounts is the response's counts object: uninterpreted provider
// integers, such as how many search results or addresses it found. Each
// member is nil when not reported; a reported 0 is a pointer to 0. Members the
// SDK does not know are ignored.
type PersonSearchCounts struct {
	Names                      *int  `json:"names,omitempty"`
	Addresses                  *int  `json:"addresses,omitempty"`
	Relatives                  *int  `json:"relatives,omitempty"`
	Associates                 *int  `json:"associates,omitempty"`
	DebtRecords                *int  `json:"debtRecords,omitempty"`
	DatesOfBirth               *int  `json:"datesOfBirth,omitempty"`
	DatesOfDeath               *int  `json:"datesOfDeath,omitempty"`
	DeathRecords               *int  `json:"deathRecords,omitempty"`
	PhoneNumbers               *int  `json:"phoneNumbers,omitempty"`
	ExpectedCount              *int  `json:"expectedCount,omitempty"`
	SearchResults              *int  `json:"searchResults,omitempty"`
	EmailAddresses             *int  `json:"emailAddresses,omitempty"`
	BusinessRecords            *int  `json:"businessRecords,omitempty"`
	EvictionRecords            *int  `json:"evictionRecords,omitempty"`
	ForeclosureRecords         *int  `json:"foreclosureRecords,omitempty"`
	ForeclosureV2Records       *int  `json:"foreclosureV2Records,omitempty"`
	SocialSecurityNumbers      *int  `json:"socialSecurityNumbers,omitempty"`
	ProfessionalLicenseRecords *int  `json:"professionalLicenseRecords,omitempty"`
	SearchResultsOverflow      *bool `json:"searchResultsOverflow,omitempty"`
}

// PersonSearchPerson is one candidate person.
//
// Every field is optional, and which ones the provider fills depends on the
// mode and the account's includes; teaser modes are documented to mask or
// limit data. Strings are provider text, kept verbatim and never parsed: dates
// and IDs among them. Numbers and flags are pointers, nil when not reported.
// A flag, such as IsOptedOut, is the provider's, and none is consent or
// eligibility for any purpose.
//
// Collections keep the provider's order and every entry it returned, except
// JSON null elements, which are not entries. A collection of more than 1,000
// entries fails the call with ErrResponseTooLarge, an SDK safety bound. Empty
// collections are nil.
type PersonSearchPerson struct {
	TahoeID  string           `json:"tahoeId,omitempty"`
	Name     PersonSearchName `json:"name,omitzero"`
	FullName string           `json:"fullName,omitempty"`

	IsPublic             *bool  `json:"isPublic,omitempty"`
	IsPremium            *bool  `json:"isPremium,omitempty"`
	IsOptedOut           *bool  `json:"isOptedOut,omitempty"`
	IsEnterpriseOptedOut *bool  `json:"isEnterpriseOptedOut,omitempty"`
	HasAdditionalData    *bool  `json:"hasAdditionalData,omitempty"`
	OptOutDate           string `json:"optOutDate,omitempty"`
	EnterpriseOptOutDate string `json:"enterpriseOptOutDate,omitempty"`
	SparseFlag           *int   `json:"sparseFlag,omitempty"`

	// DOB and DOD are dates of birth and death as provider text, which may be
	// partial or masked.
	DOB string `json:"dob,omitempty"`
	DOD string `json:"dod,omitempty"`
	Age *int   `json:"age,omitempty"`

	// IsCurrentPropertyOwner is provider text, such as "Not Available", not a
	// boolean.
	IsCurrentPropertyOwner string `json:"isCurrentPropertyOwner,omitempty"`

	DatesOfBirth []PersonSearchDateOfBirth `json:"datesOfBirth,omitempty"`

	// DatesOfDeath entries have no documented structure, so each is kept as
	// raw JSON (see Extra for the normalization).
	DatesOfDeath []json.RawMessage       `json:"datesOfDeath,omitempty"`
	DeathRecords PersonSearchDeathRecord `json:"deathRecords,omitzero"`

	AKAs              []PersonSearchName      `json:"akas,omitempty"`
	MergedNames       []PersonSearchName      `json:"mergedNames,omitempty"`
	Locations         []PersonSearchLocation  `json:"locations,omitempty"`
	Addresses         []PersonSearchAddress   `json:"addresses,omitempty"`
	PhoneNumbers      []PersonSearchPhone     `json:"phoneNumbers,omitempty"`
	EmailAddresses    []PersonSearchEmail     `json:"emailAddresses,omitempty"`
	RelativesSummary  []PersonSearchRelative  `json:"relativesSummary,omitempty"`
	AssociatesSummary []PersonSearchAssociate `json:"associatesSummary,omitempty"`

	// Associates are associated people as full persons, which the provider
	// returns with the Associates include. They are decoded one level deep:
	// an associate's own associates, if any, are kept in its Extra.
	Associates []PersonSearchPerson `json:"associates,omitempty"`

	Indicators PersonSearchIndicators `json:"indicators,omitzero"`

	// Extra holds every member of the person this SDK does not decode, such
	// as driversLicenseDetail or relatives, by its key as sent, so that a
	// member the provider adds or leaves undocumented is never lost. JSON null
	// members are left out, and an empty map is nil.
	//
	// Each value is a copy of the provider's JSON, never aliasing the
	// response, normalized as encoding/json marshals a json.RawMessage
	// (compact, with <, >, &, U+2028 and U+2029 escaped) after any unpaired
	// UTF-16 surrogate escape is replaced by �. The SDK neither types nor
	// interprets these values: they are opaque, may hold sensitive personal
	// data, including other people's, and print in full when formatted. They
	// count toward the response limit and the decode budget.
	Extra map[string]json.RawMessage `json:"extra,omitempty"`
}

// PersonSearchName is a name of a person: the person's own, an AKA or a
// merged name.
type PersonSearchName struct {
	Prefix     string `json:"prefix,omitempty"`
	FirstName  string `json:"firstName,omitempty"`
	MiddleName string `json:"middleName,omitempty"`
	LastName   string `json:"lastName,omitempty"`
	Suffix     string `json:"suffix,omitempty"`

	// RawNames are the name's other spellings as the provider reports them.
	RawNames []string `json:"rawNames,omitempty"`

	PublicFirstSeenDate string `json:"publicFirstSeenDate,omitempty"`
	TotalFirstSeenDate  string `json:"totalFirstSeenDate,omitempty"`

	// SourceSummary has no documented structure and is kept as raw JSON (see
	// PersonSearchPerson.Extra for the normalization). nil means absent or
	// null. The same holds for every SourceSummary below.
	SourceSummary json.RawMessage `json:"sourceSummary,omitempty"`
}

// PersonSearchDateOfBirth is one entry of a person's datesOfBirth list.
type PersonSearchDateOfBirth struct {
	Age           *int            `json:"age,omitempty"`
	SourceSummary json.RawMessage `json:"sourceSummary,omitempty"`
}

// PersonSearchDeathRecord is a person's deathRecords object. IsDeceased is
// the provider's flag, nil when not reported.
type PersonSearchDeathRecord struct {
	IsDeceased    *bool           `json:"isDeceased,omitempty"`
	SourceSummary json.RawMessage `json:"sourceSummary,omitempty"`
}

// PersonSearchLocation is one entry of a person's locations list.
type PersonSearchLocation struct {
	City  string `json:"city,omitempty"`
	State string `json:"state,omitempty"`
}

// PersonSearchAddress is one address the provider associates with a person.
// Every string, ZIP codes, coordinates, the address hash and dates among
// them, is provider text, never parsed.
type PersonSearchAddress struct {
	FullAddress         string `json:"fullAddress,omitempty"`
	HouseNumber         string `json:"houseNumber,omitempty"`
	StreetPreDirection  string `json:"streetPreDirection,omitempty"`
	StreetName          string `json:"streetName,omitempty"`
	StreetPostDirection string `json:"streetPostDirection,omitempty"`
	StreetType          string `json:"streetType,omitempty"`
	Unit                string `json:"unit,omitempty"`
	UnitType            string `json:"unitType,omitempty"`
	City                string `json:"city,omitempty"`
	State               string `json:"state,omitempty"`
	County              string `json:"county,omitempty"`
	Zip                 string `json:"zip,omitempty"`
	Zip4                string `json:"zip4,omitempty"`
	DPC                 string `json:"dpc,omitempty"`
	Latitude            string `json:"latitude,omitempty"`
	Longitude           string `json:"longitude,omitempty"`
	AddressHash         string `json:"addressHash,omitempty"`
	BldgCode            string `json:"bldgCode,omitempty"`
	UtilityCode         string `json:"utilityCode,omitempty"`
	PropertyIndicator   string `json:"propertyIndicator,omitempty"`

	FirstReportedDate   string `json:"firstReportedDate,omitempty"`
	LastReportedDate    string `json:"lastReportedDate,omitempty"`
	PublicFirstSeenDate string `json:"publicFirstSeenDate,omitempty"`
	TotalFirstSeenDate  string `json:"totalFirstSeenDate,omitempty"`

	IsPublic        *bool `json:"isPublic,omitempty"`
	IsDeliverable   *bool `json:"isDeliverable,omitempty"`
	IsMergedAddress *bool `json:"isMergedAddress,omitempty"`
	UnitCount       *int  `json:"unitCount,omitempty"`
	AddressOrder    *int  `json:"addressOrder,omitempty"`

	HighRiskMarker PersonSearchHighRiskMarker `json:"highRiskMarker,omitzero"`

	// PhoneNumbers are phone numbers the provider associates with the
	// address, as text.
	PhoneNumbers        []string `json:"phoneNumbers,omitempty"`
	AddressQualityCodes []string `json:"addressQualityCodes,omitempty"`

	// Neighbors and NeighborSummaryRecords have no documented structure, so
	// each entry is kept as raw JSON (see PersonSearchPerson.Extra for the
	// normalization). They describe other people.
	Neighbors              []json.RawMessage `json:"neighbors,omitempty"`
	NeighborSummaryRecords []json.RawMessage `json:"neighborSummaryRecords,omitempty"`

	SourceSummary json.RawMessage `json:"sourceSummary,omitempty"`
}

// PersonSearchHighRiskMarker is an address's highRiskMarker object: the
// provider's flag and codes, uninterpreted.
type PersonSearchHighRiskMarker struct {
	IsHighRisk  *bool  `json:"isHighRisk,omitempty"`
	SIC         string `json:"sic,omitempty"`
	AddressType string `json:"addressType,omitempty"`
}

// PersonSearchPhone is one phone number the provider associates with a
// person.
//
// IsConnected and IsPublic are the provider's flags: nil means not reported.
// Neither is consent to contact the number, and the provider does not scrub
// numbers against any Do Not Call registry.
type PersonSearchPhone struct {
	PhoneNumber string `json:"phoneNumber,omitempty"`
	Company     string `json:"company,omitempty"`
	Location    string `json:"location,omitempty"`
	PhoneType   string `json:"phoneType,omitempty"`
	Latitude    string `json:"latitude,omitempty"`
	Longitude   string `json:"longitude,omitempty"`

	FirstReportedDate   string `json:"firstReportedDate,omitempty"`
	LastReportedDate    string `json:"lastReportedDate,omitempty"`
	PublicFirstSeenDate string `json:"publicFirstSeenDate,omitempty"`
	TotalFirstSeenDate  string `json:"totalFirstSeenDate,omitempty"`

	IsConnected *bool `json:"isConnected,omitempty"`
	IsPublic    *bool `json:"isPublic,omitempty"`
	PhoneOrder  *int  `json:"phoneOrder,omitempty"`

	SourceSummary json.RawMessage `json:"sourceSummary,omitempty"`
}

// PersonSearchEmail is one email address the provider associates with a
// person.
//
// NonBusiness is the provider's integer, 0 or 1 on the wire, kept as it is
// rather than converted to a boolean. No flag or engagement value is consent
// to contact the address.
type PersonSearchEmail struct {
	EmailAddress    string                      `json:"emailAddress,omitempty"`
	EmailOrdinal    *int                        `json:"emailOrdinal,omitempty"`
	IsPremium       *bool                       `json:"isPremium,omitempty"`
	NonBusiness     *int                        `json:"nonBusiness,omitempty"`
	EmailEngagement PersonSearchEmailEngagement `json:"emailEngagementData,omitzero"`
	SourceSummary   json.RawMessage             `json:"sourceSummary,omitempty"`
}

// PersonSearchEmailEngagement is an email's emailEngagementData object: the
// provider's engagement data, uninterpreted. EngagementScore is a provider
// value, not a probability. Dates and times are provider text. Members the SDK
// does not know are ignored.
type PersonSearchEmailEngagement struct {
	IsBounce         *bool    `json:"isBounce,omitempty"`
	IsMatched        *bool    `json:"isMatched,omitempty"`
	IsGoodDomain     *bool    `json:"isGoodDomain,omitempty"`
	SendTime         *int     `json:"sendTime,omitempty"`
	EngagementScore  *int     `json:"engagementScore,omitempty"`
	Frequency        string   `json:"frequency,omitempty"`
	BestDayOfTheWeek string   `json:"bestDayOfTheWeek,omitempty"`
	BestTimeOfTheDay string   `json:"bestTimeOfTheDay,omitempty"`
	NAICS            []string `json:"naics,omitempty"`
	LastCheckDate    string   `json:"lastCheckDate,omitempty"`
	LastTouchDate    string   `json:"lastTouchDate,omitempty"`
}

// PersonSearchRelative is one entry of a person's relativesSummary list: the
// provider's summary of another person it relates to the candidate.
//
// RelativeType, RelativeLevel and Spouse are the provider's labels and
// values; the SDK infers no relationship. Score is a provider value, not a
// probability. The flags are the provider's, nil when not reported.
type PersonSearchRelative struct {
	TahoeID            string   `json:"tahoeId,omitempty"`
	Prefix             string   `json:"prefix,omitempty"`
	FirstName          string   `json:"firstName,omitempty"`
	MiddleName         string   `json:"middleName,omitempty"`
	LastName           string   `json:"lastName,omitempty"`
	Suffix             string   `json:"suffix,omitempty"`
	DOB                string   `json:"dob,omitempty"`
	RelativeLevel      string   `json:"relativeLevel,omitempty"`
	RelativeType       string   `json:"relativeType,omitempty"`
	City               string   `json:"city,omitempty"`
	State              string   `json:"state,omitempty"`
	Spouse             *int     `json:"spouse,omitempty"`
	Score              *int     `json:"score,omitempty"`
	SharedHouseholdIDs []string `json:"sharedHouseholdIds,omitempty"`
	IsPrivate          *bool    `json:"isPrivate,omitempty"`
	IsOptedOut         *bool    `json:"isOptedOut,omitempty"`
	IsDeceased         *bool    `json:"isDeceased,omitempty"`
}

// PersonSearchAssociate is one entry of a person's associatesSummary list: the
// provider's summary of another person it associates with the candidate.
// Score is a provider value, not a probability.
type PersonSearchAssociate struct {
	TahoeID    string `json:"tahoeId,omitempty"`
	Prefix     string `json:"prefix,omitempty"`
	FirstName  string `json:"firstName,omitempty"`
	MiddleName string `json:"middleName,omitempty"`
	LastName   string `json:"lastName,omitempty"`
	Suffix     string `json:"suffix,omitempty"`
	DOB        string `json:"dob,omitempty"`
	Score      *int   `json:"score,omitempty"`
}

// PersonSearchIndicators is a person's indicators object, returned with the
// Indicators include: the provider's integers for whether, or how many,
// records of each kind it holds about the person. They are uninterpreted: the
// documentation shows only 0 and 1, while counts above 1 have been observed.
// Each member is nil when not reported. Members the SDK does not know are
// ignored.
type PersonSearchIndicators struct {
	HasBankruptcyRecords           *int `json:"hasBankruptcyRecords,omitempty"`
	HasBusinessRecords             *int `json:"hasBusinessRecords,omitempty"`
	HasDivorceRecords              *int `json:"hasDivorceRecords,omitempty"`
	HasDomainsRecords              *int `json:"hasDomainsRecords,omitempty"`
	HasEvictionsRecords            *int `json:"hasEvictionsRecords,omitempty"`
	HasFEINRecords                 *int `json:"hasFeinRecords,omitempty"`
	HasForeclosuresRecords         *int `json:"hasForeclosuresRecords,omitempty"`
	HasJudgmentRecords             *int `json:"hasJudgmentRecords,omitempty"`
	HasLienRecords                 *int `json:"hasLienRecords,omitempty"`
	HasMarriageRecords             *int `json:"hasMarriageRecords,omitempty"`
	HasProfessionalLicenseRecords  *int `json:"hasProfessionalLicenseRecords,omitempty"`
	HasPropertyRecords             *int `json:"hasPropertyRecords,omitempty"`
	HasVehicleRegistrationsRecords *int `json:"hasVehicleRegistrationsRecords,omitempty"`
	HasWorkplaceRecords            *int `json:"hasWorkplaceRecords,omitempty"`
	HasDEARecords                  *int `json:"hasDeaRecords,omitempty"`

	// The documentation lists the indicators above; these have been observed
	// as well.
	HasPropertyV2Records     *int `json:"hasPropertyV2Records,omitempty"`
	HasForeclosuresV2Records *int `json:"hasForeclosuresV2Records,omitempty"`
	HasUCCRecords            *int `json:"hasUccRecords,omitempty"`
	HasDebtRecords           *int `json:"hasDebtRecords,omitempty"`
}
