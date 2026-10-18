<div align="center">

# 🔎&nbsp;&nbsp;go-enformion

**A small, standard-library-only Go SDK for the EnformionGO developer APIs.**

<br/>

<a href="https://github.com/mrz1836/go-enformion/releases"><img src="https://img.shields.io/github/release-pre/mrz1836/go-enformion.svg?include_prereleases&style=flat-square&logo=github&color=black" alt="Release"></a>
<a href="https://golang.org/"><img src="https://img.shields.io/github/go-mod/go-version/mrz1836/go-enformion?style=flat-square&logo=go&color=00ADD8" alt="Go Version"></a>
<a href="https://github.com/mrz1836/go-enformion/blob/master/LICENSE"><img src="https://img.shields.io/github/license/mrz1836/go-enformion.svg?style=flat-square&color=blue" alt="License"></a>

<br/>

<table align="center" border="0">
  <tr>
    <td align="right">
       <code>CI / CD</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://github.com/mrz1836/go-enformion/actions"><img src="https://img.shields.io/github/actions/workflow/status/mrz1836/go-enformion/fortress.yml?branch=master&label=build&logo=github&style=flat-square" alt="Build"></a>
       <a href="https://github.com/mrz1836/go-enformion/actions"><img src="https://img.shields.io/github/last-commit/mrz1836/go-enformion?style=flat-square&logo=git&logoColor=white&label=last%20update" alt="Last Commit"></a>
    </td>
    <td align="right">
       &nbsp;&nbsp;&nbsp;&nbsp; <code>Quality</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://codecov.io/gh/mrz1836/go-enformion"><img src="https://codecov.io/gh/mrz1836/go-enformion/branch/master/graph/badge.svg?style=flat-square" alt="Coverage"></a>
    </td>
  </tr>

  <tr>
    <td align="right">
       <code>Security</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://scorecard.dev/viewer/?uri=github.com/mrz1836/go-enformion"><img src="https://api.scorecard.dev/projects/github.com/mrz1836/go-enformion/badge?style=flat-square" alt="Scorecard"></a>
       <a href=".github/SECURITY.md"><img src="https://img.shields.io/badge/policy-active-success?style=flat-square&logo=security&logoColor=white" alt="Security"></a>
    </td>
    <td align="right">
       &nbsp;&nbsp;&nbsp;&nbsp; <code>Community</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://github.com/mrz1836/go-enformion/graphs/contributors"><img src="https://img.shields.io/github/contributors/mrz1836/go-enformion?style=flat-square&color=orange" alt="Contributors"></a>
       <a href="https://mrz1818.com/"><img src="https://img.shields.io/badge/donate-bitcoin-ff9900?style=flat-square&logo=bitcoin" alt="Bitcoin"></a>
    </td>
  </tr>
</table>

</div>

<br/>
<br/>

<div align="center">

### <code>Project Navigation</code>

</div>

<table align="center">
  <tr>
    <td align="center" width="20%">
       🚀&nbsp;<a href="#-installation"><code>Installation</code></a>
    </td>
    <td align="center" width="20%">
       🏁&nbsp;<a href="#-quick-start"><code>Quick&nbsp;Start</code></a>
    </td>
    <td align="center" width="20%">
       💡&nbsp;<a href="#-usage"><code>Usage</code></a>
    </td>
    <td align="center" width="20%">
       ⚙️&nbsp;<a href="#%EF%B8%8F-behavior"><code>Behavior</code></a>
    </td>
    <td align="center" width="20%">
       📚&nbsp;<a href="#-documentation"><code>Documentation</code></a>
    </td>
  </tr>
  <tr>
    <td align="center">
       🧪&nbsp;<a href="#-examples--tests"><code>Examples&nbsp;&&nbsp;Tests</code></a>
    </td>
    <td align="center">
      ⚡&nbsp;<a href="#-benchmarks"><code>Benchmarks</code></a>
    </td>
    <td align="center">
      🛠️&nbsp;<a href="#%EF%B8%8F-code-standards"><code>Code&nbsp;Standards</code></a>
    </td>
    <td align="center">
      🤖&nbsp;<a href="#-ai-usage--assistant-guidelines"><code>AI&nbsp;Usage</code></a>
    </td>
    <td align="center">
       🤝&nbsp;<a href="#-contributing"><code>Contributing</code></a>
    </td>
  </tr>
</table>
<br/>

go-enformion is a Go SDK for the [EnformionGO](https://go.enformion.com/developer-apis/) developer APIs
(Enformion, LLC). Every API it supports shares one deliberately narrow and strict core: one HTTP attempt
per call, no retries or redirects, bounded reads, strict JSON decoding, and errors that never contain
credentials, personal data or provider text.

Two APIs are supported today:

- **Contact Enrichment** (`POST https://devapi.enformion.com/Contact/Enrich`, search type
  `DevAPIContactEnrich`). Give it a subject with at least two of name, phone, email and address, and it
  returns the provider's top person match (name, age, addresses, phones and emails).
- **Person Search** (`POST https://devapi.enformion.com/PersonSearch`, in one of four documented search
  modes). Give it a mode and at least one criterion, and it returns one page of the provider's candidate
  persons, with their names, addresses, phones, emails, relatives, associates and indicators.

Both return typed Go values, with every field optional and nothing invented. The rest of the EnformionGO
API is planned for future releases: see [Supported APIs](#-supported-apis).

<br/>

## 🧭 Supported APIs

go-enformion adds the EnformionGO APIs one at a time. Each arrives as its own method on `Client`, with its
own request and result types, without breaking existing callers. The list follows the provider's
[API reference](https://enformiongo.readme.io/reference/overview); <sup>PRO</sup> marks the endpoints it
labels as Pro.

| Category | API | Status |
|---|---|---|
| **Single result** | [Contact Enrichment](https://enformiongo.readme.io/reference/contact-enrichment) | Available: `EnrichContact` |
| | [Contact Enrich Plus](https://enformiongo.readme.io/reference/contact-enrich-plus) | Planned |
| | [Caller ID](https://enformiongo.readme.io/reference/caller-id) | Planned |
| | [Caller ID Plus](https://enformiongo.readme.io/reference/caller-id-plus) | Planned |
| | [Email ID](https://enformiongo.readme.io/reference/email-id) | Planned |
| | [Email ID Enrich Plus](https://enformiongo.readme.io/reference/email-id-enrich-plus) | Planned |
| | [Contact ID](https://enformiongo.readme.io/reference/contact-id) | Planned |
| | [Contact ID Plus](https://enformiongo.readme.io/reference/contact-id-plus) | Planned |
| | [Address ID](https://enformiongo.readme.io/reference/address-id) | Planned |
| | [Address ID Plus](https://enformiongo.readme.io/reference/address-id-plus) | Planned |
| | [Address AutoComplete](https://enformiongo.readme.io/reference/address-autocomplete) | Planned |
| **Lead verification** | [ELeadVerify](https://enformiongo.readme.io/reference/eleadverify) | Planned |
| **People data** | [Person Search](https://enformiongo.readme.io/reference/person-search) | Available: `SearchPersons` |
| | [ID Verification](https://enformiongo.readme.io/reference/id-verification) | Planned |
| | [Census Search](https://enformiongo.readme.io/reference/census) | Planned |
| | [Debt Search V2](https://enformiongo.readme.io/reference/debt-v2) <sup>PRO</sup> | Planned |
| | [Divorce Search](https://enformiongo.readme.io/reference/divorce) | Planned |
| | [Marriage Search](https://enformiongo.readme.io/reference/marriage) <sup>PRO</sup> | Planned |
| | [Reverse Phone Search](https://enformiongo.readme.io/reference/reverse-phone) | Planned |
| | [LinkedIn ID](https://enformiongo.readme.io/reference/linkedin-id) | Planned |
| **Business data** | [Business Search](https://enformiongo.readme.io/reference/business) | Planned |
| | [Business ID](https://enformiongo.readme.io/reference/business-id) | Planned |
| | [Domain Search](https://enformiongo.readme.io/reference/domain) | Planned |
| | [Workplace Search](https://enformiongo.readme.io/reference/workplace) | Planned |
| | [Workplace drilldown lookups](https://enformiongo.readme.io/reference/get-states) (states, cities, industries, levels, departments, job titles) | Planned |
| **Property data** | [Property Search V2](https://enformiongo.readme.io/reference/property-v2) | Planned |
| **Court records** | [Criminal Search V2](https://enformiongo.readme.io/reference/criminal-search-v2) <sup>PRO</sup> | Planned |
| | [Eviction Search](https://enformiongo.readme.io/reference/eviction) <sup>PRO</sup> | Planned |
| | [Pre-Foreclosure Search](https://enformiongo.readme.io/reference/pre-foreclosure) <sup>PRO</sup> | Planned |
| **Asset data** | [OFAC Search](https://enformiongo.readme.io/reference/ofac) <sup>PRO</sup> | Planned |
| | [Vehicle Ownership](https://enformiongo.readme.io/reference/vehicle-ownership) <sup>PRO</sup> | Planned |
| **License data** | [DEA Search](https://enformiongo.readme.io/reference/dea) <sup>PRO</sup> | Planned |
| | [Professional License Search](https://enformiongo.readme.io/reference/professional-license) <sup>PRO</sup> | Planned |
| **Data alerts** | [Data Alerts](https://enformiongo.readme.io/reference/data-alerts) (subscriptions, alerts and push notifications) | Planned |

Each new API is a new billable surface with its own data, terms and contract, so each is added only after
its own review; see [docs/adding-an-operation.md](docs/adding-an-operation.md).

<br/>

## 📦 Installation

**go-enformion** requires a [supported release of Go](https://golang.org/doc/devel/release.html#policy) (Go 1.26 or newer).
```shell script
go get github.com/mrz1836/go-enformion
```

The library has **zero third-party runtime dependencies**: it imports only the standard library.
`testify` is used only by the tests and their internal helper package.

<br/>

### Credentials

EnformionGO authenticates with an API key **name** and **password**, copied from the **Keys** tab of the
dashboard at [api.enformion.com](https://api.enformion.com). The SDK sends them as the `galaxy-ap-name` and
`galaxy-ap-password` headers. It never reads credentials from environment variables or files: your application decides
where the secrets come from and passes them to `NewClient`. Keeping them secret in your own configuration
and logs is your application's job.

<br/>

## 🏁 Quick Start

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/mrz1836/go-enformion"
)

func main() {
	// Your application chooses where the secrets come from.
	client, err := enformion.NewClient(loadKeyName(), loadKeyPassword(),
		enformion.WithTimeout(10*time.Second),
	)
	if err != nil {
		log.Fatal(err) // never quotes the secrets
	}

	// Two categories: name and email. This call is live and billable.
	res, err := client.EnrichContact(context.Background(), enformion.EnrichContactRequest{
		FirstName: "Avery",
		LastName:  "Example",
		Email:     "avery.example@example.com",
	})
	if errors.Is(err, enformion.ErrRateLimited) {
		log.Fatal("rate limited or monthly quota reached; the SDK never retries")
	}
	if err != nil {
		log.Fatal(err) // safe to log: kind, HTTP status and field path only
	}
	if res.Person == nil {
		fmt.Println("no person data returned (not proof that no match exists)")
		return
	}
	p := res.Person
	fmt.Println(p.Name.FirstName, p.Name.LastName, p.Age)
	for _, ph := range p.Phones {
		fmt.Println(ph.Number, ph.Type)
	}
}
```

<br/>

## 💡 Usage

### Configure a client

`NewClient(keyName, keyPassword string, opts ...Option)` validates everything up front and performs no I/O.
A `Client` is immutable and safe for concurrent use; create one and reuse it.

| Option | Default | Limits | Effect |
|---|---|---|---|
| `WithTimeout(d)` | 30s | `0 < d ≤ 5m` | Bounds each call, including reading the whole body. The context deadline applies too. |
| `WithMaxResponseBytes(n)` | 1 MiB for `EnrichContact`, 16 MiB for `SearchPersons` | 1 B to 16 MiB | Limits the (decompressed) response body. Once given, `n` applies to every operation. |
| `WithHTTPClient(hc)` | fresh transport | non-nil | Calls go through a **copy** of `hc` that refuses redirects and has no cookie jar; `hc` is never modified. |
| `WithTestBaseURL(url)` | production | `https` on `127.0.0.0/8` or `[::1]` | Points the client at a local test server. There is no other base URL option. |
| `WithProviderDiagnostics()` | off | | Keeps provider messages on errors, reachable only through `(*CallError).Diagnostics()`. |

Credentials must each be 1 to 1024 bytes of printable ASCII with no leading or trailing spaces. Invalid
arguments return a `*ValidationError` matching `ErrInvalidConfig` that names the argument, never its value.

<br/>

### Contact Enrichment

`EnrichContact` sends one subject and returns the provider's top person match.

<br/>

#### Build a request

```go
req := enformion.EnrichContactRequest{
	FirstName:    "Avery",
	MiddleName:   "Q",
	LastName:     "Example",
	DOB:          "06/01/1985",        // mm/dd/yyyy, zero-padded, a real date
	Age:          41,                  // 0 = not sent
	AddressLine1: "100 Sample Way",    // house number and street, or PO Box
	AddressLine2: "Exampleton, OH",    // city and state, state, or ZIP
	Phone:        "(312) 555-0142",
	Email:        "avery.example@example.com",
	SessionID:    "batch-7f3a9c-0001", // optional opaque logging ID, never personal data
}
```

The provider requires **at least two** of four categories. The SDK counts them as follows, and refuses a
request with fewer before sending anything:

| Category | Counts when |
|---|---|
| Name | `FirstName` **and** `LastName` are set. `MiddleName` alone, or a lone first or last name, is sent but does not count. |
| Phone | `Phone` is set. |
| Email | `Email` is set. |
| Address | `AddressLine1` **or** `AddressLine2` is set. |

`DOB` and `Age` are sent when set but are not categories. Values are sent verbatim, never trimmed or
rewritten. Every text field must be valid UTF-8 of at most 256 bytes, with no control characters and not
only whitespace; these are SDK limits, as the provider documents none. The phone format is not enforced,
because the provider's own examples disagree with its documented formats. `SessionID` must be empty or 1
to 128 bytes of visible ASCII.

<br/>

#### Read the result

```go
res, err := client.EnrichContact(ctx, req)
if err != nil {
	return err
}
if res.Person == nil {
	return nil // no person data in the response; NOT proof that no match exists
}
for _, ph := range res.Person.Phones {
	connected := "not reported"
	if ph.IsConnected != nil {
		connected = strconv.FormatBool(*ph.IsConnected) // a provider flag, not consent to contact
	}
	fmt.Println(ph.Number, ph.Type, connected, ph.LastReportedDate)
}
if res.IdentityScore != nil {
	fmt.Println("provider identityScore (uninterpreted):", *res.IdentityScore)
}
```

| Field | Type | Absent or null | Notes |
|---|---|---|---|
| `Person` | `*Person` | `nil` | `nil` also when the person object holds no data at all. |
| `IdentityScore` | `*int` | `nil` | An uninterpreted provider integer: not a probability or confidence. `0` is `&0`. |
| `RequestID` | `string` | `""` | Kept only when 1 to 128 visible ASCII bytes. |
| `Diagnostics` | `ProviderDiagnostics` | empty | The envelope's `message`, `error.inputErrors` and `error.warnings`, bounded. May echo personal data. |
| `Person.PersonID`, `Age`, `PartialDOB` | `string` | `""` | Provider text. `Age` is a string on the wire. |
| `Person.Name` | `Name` | zero | `FirstName`, `MiddleName`, `LastName`. |
| `Addresses`, `Phones`, `Emails` | slices | `nil` | Provider order, every entry kept; JSON `null` elements are not entries. More than 1,000 entries fails the call (an SDK safety bound; the provider documents at most five). |
| `Phone.IsConnected`, `Email.IsValidated`, `Email.IsBusiness` | `*bool` | `nil` | `nil` means not reported; `false` is `&false`. None is consent to contact. |

Dates are provider text (shown as `M/D/YYYY` without padding in the documentation's examples) and are never parsed. The SDK never
deduplicates, truncates, ranks, picks a "preferred" entry or infers relationships. The result is the
provider's top match, not identity verification.

Results marshal to JSON and back unchanged (`reflect.DeepEqual`); that JSON is this SDK's own
serialization, not the provider's wire format.

<br/>

### Person Search

`SearchPersons` sends one page request in one of the four documented search modes and returns that page
of the provider's candidate persons. The provider's terms apply to every search: see
[Personal data and permitted use](#%EF%B8%8F-behavior).

<br/>

#### Build a request

```go
req := enformion.PersonSearchRequest{
	Mode:      enformion.PersonSearchModePerson, // required: the search type header
	FirstName: "Avery",
	LastName:  "Example",
	Addresses: []enformion.PersonSearchRequestAddress{
		{AddressLine2: "Exampleton, OH"}, // city and state, state, or ZIP
	},
	Includes:       []enformion.PersonSearchInclude{enformion.PersonSearchIncludeAddresses},
	FilterOptions:  []enformion.PersonSearchFilter{enformion.PersonSearchFilterIncludeLowQualityAddresses},
	Page:           1,  // 0 = provider default
	ResultsPerPage: 10, // 0 = provider default
	SessionID:      "batch-7f3a9c-0001",
}
```

The mode is sent as the `galaxy-search-type` header, never in the body. The provider documents four:

| Mode | Search type | Documented as |
|---|---|---|
| `PersonSearchModePerson` | `Person` | Detailed results, mainly for "logged in or paying" users |
| `PersonSearchModeTeaser` | `Teaser` | For "logged out" users: "Most of the data elements in a teaser search are masked and/or limited by the quantity returned"; suggested as the initial search |
| `PersonSearchModeReversePhonePerson` | `ReversePhonePerson` | The same text as `Person`. A mode of Person Search, not the separate Reverse Phone Search API |
| `PersonSearchModeReversePhonePersonTeaser` | `ReversePhonePersonTeaser` | The same text as `Teaser` |

At least one criterion is required: a name part, an AKA, `DOB`, an age field, an address, `Email`, `Phone`,
`ClientIP`, a relative or a Tahoe ID. The SDK adds no rule the provider does not document: it does not
decide which criteria a mode needs (a reverse-phone mode does not require `Phone`) or which age field wins.
Every limit below is the SDK's own, as the provider documents none.

| Field | Sent as | Rules |
|---|---|---|
| `FirstName`, `MiddleName`, `LastName` | the same names | Text rules: valid UTF-8, at most 256 bytes, no control characters, not only whitespace |
| `AKAs`, `Relatives` | `Akas`, `Relatives`: lists of `{FirstName, MiddleName, LastName}` | At most 32 entries; an entry with every field empty is `blank`. The shape is unverified |
| `DOB` | `Dob` | `mm/dd/yyyy`, zero-padded, a real date |
| `Age`, `AgeRangeMinAge`, `AgeRangeMaxAge` | the same names | `0` (not sent) or 1 to 150; never checked against each other |
| `AgeRange` | `AgeRange` | `N-M`: one to three digits each, each at most 150 |
| `Addresses` | `Addresses`: a list of `{AddressLine1, AddressLine2, County}` | At most 32 entries; an entry with every field empty is `blank` |
| `Email`, `Phone` | the same names | Text rules; the phone format is not enforced |
| `ClientIP` | `ClientIp` | A canonical dotted IPv4 address, sent as a string (unverified: the provider types it as an integer) |
| `TahoeIDs` | `TahoeIds` | At most 32 IDs, none empty, such as a result's `TahoeID` |
| `FirstNameCharOffset`, `LastNameCharOffset` | the same names | `nil` (not sent) or 0 to 256; `new(0)` sends `0` |
| `DOBFormat` | `DobFormat` | Text rules, sent verbatim |
| `MaxAddressYears`, `MaxPhoneYears` | the same names | `nil` (not sent) or 0 to 150; `new(0)` sends `0` |
| `Includes`, `FilterOptions` | the same names | At most 32 tokens each, 1 to 128 ASCII letters and digits, sent verbatim, in order, duplicates kept |
| `Page`, `ResultsPerPage` | the same names | `0` (provider default) or from 1; `ResultsPerPage` at most 1,000 |
| `SessionID` | `galaxy-client-session-id` header | Empty, or 1 to 128 bytes of visible ASCII |

Includes and filters are enabled per account by the provider. Its includes page lists labels rather than
tokens, and only `Addresses` and `PhoneNumbers` appear as tokens in its documentation; the
`PersonSearchInclude` constants also cover `EmailAddresses`, `DatesOfBirth`, `Relatives`,
`RelativesSummary` and `Associates`, which have been observed to be accepted. The `PersonSearchFilter`
constants quote the provider's descriptions, several of which contradict the filter's name; the SDK sends
the token and does not interpret it. Any other alphanumeric token can be passed as
`PersonSearchInclude("...")` or `PersonSearchFilter("...")`.

A failure names the field and list index, never the value, such as
`enformion: invalid request: Addresses[1].County: control_character`. The new reasons are `unsupported`
(an unknown mode), `too_many` (a list over 32 entries) and `no_criteria`.

<br/>

#### Read the result

```go
page, err := client.SearchPersons(ctx, req)
if err != nil {
	return err
}
for _, p := range page.Persons { // the provider's order; duplicates kept
	fmt.Println(p.TahoeID, p.FullName)
	for _, ph := range p.PhoneNumbers {
		connected := "not reported"
		if ph.IsConnected != nil {
			connected = strconv.FormatBool(*ph.IsConnected) // a provider flag, not consent to contact
		}
		fmt.Println(" ", ph.PhoneNumber, ph.PhoneType, connected)
	}
}
if len(page.Persons) == 0 {
	// no candidates in the response; NOT proof that no match exists
}
```

| Field | Type | Absent or null | Notes |
|---|---|---|---|
| `Persons` | `[]PersonSearchPerson` | `nil` | The page's candidates, in the provider's order, duplicates kept. `null` or `[]` is a page without candidates; an envelope without `persons` fails as `ErrUnexpectedResponse`. |
| `Pagination` | `PersonSearchPagination` | zero | `CurrentPageNumber`, `ResultsPerPage`, `TotalPages`, `TotalResults`, each `*int`. The member names are inferred. |
| `Counts` | `PersonSearchCounts` | zero | The provider's `counts`: uninterpreted `*int` counts and `SearchResultsOverflow *bool`. |
| `RequestID` | `string` | `""` | Kept only when 1 to 128 visible ASCII bytes. |
| `CriteriaType`, `SmartSearchStatistics` | `json.RawMessage` | `nil` | Undocumented structure, kept raw. May echo the request's personal data. |
| `Diagnostics` | `ProviderDiagnostics` | empty | As for Contact Enrichment. |
| `PersonSearchPerson.TahoeID`, `FullName`, `DOB`, `DOD`, dates | `string` | `""` | Provider text, never parsed; teaser modes may mask it. `IsCurrentPropertyOwner` is text too. |
| `Age`, `SparseFlag` | `*int` | `nil` | `0` is `&0`. |
| `IsPublic`, `IsPremium`, `IsOptedOut`, `IsEnterpriseOptedOut`, `HasAdditionalData` | `*bool` | `nil` | Provider flags, not consent or eligibility. |
| `Name`, `AKAs`, `MergedNames` | `PersonSearchName` | zero, `nil` | Prefix, name parts, suffix, `RawNames`, first-seen dates and a raw `SourceSummary`. |
| `Addresses`, `PhoneNumbers`, `EmailAddresses`, `Locations`, `DatesOfBirth` | slices | `nil` | Every entry kept; more than 1,000 entries fails the call. Coordinates, ZIP codes and hashes are text. |
| `RelativesSummary`, `AssociatesSummary` | slices | `nil` | The provider's summaries of other people. Its labels (`RelativeType`) and `Score` are its own; the SDK infers no relationship. |
| `Associates` | `[]PersonSearchPerson` | `nil` | Full associated persons (the `Associates` include), decoded one level deep. |
| `Indicators` | `PersonSearchIndicators` | zero | Nineteen `*int` record indicators, uninterpreted: counts above 1 have been observed. |
| `Extra` | `map[string]json.RawMessage` | `nil` | Every person member the SDK does not decode, by its key as sent: see [raw and ignored members](#%EF%B8%8F-behavior). |

The SDK never selects, ranks, merges or deduplicates candidates or entries, and never picks a "best" match.
A candidate is the provider's observation, not identity verification. Results marshal to JSON and back
unchanged (`reflect.DeepEqual`), in this SDK's own serialization.

<br/>

#### Pages

Each call returns one page. The SDK never fetches the next one: ask for it explicitly, knowing that **each
page is a separate call that may be billed**.

```go
req.Page = 2
next, err := client.SearchPersons(ctx, req) // a second, separately billable call
```

<br/>

### Handle errors

Validation failures are `*ValidationError` values, and **nothing was sent**. Every other failure is a
`*CallError` whose `Kind` matches one target with `errors.Is` (a `timeout` matches two):

| Kind | Matches | When |
|---|---|---|
| `unauthorized` | `ErrUnauthorized` | HTTP 401 or 403 |
| `rate_limited` | `ErrRateLimited` | HTTP 429 (rate limit or monthly quota). The SDK never retries. |
| `rejected` | `ErrRejected` | Another 4xx (a 404 is never a "no match"), or HTTP 200 with `isError: true` |
| `upstream` | `ErrUpstream` | HTTP 5xx |
| `unexpected_response` | `ErrUnexpectedResponse` | 1xx, 3xx (never followed), another 2xx, 600 and above, a body that is not an object, a missing or non-boolean `isError`, a Person Search body without `persons`, or a person or one of its record entries (a name, address, phone, email and, for Person Search, a location, relative, associate, date of birth or death record) whose only data is under unrecognized keys |
| `malformed_response` | `ErrMalformedResponse` | Invalid UTF-8 or JSON, trailing data, a repeated key, or a recognized field of the wrong type |
| `response_too_large` | `ErrResponseTooLarge` | An HTTP 200 body over the limit, a decoded list of more than 1,000 entries, or a Person Search body that would decode to more than four times its size plus 1 MiB |
| `transport` | `ErrTransport` | DNS, connection, TLS, proxy, reset or truncated body |
| `timeout` | `ErrTimeout` **and** `context.DeadlineExceeded` | The SDK's timeout, an `http.Client` timeout, or a network timeout |
| `canceled` | `context.Canceled` | The caller's context was canceled |
| `deadline_exceeded` | `context.DeadlineExceeded` | The caller's context deadline expired |

```go
var (
	ve *enformion.ValidationError
	ce *enformion.CallError
)
switch {
case errors.As(err, &ve):
	// not sent: fix the request (ve.Field, ve.Reason)
case errors.Is(err, enformion.ErrTimeout): // check before context.DeadlineExceeded
case errors.Is(err, context.DeadlineExceeded):
case errors.Is(err, enformion.ErrRateLimited):
case errors.Is(err, enformion.ErrRejected):
	if errors.As(err, &ce) {
		_ = ce.Diagnostics() // only with WithProviderDiagnostics; may echo personal data
	}
}
```

`Error()` holds only the kind, the HTTP status and a JSON field path, such as
`enformion: malformed_response (HTTP 200): person.phones[1].isConnected`, so errors are safe to log.
`CallError` exposes `Kind`, `StatusCode`, `RequestID` and `Field`; tests may build one from those fields
and it matches `errors.Is` like a real one.

<br/>

### In a net/http handler

```go
func (s *Server) HandleEnrich(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	res, err := s.Enricher.EnrichContact(ctx, requestFrom(r))
	if err != nil {
		s.Log.WarnContext(ctx, "contact enrichment failed", "error", err) // PII-free
		http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
		return
	}
	if res.Person == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	_ = json.NewEncoder(w).Encode(res.Person)
}
```

<br/>

### Testing your application

Depend on a small interface you own, and fake it in tests:

```go
type ContactEnricher interface {
	EnrichContact(context.Context, enformion.EnrichContactRequest) (*enformion.EnrichContactResult, error)
}

var _ ContactEnricher = (*enformion.Client)(nil)

type PersonSearcher interface {
	SearchPersons(context.Context, enformion.PersonSearchRequest) (*enformion.PersonSearchResult, error)
}

var _ PersonSearcher = (*enformion.Client)(nil)

fake := &fakeEnricher{err: &enformion.CallError{Kind: enformion.FailureRateLimited, StatusCode: 429}}
// errors.Is(err, enformion.ErrRateLimited) is true for the fake's error too.
```

To exercise the real client offline, serve synthetic responses from `httptest.NewTLSServer` and point the
client at it with `WithTestBaseURL(srv.URL)` and `WithHTTPClient(srv.Client())`. The package's runnable
examples do exactly that.

<br/>

## ⚙️ Behavior

<details>
<summary><strong><code>Billing, retries and timeouts</code></strong></summary>
<br/>

- `devapi.enformion.com` is the only server the provider documents. **Every call is live**, and the
  provider's [pricing](https://go.enformion.com/pricing/) and help pages say it bills per successful match.
  There is no documented sandbox.
- Each call makes **at most one** HTTP attempt. The SDK never retries, follows a redirect, polls,
  hedges or makes a follow-on call, and it disables the standard library's own resends (`GetBody` is nil,
  so neither HTTP/1 nor HTTP/2 can replay the POST, and no idempotency header is sent).
- Invalid requests, and calls whose context is already done, send nothing.
- After a timeout, cancellation, transport failure, oversized body or unreadable response, the provider
  **may already have processed and billed** the request. The SDK cannot tell, and makes no claim about
  what was or was not billed. Repeating a call may be billed again.
- An injected `http.Client`'s transport is trusted: it sees the credential headers and the request's
  personal data, and it may retry on its own.
- An `httptrace.ClientTrace` on the context still runs, but its `WroteHeaderField` hook never receives
  the `galaxy-ap-name` or `galaxy-ap-password` headers, so tracing libraries that record request headers
  cannot capture the credentials. The standard library's own debug logging is outside the SDK's control:
  `GODEBUG=http2debug=2` logs every request header, credentials included, so never enable it with real
  credentials.
- The default transport honors `HTTPS_PROXY` and `NO_PROXY`, as Go's default transport does. To bypass
  proxies, inject a client whose transport sets no `Proxy`.

</details>

<details>
<summary><strong><code>Personal data and permitted use</code></strong></summary>
<br/>

Requests and results hold personal data. The library never logs, prints, stores or caches anything and
never reads credentials or configuration from the environment, but its request and result types have no
redacting formatter: **access
control, logging, retention, security and permitted use are your application's responsibility**.
Serializing or hashing personal data does not anonymize it. Error strings never contain personal data;
provider diagnostics on errors are off unless you opt in. Person Search results also describe other people
(relatives, associates and neighbors) and keep undocumented members as raw JSON, which prints in full.

A result is the provider's observation, **not identity verification**: no match, candidate, label, score,
indicator or flag establishes anyone's identity, consent or eligibility for any purpose.

The provider's [terms](https://go.enformion.com/terms/) restrict use of its data. In summary, and with no
claim of legal suitability (read the current terms yourself):

- It is **not a consumer reporting agency** under the FCRA. Its data may not be used as a factor in
  eligibility for credit, insurance, employment, housing or any other FCRA purpose.
- Results may not be stored or cached to make additional inquiries, merged with other data, scraped or
  resold.
- Minors, celebrities and public figures may not be searched.
- Compliance with CAN-SPAM, the TCPA and Do Not Call rules is yours: the provider scrubs no numbers.
  `IsConnected`, `IsValidated` and `IsBusiness` are provider flags, not consent to contact anyone.

</details>

<details>
<summary><strong><code>Contact Enrichment: when no person comes back</code></strong></summary>
<br/>

The provider does not document what a "no match" looks like. When an HTTP 200 response with
`isError: false` has no `person`, a `null` one, or one with no data at all, the result's `Person` is `nil`.
That means only that the response carried no person data: **it is not a verified no-match**, and the SDK
returns no `ErrNoMatch`. A person object, its name, or any address, phone or email entry whose only data
sits under keys the SDK does not recognize (as after a field rename) fails the call as
`ErrUnexpectedResponse` instead, so a billed match never silently comes back empty. An HTTP 404 is a rejection, never a "no match".

</details>

<details>
<summary><strong><code>Contact Enrichment: response members the SDK ignores</code></strong></summary>
<br/>

These members of the response are never decoded or type-checked, so a change in their contents cannot fail
a call. The whole-body checks (valid UTF-8 and JSON, no repeated keys) still cover them, and a person whose
only data is under `relatives` fails as `ErrUnexpectedResponse`, like any person without recognized data.
Any of them could be added later without breaking callers.

| Member | Why it is excluded |
|---|---|
| `person.relatives` | Third-party personal data, outside subject contact enrichment |
| `searchCriteria` | Echoes the request's personal data |
| `pagination` | No paging contract; the documented sample reports `totalResults: 0` beside a match |
| `databaseQueryInfo` | Undocumented internals |
| `requestType`, `totalRequestExecutionTimeMs` | Diagnostics with no documented use |

</details>

<details>
<summary><strong><code>Contact Enrichment: limits and unverified assumptions</code></strong></summary>
<br/>

Built from the provider's Contact Enrichment documentation (readme.io docs version 1.4), which leaves
these open. None has been checked against the live API:

| Open question | What the SDK does until it is resolved |
|---|---|
| What a no-match response looks like | `Person == nil` for no person data; never declared a no-match |
| Whether a validation failure is HTTP 400 `{}` or HTTP 200 with `isError: true`, and what `inputErrors` and `warnings` entries look like | Both are `rejected`; entries are kept as raw JSON |
| Authentication failure status and body | 401 and 403 are `unauthorized`; anything else is classified by status |
| Rate and quota signals, `Retry-After` | 429 is `rate_limited`; no retry hint is exposed |
| Address key casing (`addressLine1` in every valid example, `AddressLine1` in the properties table) | Sends `addressLine1` and `addressLine2` |
| Which envelope and person fields the standard tier returns (`requestId`, `personId`, `partialDob`) | Every field is optional |
| What counts as a billable match | No billing claim is made |
| Header strictness | Always sends `galaxy-ap-name`, `galaxy-ap-password` and `galaxy-search-type` |
| Length limits | SDK limits only: 256 bytes per text field, 128 for `SessionID`, 1,000 entries per person list |

The documentation's own 200 example is not valid JSON. The test fixtures were written from scratch to its
shape with synthetic values. An opt-in live probe, run only by the maintainer, exists to settle the
no-match, validation-failure, authentication-failure and address-casing questions (none probes rate
limits); see [Examples & Tests](#-examples--tests).

</details>

<details>
<summary><strong><code>Person Search: pages, modes and billing</code></strong></summary>
<br/>

- Each `SearchPersons` call is **one live, potentially billable call for one page**. The provider documents
  no Person Search pricing, so the SDK makes no billing claim beyond that.
- The SDK never fetches a next page, never upgrades a `Teaser` search to `Person`, never retries a page and
  never makes a follow-up call, such as a search by a returned `TahoeID`. Each of those is a separate call
  your application chooses to make.
- A page without candidates means only that the response carried none: **it is not a verified no-match**.
  A refusal (a 4xx, or `isError: true`) is an error, never an empty page, and never returns candidates.
- Person Search's response limit defaults to **16 MiB**, the maximum, because a page can hold many people:
  one person with every include, associates among them, can be a few hundred kilobytes. A larger body fails
  with `ErrResponseTooLarge` **after** the call may have been billed, so size `ResultsPerPage` and
  `Includes` to fit. `WithMaxResponseBytes`, once given, sets the limit for every operation.
- The SDK decodes candidates in the provider's order and keeps duplicates. It never selects, ranks, merges
  or deduplicates them.

</details>

<details>
<summary><strong><code>Person Search: raw and ignored members</code></strong></summary>
<br/>

Members whose structure the provider does not document are kept as **raw JSON** rather than guessed at:

| Member | Kept as |
|---|---|
| `sourceSummary` (names, addresses, phones, emails, dates of birth, death records) | `SourceSummary json.RawMessage` |
| `datesOfDeath` entries | `DatesOfDeath []json.RawMessage` |
| an address's `neighbors` and `neighborSummaryRecords` entries | `[]json.RawMessage` |
| `criteriaType`, `smartSearchStatistics` | `json.RawMessage` on the result |
| **every other person member**, such as `driversLicenseDetail` or `relatives`, and an associate's own `associates` | `PersonSearchPerson.Extra`, keyed as sent |

Raw values are copies, never aliasing the response, normalized as `encoding/json` marshals a
`json.RawMessage` (compact, with `<`, `>`, `&`, U+2028 and U+2029 escaped, and unpaired surrogate escapes
replaced), so results round-trip unchanged. JSON `null` is `nil`, and `null` person members are left out
of `Extra`.

They are opaque **personal data, often about other people**: the SDK never types or interprets them, and
printing one prints all of it. They also cost memory. The response limit bounds the body, and a decode
budget of four times the body's size plus 1 MiB bounds what the body may decode to, so that a hostile
response of nested empty lists fails with `ErrResponseTooLarge` instead of costing a hundred times its size.
Real pages decode to less than their own size.

A record whose only data is under keys the SDK does not decode (a person, a name, an address, a phone, an
email, a location, a relative, an associate, a date of birth or a death record) fails as
`ErrUnexpectedResponse`, so a renamed field never makes a billed page come back silently empty. A person
keeps its unknown members in `Extra`, but still fails when none of its members is recognized. The flag and
metadata objects (`pagination`, `counts`, `indicators`, `highRiskMarker`, `emailEngagementData`) ignore
members they do not know, so a new indicator never fails a page.

These envelope members are never decoded or type-checked: `searchCriteria` (it echoes the request's personal
data), `databaseQueryInfo`, `requestType`, `requestTime` and `totalRequestExecutionTimeMs`. The whole-body
checks (valid UTF-8 and JSON, no repeated keys) still cover them.

</details>

<details>
<summary><strong><code>Person Search: limits and unverified assumptions</code></strong></summary>
<br/>

Built from the provider's Person Search documentation (readme.io docs version 1.4) and an observed response
structure (key names and types only). The documentation leaves these open, and none has been checked
against the live API by this SDK's tests:

| Open question | What the SDK does until it is resolved |
|---|---|
| The response envelope: the documentation's only example is one PascalCase person, without `persons`, `pagination`, `isError` or `requestId` | Follows the observed camelCase envelope; keys match without regard to ASCII case, so the documented PascalCase decodes too |
| The `pagination` member names, observed only as `{}` | Decodes `currentPageNumber`, `resultsPerPage`, `totalPages` and `totalResults`, as the provider's other endpoints name them |
| The shape of `Akas` and `Relatives` (documented as an undefined `Name{}`) | Sends lists of `{FirstName, MiddleName, LastName}` |
| `ClientIp`, typed as an integer with a dotted format | Sends a canonical dotted IPv4 string |
| `TahoeIds` (documented) or a single `TahoeId` | Sends the `TahoeIds` list |
| Include tokens: the includes page lists labels such as "Workplace Summary" | Constants for the two documented and five observed tokens; any other token is sent as given |
| Filters whose descriptions contradict their names | Sent verbatim; the SDK never interprets them |
| What `Teaser` and the reverse-phone modes mask or limit, and how | Every value is kept verbatim; nothing is inferred from a mask |
| The types of a relative's `score` and flags | Integers and booleans; another type fails as `ErrMalformedResponse` |
| Whether a validation failure is HTTP 400 `{}` or HTTP 200 with `isError: true` | Both are `rejected` |
| What, and how much, is billed | No billing claim is made |
| Length limits | SDK limits only: 32 entries per request list, 128-byte tokens, 1,000 entries per page and per response list, and the decode budget |

The test fixtures were written from scratch to these shapes with synthetic values. The opt-in live probe's
`search-nomatch` and `search-validation` probes exist to settle the envelope, pagination and
validation-failure questions; see [Examples & Tests](#-examples--tests).

</details>

<details>
<summary><strong><code>Scope and extension</code></strong></summary>
<br/>

Contact Enrichment and Person Search are implemented today; every other API in
[Supported APIs](#-supported-apis) is planned, and none is stubbed. The SDK has no automatic paging and no
batch requests. Each arrives as a new method on `Client` with its own request and result types,
added without breaking existing callers; see [docs/adding-an-operation.md](docs/adding-an-operation.md).

</details>

<br/>

## 📚 Documentation

View the generated [documentation](https://pkg.go.dev/github.com/mrz1836/go-enformion?tab=doc) and the
provider's [API reference](https://enformiongo.readme.io/reference/overview).

<details>
<summary><strong><code>Development Setup (Getting Started)</code></strong></summary>
<br/>

Install [MAGE-X](https://github.com/mrz1836/mage-x) build tool for development:

```bash
# Install MAGE-X for development and building
go install github.com/mrz1836/mage-x/cmd/magex@latest
magex update:install
```
</details>

<details>
<summary><strong><code>Library Deployment</code></strong></summary>
<br/>

This project uses [goreleaser](https://github.com/goreleaser/goreleaser) for streamlined binary and library deployment to GitHub. To get started, install it via:

```bash
brew install goreleaser
```

The release process is defined in the [.goreleaser.yml](.goreleaser.yml) configuration file.

Then create and push a new Git tag using:

```bash
magex version:bump bump=patch push=true branch=master
```

</details>

<details>
<summary><strong><code>Build Commands</code></strong></summary>
<br/>

View all build commands

```bash script
magex help
```

</details>

<details>
<summary><strong><code>GitHub Workflows</code></strong></summary>
<br/>

All workflows are driven by modular configuration in [`.github/env/`](.github/env/README.md) — no YAML editing required.

**[View all workflows and the control center →](.github/docs/workflows.md)**

</details>

<details>
<summary><strong><code>Updating Dependencies</code></strong></summary>
<br/>

To update all dependencies (Go modules, linters, and related tools), run:

```bash
magex deps:update
```

</details>

<br/>

## 🧪 Examples & Tests

Unit tests and examples run via [GitHub Actions](https://github.com/mrz1836/go-enformion/actions) on Go 1.26.x (the supported baseline) and the current Go release; fuzz tests run on the baseline. View the [configuration file](.github/workflows/fortress.yml).

Run all tests (fast):

```bash script
magex test
```

Run all tests with race detector (slower):
```bash script
magex test:race
```

Run the fuzz tests briefly:
```bash script
magex test:fuzz time=5s
```

Unit tests never leave the machine: they use in-memory transports and loopback TLS servers with synthetic
data. They prove, among other things, the exact wire request, the full status table, zero calls for
invalid input, exactly one attempt on every failure (including HTTP/2 stream resets), that redirects never
reach their target, bounded reads, timeouts during the body read, safe concurrent use across operations,
that hostile Person Search bodies fail on the decode budget instead of costing many times their size, and
that credentials and personal data never appear in errors or formatted output.

The runnable `Example*` functions in [example_test.go](example_test.go) run offline against a local TLS
server.

<br/>

### Live contract probe

An opt-in probe in [live_probe_internal_test.go](live_probe_internal_test.go) makes **real, potentially
billable** calls to settle the open questions above. It is run only by the maintainer, by hand. It needs
the `live` build tag and these variables:

| Variable | Effect |
|---|---|
| `ENFORMION_LIVE=1` | Enables the probe; without it the probe skips. |
| `ENFORMION_LIVE_ACK_BILLING=I-understand` | Acknowledges that calls may be billed; without it the probe skips. |
| `CI` | Must be unset; when set, the probe skips. |
| `ENFORMION_LIVE_PROBES` | Comma-separated probes. Contact Enrichment: `nomatch`, `validation` and `auth` take 1 call each, `casing` takes 2. Person Search: `search-nomatch` (a `Teaser` search for an invented subject, one result at most) and `search-validation` (an empty `{}` body) take 1 call each. |
| `ENFORMION_AP_NAME`, `ENFORMION_AP_PASSWORD` | Credentials, read only by the test harness. |

Once enabled, the probe fails before sending anything when `ENFORMION_LIVE_PROBES` names no known probe,
when the chosen probes need more than 4 calls (all six together need 7), or when the credentials are
invalid.

```bash
go test -tags live -count=1 -p 1 -run TestLiveProbe -v .
```

The `magex` test targets and CI compile the probe with the `live` tag too, and it skips there unless
`ENFORMION_LIVE` and `ENFORMION_LIVE_ACK_BILLING` are set, so set them only for the run.

A run makes at most 4 calls, never retries, and stops at the first HTTP 429. It prints only the response
shape (status, response header names, the JSON type tree with identifier-like key names, and the `isError`
flag), never a value, and discards unprinted any response that unexpectedly holds person data. The
Person Search probes have not been run yet: running `search-nomatch` and `search-validation` is the check to
make before relying on that operation's unverified assumptions.

<br/>

## ⚡ Benchmarks

Run the Go benchmarks:

```bash script
magex bench
```

Benchmarks measure local CPU work and allocations only (validation, encoding, strict checks and decoding)
with in-memory transports. They say nothing about network or provider latency, which dominates real calls.

<br/>

## 🛠️ Code Standards
Read more about this Go project's [code standards](.github/CODE_STANDARDS.md).

<br/>

## 🤖 AI Usage & Assistant Guidelines
Read the [AI Usage & Assistant Guidelines](.github/tech-conventions/ai-compliance.md) for details on how AI is used in this project and how to interact with the AI assistants.

<br/>

## 👥 Maintainers
| [<img src="https://github.com/mrz1836.png" height="50" alt="MrZ" />](https://github.com/mrz1836) |
|:------------------------------------------------------------------------------------------------:|
|                                [MrZ](https://github.com/mrz1836)                                 |

<br/>

## 🤝 Contributing
View the [contributing guidelines](.github/CONTRIBUTING.md) and please follow the [code of conduct](.github/CODE_OF_CONDUCT.md).

### How can I help?
All kinds of contributions are welcome :raised_hands:!
The most basic way to show your support is to star :star2: the project, or to raise issues :speech_balloon:.
You can also support this project by [becoming a sponsor on GitHub](https://github.com/sponsors/mrz1836) :clap:
or by making a [**bitcoin donation**](https://mrz1818.com/?tab=tips&utm_source=github&utm_medium=sponsor-link&utm_campaign=go-enformion&utm_term=go-enformion&utm_content=go-enformion) to ensure this journey continues indefinitely! :rocket:


[![Stars](https://img.shields.io/github/stars/mrz1836/go-enformion?label=Please%20like%20us&style=social)](https://github.com/mrz1836/go-enformion/stargazers)

<br/>

## 📝 License

[![License](https://img.shields.io/github/license/mrz1836/go-enformion.svg?style=flat&v=1)](LICENSE)
