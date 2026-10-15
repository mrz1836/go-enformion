# CLAUDE.md

## 🤖 Welcome, Claude

This repository uses **`AGENTS.md`** as the single source of truth for:

* Coding conventions (naming, formatting, commenting, testing)
* Contribution workflows (branch prefixes, commit message style, PR templates)
* Release, CI, and dependency‑management policies
* Security reporting and governance links

> **TL;DR:** **Read `AGENTS.md` first.**
> All technical or procedural questions are answered there.

### Quick Checklist for Claude

1. **Study `AGENTS.md`**
   Make sure every automated change or suggestion respects those rules.
2. **Follow branch‑prefix and commit‑message standards**
   They drive Mergify auto‑labeling and CI gates.
3. **Never tag releases**
4. **Pass CI**
   Run `go fmt`, `goimports`, `go vet`, `staticcheck`, and `golangci‑lint` locally before opening a PR.

If you encounter conflicting guidance elsewhere, `AGENTS.md` wins.
Questions or ambiguities? Open a discussion or ping a maintainer instead of guessing.

Happy hacking!

---

## About This Project

`go-enformion` is a small Go client for one EnformionGO developer API operation: standard
**Contact Enrichment**, `POST https://devapi.enformion.com/Contact/Enrich` with the search type
`DevAPIContactEnrich`. It is a library, not an application: there is no binary, no CLI, and no
runtime configuration file.

- Module: `github.com/mrz1836/go-enformion` (package `enformion`, one flat package)
- Go version: 1.26 (CI also checks the current release)
- Runtime dependencies: none (`testify` is used only by tests and `internal/testutil`)

**Invariants that changes must preserve:**
- **No runtime dependencies.** Library code imports only the standard library, and never `log`,
  `os`, `unsafe` or `net/http/httputil`. The `depguard` and `forbidigo` linters enforce this.
- **At most one attempt per call.** No retries, redirects, polling, hedging, fallbacks or
  follow-on calls, and `GetBody` stays nil so the standard library cannot resend a POST: the
  provider's pricing pages say it bills per match.
- **Credentials reach only the provider or loopback.** There is no general base URL option;
  `WithTestBaseURL` accepts only `https` on `127.0.0.0/8` or `[::1]`. Credentials live only in the
  sender closure, and are withheld from `httptrace` header hooks.
- **No values in errors.** `Error()` holds only kinds, statuses and JSON field paths; `GoString` and
  every fmt verb add only the provider request ID: never credentials, request values, URLs, bodies or
  provider text. Provider diagnostics on errors are opt-in (`WithProviderDiagnostics`).
- **No logging, globals (other than sentinel errors) or `init`, and no environment reads** in SDK
  code (the default transport's `http.ProxyFromEnvironment` honors the standard proxy variables).
- **Unit tests stay offline.** The live contract probe (`live_probe_internal_test.go`, `live`
  build tag) makes billable calls and is run only by the owner, by hand, after approval. Never run it.
- **Synthetic data only.** Fixtures are written from scratch to the documented shape; never copy
  provider examples or commit real responses.
