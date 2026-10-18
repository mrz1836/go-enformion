# Provider-shaped fixtures

These fixtures follow the **shape** of the provider's responses. They were
written from scratch for this SDK. No example, value or schema was copied: every
name, number, address, email and ID is synthetic (fictional `555-01xx` phone
numbers, `example.com` and `example.org` domains, invented streets and IDs).
The documentation's own examples are not valid JSON; these files are.

| File | Source of the shape | Shape |
|---|---|---|
| `match.json` | The Contact Enrichment response examples (<https://enformiongo.readme.io/reference/contact-enrichment-search>, OAS `info.version` "1.4", page updated 2024-07-18; read on 2026-10-14) | HTTP 200 envelope with `isError: false` and a person with every documented field, including the excluded `relatives`, `pagination`, `searchCriteria` and `databaseQueryInfo` members |
| `rejected-400.json` | The same page | The documented HTTP 400 body, `{}` |
| `person-search-page.json` | The Person Search response example (<https://enformiongo.readme.io/reference/search>, OAS `info.version` "1.4"; read on 2026-10-17) for the person members, and the observed response structure for the envelope, its casing and the members the documentation leaves out | HTTP 200 envelope (`persons`, `counts`, `isError`, `error`, `requestId`, and `pagination`, `criteriaType` and `smartSearchStatistics` as the empty objects observed) holding two persons with the observed members, including the undocumented `driversLicenseDetail`; `sourceSummary`, documented as null and not observed, is left out |
| `person-search-rejected-400.json` | The same page | The documented HTTP 400 body, `{}` |
