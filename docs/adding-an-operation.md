# Adding an operation

go-enformion implements two EnformionGO operations: standard Contact
Enrichment (`EnrichContact`, one top match) and Person Search
(`SearchPersons`, one page of candidates). This note describes how a future
operation, such as Contact Enrich Plus or Caller ID, would be added without
breaking existing callers. Each one needs its own approval first: it is a new
billable surface with its own data, terms and contract evidence.

## The pattern

An operation is **one method on `*Client`** with **its own request and result
types**. There is no registry, generic executor, service object or stub.

```go
// EnrichContactPlus would send one subject to POST /Contact/EnrichPlus, the
// route the provider documents for Contact Enrich Plus.
func (c *Client) EnrichContactPlus(ctx context.Context, req EnrichContactPlusRequest) (*EnrichContactPlusResult, error)
```

The method follows the same steps as `EnrichContact` in `contact.go` and
`SearchPersons` in `person_search.go`:

1. Return `ErrInvalidConfig` (`Client`, `not_initialized`) for a nil or zero
   Client, and `ErrInvalidRequest` (`ctx`, `nil`) for a nil context.
2. Validate and encode the request with the operation's own rules, returning a
   `*ValidationError` that names fields (with a list index, such as
   `Addresses[1].County`) and fixed reasons, never values.
3. Return a `canceled` or `deadline_exceeded` `*CallError`, sending nothing, when
   the context is already done.
4. Call `c.send` with a `call`: the operation's path, its `galaxy-search-type`
   value, the session ID, the body and, when the operation needs one, its own
   response limit (`maxBytes`, where 0 means the client's). The sender is
   shared: one attempt, no retries, no redirects, the call timeout over the
   whole body read, the response limit, and sanitized errors all come with it.
   Person Search uses a 16 MiB default because a page can hold many people;
   `WithMaxResponseBytes`, once given, applies to every operation.
5. Decode the exchange with the operation's own decoder: `statusFailure` for
   any status other than 200, then `checkStrict`, the envelope (`isError`,
   `requestId`, `message`, `error`) and the operation's typed result.

## Rules for the new types

- **Request.** A plain struct passed by value. Values are sent verbatim after
  validation; limits the provider does not document are labelled as SDK limits.
  Never add a rule the provider does not document, and never modify the
  caller's request or its slices.
- **Result.** Every field optional unless the provider documents otherwise:
  pointers for numbers and booleans whose absence differs from zero, strings for
  provider text (dates and IDs are never parsed), slices normalized so that an
  empty list is nil, and JSON tags that round-trip.
- **Decoding.** Decode only the members the result exposes. Excluded members
  are never decoded, and a record whose data is only under unrecognized keys
  fails as `ErrUnexpectedResponse` rather than coming back silently empty.
  Flag and metadata objects may ignore unknown members instead.
- **Fixtures.** Write fixtures from scratch to the documented or observed
  shape with synthetic values; never copy the provider's examples or a real
  response. Name the source page and date in the fixture README.
- **Tests.** The same evidence as the existing operations: exact wire request,
  the status table, zero calls on invalid input, exactly one attempt on every
  failure, privacy markers across fmt verbs and error chains, allocation
  ceilings, an every-field fixture that leaves no exported field at zero, and
  fuzzing of the decoder.

## Decoding building blocks

`decode.go` holds what every operation's decoder is made of:

- **Objects of any size.** `readObject` reads an object against a list of
  names, matching keys without regard to ASCII case. Up to eight members are
  held inline; larger objects allocate the rest, or, for slot tables, keep
  them on the caller's stack.
- **Slot tables.** An object is decoded from a table of `slot`s, one per
  member, each with one destination: `str`, `flag` (`*bool`), `num` (`*int`,
  where a reported 0 is kept as a pointer to 0), `raw` (`json.RawMessage`) or
  `sub` (a nested object or list). `decodeSlots` applies the unknown-only rule
  when asked to track other members.
- **Lists.** `decodeList` decodes lists of objects and `decodeArray` lists of
  any element, such as `stringEntry` and raw entries. Every list keeps the
  provider's order, drops JSON null elements and fails past 1,000 entries.
- **Paths.** A `path` is a small value rendered only for an error, such as
  `persons[0].addresses[2].zip`. A `sub` decoder works at a path relative to its
  member, and the slot adds the member's path to any failure: a slot's name
  never reaches an indirect call, which would move every slot destination to
  the heap. Confirm with `go build -gcflags=-m` that a new decoder's slot
  tables, closures and destinations do not escape.
- **Raw capture.** Members whose structure is undocumented are kept as
  `json.RawMessage`, normalized by `normalizeJSON` and never aliasing the body.
  A record may also keep its unrecognized members in an `Extra` map, as Person
  Search persons do. Raw values are opaque personal data: document them so.
- **Decode budget.** An operation whose lists nest in other lists passes a
  `budget` (four times the body plus 1 MiB), charged per kept entry and raw
  value, so that a hostile body of nested empty entries fails with
  `ErrResponseTooLarge` instead of decoding to a hundred times its size.
  Contact Enrichment passes none.

## What must not change

- No general base URL option: credentials reach only the provider or loopback.
- No retries, polling, paging, fallbacks or follow-on calls inside a method.
- `Error()` strings stay free of values, URLs, bodies and provider text.
- The library stays standard-library only, and never logs, prints, stores or
  caches data, or reads credentials or configuration from the environment.
