# Provider-shaped fixtures

These fixtures follow the **shape** of the response examples on the EnformionGO
documentation page for Contact Enrichment
(<https://enformiongo.readme.io/reference/contact-enrichment-search>, OAS
`info.version` "1.4", page updated 2024-07-18; read on 2026-10-14).

They were written from scratch for this SDK. No example, value or schema was
copied from the documentation: every name, number, address, email and ID is
synthetic (fictional `555-01xx` phone numbers, `example.com` and `example.org`
domains, invented streets and IDs). The documentation's own example is not
valid JSON; these files are.

| File | Shape |
|---|---|
| `match.json` | HTTP 200 envelope with `isError: false` and a person with every documented field, including the excluded `relatives`, `pagination`, `searchCriteria` and `databaseQueryInfo` members |
| `rejected-400.json` | The documented HTTP 400 body, `{}` |
