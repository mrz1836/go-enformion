# Adding an operation

go-enformion implements one EnformionGO operation, standard Contact Enrichment.
This note describes how a future operation, such as Contact Enrich Plus or
Caller ID, would be added without breaking existing callers. Each one needs its
own approval first: it is a new billable surface with its own data, terms and
contract evidence.

## The pattern

An operation is **one method on `*Client`** with **its own request and result
types**. There is no registry, generic executor, service object or stub.

```go
// EnrichContactPlus would send one subject to POST /Contact/EnrichPlus, the
// route the provider documents for Contact Enrich Plus.
func (c *Client) EnrichContactPlus(ctx context.Context, req EnrichContactPlusRequest) (*EnrichContactPlusResult, error)
```

The method follows the same steps as `EnrichContact` in `contact.go`:

1. Return `ErrInvalidConfig` (`Client`, `not_initialized`) for a nil or zero
   Client, and `ErrInvalidRequest` (`ctx`, `nil`) for a nil context.
2. Validate and encode the request with the operation's own rules, returning a
   `*ValidationError` that names fields and fixed reasons, never values.
3. Return a `canceled` or `deadline_exceeded` `*CallError`, sending nothing, when
   the context is already done.
4. Call `c.send` with the operation's path, its `galaxy-search-type` value, the
   session ID and the body. The sender is shared: one attempt, no retries, no
   redirects, the call timeout over the whole body read, the response limit,
   and sanitized errors all come with it.
5. Decode the exchange with the operation's own decoder: `statusFailure` for
   any status other than 200, then `checkStrict`, the envelope (`isError`,
   `requestId`, `message`, `error`) and the operation's typed result.

## Rules for the new types

- **Request.** A plain struct passed by value. Values are sent verbatim after
  validation; limits the provider does not document are labelled as SDK limits.
- **Result.** Every field optional unless the provider documents otherwise:
  pointers for numbers and booleans whose absence differs from zero, strings for
  provider text (dates and IDs are never parsed), slices normalized so that an
  empty list is nil, and JSON tags that round-trip.
- **Decoding.** Decode only the members the result exposes. Excluded members
  are never decoded, and data found only under unrecognized keys fails as
  `ErrUnexpectedResponse` rather than coming back silently empty.
- **Fixtures.** Write fixtures from scratch to the documented shape with
  synthetic values; never copy the provider's examples. Name the source page
  and date in the fixture README.
- **Tests.** The same evidence as Contact Enrichment: exact wire request, the
  status table, zero calls on invalid input, exactly one attempt on every
  failure, privacy markers across fmt verbs and error chains, and fuzzing of
  the decoder.

## What must not change

- No general base URL option: credentials reach only the provider or loopback.
- No retries, polling, fallbacks or follow-on calls inside a method.
- `Error()` strings stay free of values, URLs, bodies and provider text.
- The library stays standard-library only, and never logs, prints, stores or
  caches data, or reads credentials or configuration from the environment.
