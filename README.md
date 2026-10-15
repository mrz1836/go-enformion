<div align="center">

# 🔎&nbsp;&nbsp;go-enformion

**A small, standard-library-only Go client for EnformionGO Contact Enrichment.**

<br/>

<a href="https://github.com/mrz1836/go-enformion/releases"><img src="https://img.shields.io/github/release-pre/mrz1836/go-enformion?include_prereleases&style=flat-square&logo=github&color=black" alt="Release"></a>
<a href="https://golang.org/"><img src="https://img.shields.io/github/go-mod/go-version/mrz1836/go-enformion?style=flat-square&logo=go&color=00ADD8" alt="Go Version"></a>
<a href="https://github.com/mrz1836/go-enformion/blob/master/LICENSE"><img src="https://img.shields.io/github/license/mrz1836/go-enformion?style=flat-square&color=blue" alt="License"></a>

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

go-enformion calls one operation of the [EnformionGO](https://go.enformion.com/developer-apis/) developer
APIs (Enformion, LLC): standard **Contact Enrichment**, `POST https://devapi.enformion.com/Contact/Enrich`
with the search type `DevAPIContactEnrich`. Give it a subject with at least two of name, phone, email and
address, and it returns the provider's top person match (name, age, addresses, phones and emails) as
typed Go values, with every field optional and nothing invented.

The client is deliberately narrow and strict: one HTTP attempt per call, no retries or redirects, bounded
reads, strict JSON decoding, and errors that never contain credentials, personal data or provider text.

<br/>

## 📦 Installation

**go-enformion** requires a [supported release of Go](https://golang.org/doc/devel/release.html#policy) (Go 1.26 or newer).
```shell script
go get github.com/mrz1836/go-enformion
```

The library has **zero third-party runtime dependencies**: it imports only the standard library.
`testify` is used only by the tests and their internal helper package.

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
| `WithMaxResponseBytes(n)` | 1 MiB | 1 B to 16 MiB | Limits the (decompressed) response body. |
| `WithHTTPClient(hc)` | fresh transport | non-nil | Calls go through a **copy** of `hc` that refuses redirects and has no cookie jar; `hc` is never modified. |
| `WithTestBaseURL(url)` | production | `https` on `127.0.0.0/8` or `[::1]` | Points the client at a local test server. There is no other base URL option. |
| `WithProviderDiagnostics()` | off | | Keeps provider messages on errors, reachable only through `(*CallError).Diagnostics()`. |

Credentials must each be 1 to 1024 bytes of printable ASCII with no leading or trailing spaces. Invalid
arguments return a `*ValidationError` matching `ErrInvalidConfig` that names the argument, never its value.

### Build a request

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

### Read the result

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

### Handle errors

Validation failures are `*ValidationError` values, and **nothing was sent**. Every other failure is a
`*CallError` whose `Kind` matches one target with `errors.Is` (a `timeout` matches two):

| Kind | Matches | When |
|---|---|---|
| `unauthorized` | `ErrUnauthorized` | HTTP 401 or 403 |
| `rate_limited` | `ErrRateLimited` | HTTP 429 (rate limit or monthly quota). The SDK never retries. |
| `rejected` | `ErrRejected` | Another 4xx (a 404 is never a "no match"), or HTTP 200 with `isError: true` |
| `upstream` | `ErrUpstream` | HTTP 5xx |
| `unexpected_response` | `ErrUnexpectedResponse` | 1xx, 3xx (never followed), another 2xx, 600 and above, a body that is not an object, a missing or non-boolean `isError`, or a person, its name, or an address, phone or email entry whose only data is under unrecognized keys |
| `malformed_response` | `ErrMalformedResponse` | Invalid UTF-8 or JSON, trailing data, a repeated key, or a recognized field of the wrong type |
| `response_too_large` | `ErrResponseTooLarge` | An HTTP 200 body over the limit, or a person address, phone or email list of more than 1,000 entries |
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

### Testing your application

Depend on a small interface you own, and fake it in tests:

```go
type ContactEnricher interface {
	EnrichContact(context.Context, enformion.EnrichContactRequest) (*enformion.EnrichContactResult, error)
}

var _ ContactEnricher = (*enformion.Client)(nil)

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
- Each `EnrichContact` makes **at most one** HTTP attempt. The SDK never retries, follows a redirect, polls,
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
provider diagnostics on errors are off unless you opt in.

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
<summary><strong><code>When no person comes back</code></strong></summary>
<br/>

The provider does not document what a "no match" looks like. When an HTTP 200 response with
`isError: false` has no `person`, a `null` one, or one with no data at all, the result's `Person` is `nil`.
That means only that the response carried no person data: **it is not a verified no-match**, and the SDK
returns no `ErrNoMatch`. A person object, its name, or any address, phone or email entry whose only data
sits under keys the SDK does not recognize (as after a field rename) fails the call as
`ErrUnexpectedResponse` instead, so a billed match never silently comes back empty. An HTTP 404 is a rejection, never a "no match".

</details>

<details>
<summary><strong><code>Response members the SDK ignores</code></strong></summary>
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
<summary><strong><code>Limits and unverified assumptions</code></strong></summary>
<br/>

Built from the provider's documentation (readme.io docs version 1.4), which leaves these open. None has
been checked against the live API:

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
<summary><strong><code>Scope and extension</code></strong></summary>
<br/>

Only standard Contact Enrichment is implemented. Contact Enrich Plus, Person Search, Caller ID, ID
Verification, criminal and background searches, batch, pagination and detail lookups are out of scope,
and none is stubbed. A future operation would be a new method on `Client` with its own request and result
types, added without breaking existing callers; see [docs/adding-an-operation.md](docs/adding-an-operation.md).

</details>

<br/>

## 📚 Documentation

View the generated [documentation](https://pkg.go.dev/github.com/mrz1836/go-enformion?tab=doc) and the
provider's [Contact Enrichment reference](https://enformiongo.readme.io/reference/contact-enrichment).

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
reach their target, bounded reads, timeouts during the body read, safe concurrent use, and that
credentials and personal data never appear in errors or formatted output.

The runnable `Example*` functions in [example_test.go](example_test.go) run offline against a local TLS
server.

### Live contract probe

An opt-in probe in [live_probe_internal_test.go](live_probe_internal_test.go) makes **real, potentially
billable** calls to settle the open questions above. It is run only by the maintainer, by hand. It needs
the `live` build tag and these variables:

| Variable | Effect |
|---|---|
| `ENFORMION_LIVE=1` | Enables the probe; without it the probe skips. |
| `ENFORMION_LIVE_ACK_BILLING=I-understand` | Acknowledges that calls may be billed; without it the probe skips. |
| `CI` | Must be unset; when set, the probe skips. |
| `ENFORMION_LIVE_PROBES` | Comma-separated probes: `nomatch`, `validation` and `auth` take 1 call each, `casing` takes 2. |
| `ENFORMION_AP_NAME`, `ENFORMION_AP_PASSWORD` | Credentials, read only by the test harness. |

Once enabled, the probe fails before sending anything when `ENFORMION_LIVE_PROBES` names no known probe,
when the chosen probes need more than 4 calls (all four together need 5), or when the credentials are
invalid.

```bash
go test -tags live -count=1 -p 1 -run TestLiveProbe -v .
```

The `magex` test targets and CI compile the probe with the `live` tag too, and it skips there unless
`ENFORMION_LIVE` and `ENFORMION_LIVE_ACK_BILLING` are set, so set them only for the run.

A run makes at most 4 calls, never retries, and stops at the first HTTP 429. It prints only the response
shape (status, response header names, the JSON type tree with identifier-like key names, and the `isError`
flag), never a value, and discards unprinted any response that unexpectedly holds person data.

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
