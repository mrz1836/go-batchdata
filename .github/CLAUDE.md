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

`go-batchdata` is a small Go SDK for the [BatchData](https://batchdata.com) V3 API. Version 1 covers
synchronous forward property skip tracing only (`POST /api/v3/property/skip-trace`). It is a library,
not an application: there is no binary, no CLI and no runtime configuration file.

- Module: `github.com/mrz1836/go-batchdata` (package `batchdata`; service package `skiptrace`)
- Go version: 1.26 (CI also checks the current release)
- Runtime dependencies: none (`testify` is used only by tests and `internal/testutil`)
- Adding an endpoint: follow [docs/adding-a-service.md](../docs/adding-a-service.md)

**Invariants that changes must preserve:**
- **No runtime dependencies.** The root, `skiptrace` and their internal packages import only the
  standard library. The `depguard` linter enforces this for every non-test file outside
  `internal/testutil`, which only tests import.
- **One attempt per call.** No retries, fallback, hedging, chunking, polling or followed redirects,
  and no rewindable request body: every call may be billed.
- **No environment reads, globals or `init`.** Credentials and options are passed explicitly. The SDK's
  own code reads no environment variable; the standard library's transport reads the proxy variables.
- **No secrets or payloads in errors.** Keys, URLs, bodies, names, addresses, phones, emails, provider
  messages and network addresses never appear in `Error()` strings, `GoString`, or any error an
  `Unwrap` chain returns.
- **No positional guessing.** A record is linked to a request item only by a unique echoed
  `requestId`, or by elimination for a one-item request answered by one decodable record.
- **Unit tests stay offline.** Fixtures are synthetic; nothing is copied from BatchData's examples, and
  BatchData's OpenAPI export is never committed.
