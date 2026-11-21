# Contract: approved third-party toolchain pins (EX-P03)

**Owner:** director · Owner approvals (2026-11-21): Go ≥ 1.25, goose, pgx/v5, coder/websocket v1.8.15 + JSON v1. Research: `docs/research/db-migrations.md`.
Anything not listed here is **not approved**. Changing a pin = director task + new verification.

| Component | Pin | Why / verification (2026-11-21, director) |
|-----------|-----|-------------------------------------------|
| Go (server) | **`go 1.26.0` + `toolchain go1.26.11`** in `server/go.mod`; CI `setup-go 1.26.x` | **Changed 2026-11-21:** `govulncheck` showed 9 reachable stdlib vulnerabilities (net/http, crypto/x509, crypto/tls, net/textproto) in Go 1.25.14, fixed only in Go ≥ 1.26.9 (the 1.25 line is out of support; current stable 1.27.4 / 1.26.11). On 1.26.11: «No vulnerabilities found». pgx v5.13.0 (needs 1.25.0) is compatible. Raise the toolchain patch whenever `govulncheck` reports newer fixed-in versions |
| `github.com/jackc/pgx/v5` | **v5.13.0** (module + `pgxpool`) | MIT (LICENSE at tag read); `go.mod` at tag: `go 1.25.0`; direct requires: `pgpassfile v1.0.0`, `pgservicefile …5a60cdf6a761`, `puddle/v2 v2.2.3`, `golang.org/x/sync v0.22.0`, `golang.org/x/text v0.41.0` (testify etc. are test-only). `go.sum` committed; `go mod verify` must pass |
| goose CLI | **v3.30.0**, installed as a tool, **not** a server dependency: `go install github.com/pressly/goose/v3/cmd/goose@v3.30.0` | MIT (LICENSE at tag read). Tag `go.mod` needs Go 1.26.0 → with `GOTOOLCHAIN=auto` Go downloads 1.26.0 only for the install. Director built it successfully (`goose version: v3.30.0`). Integrity comes from the Go module checksum DB (no unverified prebuilt binary; release checksum file was not retrievable). Optional: slim build tags to drop unused DB drivers (verify tag names in goose docs before use) |
| PostgreSQL | major **17** (`postgres:17` image; digest pinned in `deploy/newmmo/` compose once pulled by an environment with Docker, recorded in the PR report) | Docker was not available to the director; CI uses the GitHub `services: postgres:17` container. Owner may prefer 18 → director re-pins |
| `github.com/coder/websocket` | **v1.8.15** (owner OK 2026-11-21; used by the EX-P04 server lane) | ISC (LICENSE.txt at tag read); `go.mod` at tag: `go 1.23`, **no external requires** (director re-checked); protocol rules in `websocket-protocol.md`; `govulncheck` must be clean before merge. Fallback `gorilla/websocket v1.5.3` is **not** approved for use unless the director re-pins |
| Android toolchain (developer machine only, **not in repo/CI**) | JDK 17, Android SDK cmdline-tools + platform-tools + platform/build-tools per **Godot 4.3** docs, Godot 4.3-stable export templates — **owner OK 2026-11-21 for T-012** | installed by the client executor under the user's home from official sources with SHA-256 verification; exact versions/checksums recorded → `docs/research/T-012-android-baseline-43.md`; no Android Studio/emulator/Gradle/NDK unless 4.3 docs require it for the prebuilt APK (then ask). CI use requires a separate director decision |
| `golang.org/x/vuln/cmd/govulncheck` (dev/CI tool, official Go team) | **v1.10.0**, `GOTOOLCHAIN=auto go install …@v1.10.0` (tag `go.mod` needs Go 1.26.0; auto-download like goose) | BSD-3-Clause (Go project); runs in CI job `server-db` against `server/`; not a module dependency |
| sqlc | **not approved** | separate decision later |
| golang-migrate | fallback only, **not approved for use** | `v4.20.1` needs Go 1.25.11 |
| ORMs (GORM/ent/bun) | **forbidden** in EX-P03 | |

**Rules:** add only the pgx modules above (and their transitive deps as resolved by `go mod tidy`; list the resulting `go.mod`/`go.sum` additions in the report);
run `go mod verify`; no `replace` directives; no vendored copies; no other new modules without a research note + owner OK.
