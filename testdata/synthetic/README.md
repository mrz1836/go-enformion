# Synthetic robustness fixtures

These fixtures are **not** documented provider responses. They are shapes this
SDK must handle safely because the provider documents no schema, no no-match
response and no populated error entries. Each file is labelled by what it
exercises; every value is synthetic.

| File | Exercises |
|---|---|
| `no-person.json` | `isError: false` with `person: null`, a request ID and a string warning: a result with no person, never a no-match claim |
| `is-error.json` | HTTP 200 with `isError: true`, a message and entries of mixed JSON types: a rejection with diagnostics |
| `defaults-person.json` | A person object holding only defaults, the way .NET serializes an empty object: no person data |
| `unknown-keys-person.json` | A person whose only data is under keys the SDK does not decode, as after a rename: an unexpected response |
| `person-search-no-persons.json` | A full Person Search envelope with `persons: []`, zeroed counts and a string warning: a page without candidates, never a no-match claim |
| `person-search-is-error.json` | HTTP 200 with `isError: true`, candidates, a message and entries of mixed JSON types: a rejection that never returns its candidates |
| `person-search-missing-persons.json` | An envelope without `persons`: an unexpected response, never an empty page |
| `person-search-defaults-person.json` | A person holding only defaults, the way .NET serializes an empty object: a candidate kept without data |
| `person-search-unknown-keys-person.json` | A person whose only data is under keys the SDK does not decode, as after a rename: an unexpected response |
| `person-search-every-field.json` | Every member the SDK decodes, filled, plus undocumented person members, a nested associate with its own associates, raw source summaries and the ignored envelope members: proves no slot is left unbound |
| `person-search-duplicates.json` | Repeated candidates and entries and a JSON null candidate: provider order and duplicates are kept |
| `person-search-teaser-masked.json` | Masked and partial values, as the teaser modes are documented to return: kept verbatim. The masking format is invented; the provider documents none |
