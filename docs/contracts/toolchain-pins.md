# Contract: approved third-party toolchain pins (EX-P03)

**Owner:** director · Owner approvals: Go ≥ 1.25, goose, pgx/v5 (2026-11-21). Research: `docs/research/db-migrations.md`.
Anything not listed here is **not approved**. Changing a pin = director task + new verification.

| Component | Pin | Why / verification (2026-11-21, director) |
|-----------|-----|-------------------------------------------|
| Go (server) | `go 1.25.0` in `server/go.mod`; CI `setup-go 1.25.x` | T-006 merged; pgx v5.13.0 requires `go 1.25.0` |
| `github.com/jackc/pgx/v5` | **v5.13.0** (module + `pgxpool`) | MIT (LICENSE at tag read); `go.mod` at tag: `go 1.25.0`; direct requires: `pgpassfile v1.0.0`, `pgservicefile …5a60cdf6a761`, `puddle/v2 v2.2.3`, `golang.org/x/sync v0.22.0`, `golang.org/x/text v0.41.0` (testify etc. are test-only). `go.sum` committed; `go mod verify` must pass |
| goose CLI | **v3.30.0**, installed as a tool, **not** a server dependency: `go install github.com/pressly/goose/v3/cmd/goose@v3.30.0` | MIT (LICENSE at tag read). Tag `go.mod` needs Go 1.26.0 → with `GOTOOLCHAIN=auto` Go downloads 1.26.0 only for the install. Director built it successfully (`goose version: v3.30.0`). Integrity comes from the Go module checksum DB (no unverified prebuilt binary; release checksum file was not retrievable). Optional: slim build tags to drop unused DB drivers (verify tag names in goose docs before use) |
| PostgreSQL | major **17** (`postgres:17` image; digest pinned in `deploy/newmmo/` compose once pulled by an environment with Docker, recorded in the PR report) | Docker was not available to the director; CI uses the GitHub `services: postgres:17` container. Owner may prefer 18 → director re-pins |
| sqlc | **not approved** | separate decision later |
| golang-migrate | fallback only, **not approved for use** | `v4.20.1` needs Go 1.25.11 |
| ORMs (GORM/ent/bun) | **forbidden** in EX-P03 | |

**Rules:** add only the pgx modules above (and their transitive deps as resolved by `go mod tidy`; list the resulting `go.mod`/`go.sum` additions in the report);
run `go mod verify`; no `replace` directives; no vendored copies; no other new modules without a research note + owner OK.
