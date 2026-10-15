// Package enformion is a small, standard-library-only Go client for one
// operation of the EnformionGO developer APIs (Enformion, LLC): standard Contact
// Enrichment, POST https://devapi.enformion.com/Contact/Enrich with the search
// type DevAPIContactEnrich.
//
// Create a Client with NewClient and the dashboard's API key name and password,
// then call EnrichContact with at least two of the four documented search
// categories: name, phone, email and address. The result holds the provider's
// top person match, if the response carried one, with typed, optional fields.
//
//	client, err := enformion.NewClient(keyName, keyPassword)
//	if err != nil {
//		return err
//	}
//	res, err := client.EnrichContact(ctx, enformion.EnrichContactRequest{
//		FirstName: "Avery", LastName: "Example", Email: "avery.example@example.com",
//	})
//
// # Calls and billing
//
// devapi.enformion.com is the only server the provider documents. Every call to
// it is live, and the provider's pricing pages say it bills per successful
// match; there is no documented sandbox. Each EnrichContact call makes at most
// one HTTP attempt: the SDK never retries, follows a redirect, polls, hedges or
// makes a follow-on call, and it disables the standard library's own automatic
// resends. An injected http.Client's transport is trusted and may still retry
// on its own. After a timeout, a cancellation, a transport failure or an
// unreadable response, the provider may already have processed and billed the
// request. The SDK makes no claim about what was or was not billed.
//
// Requests that fail local validation, and calls whose context is already done,
// send nothing.
//
// An httptrace.ClientTrace on the context still runs, but its WroteHeaderField
// hook never receives the credential headers. The standard library's own debug
// logging (GODEBUG=http2debug=2) prints every request header, credentials
// included, and must not be enabled with real credentials.
//
// # Errors
//
// Validation failures are *ValidationError values matching ErrInvalidConfig or
// ErrInvalidRequest. Every other failure is a *CallError, whose Kind matches one
// sentinel (or context.Canceled or context.DeadlineExceeded) with errors.Is.
// Error strings hold only the kind, the HTTP status and a JSON field path: never
// credentials, request values, URLs, response bodies or provider messages.
// Provider messages, inputErrors and warnings are kept on errors only with
// WithProviderDiagnostics, and are then reachable only through
// (*CallError).Diagnostics.
//
// # Personal data and permitted use
//
// Requests and results hold personal data. The SDK never logs, prints, stores
// or caches anything, and never reads credentials or configuration from the
// environment, but its request and result types have no redacting formatter:
// access control, logging, retention, security and permitted use are the
// caller's responsibility. Serializing or hashing personal data does not
// anonymize it.
//
// The provider's terms restrict how its data may be used. In summary, and
// without any claim of legal suitability: it is not a consumer reporting agency
// under the FCRA, so its data may not be used as a factor in eligibility for
// credit, insurance, employment, housing or any other FCRA purpose; results may
// not be stored or cached to make additional inquiries, merged with other data,
// scraped or resold; minors, celebrities and public figures may not be
// searched; and compliance with CAN-SPAM, the TCPA and Do Not Call rules is the
// caller's responsibility, as the provider scrubs no numbers. Read the
// provider's current terms before use.
//
// # Proxies
//
// The default transport honors HTTPS_PROXY and NO_PROXY through
// http.ProxyFromEnvironment, as the standard library's default transport does.
// To bypass proxies, inject an http.Client whose transport sets no Proxy.
//
// # Scope and extension
//
// Only standard Contact Enrichment is implemented. Contact Enrich Plus, Person
// Search, Caller ID, ID Verification, batch, pagination and every other
// EnformionGO product are out of scope. Future operations would be added as
// new methods on Client, each with its own request and result types, without
// breaking existing callers.
package enformion
