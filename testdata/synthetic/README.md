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
