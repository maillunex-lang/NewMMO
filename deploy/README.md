# Deploy notes (dev isolation)

NewMMO must use its **own** Compose project name, networks, volumes, and container names. Do not modify other VPS services, Caddy sites, certificates, or databases belonging to other projects.

Stage 1 does not ship a Compose stack yet. Stage 2 will add an isolated `newmmo` development stack (Go API, Postgres, optional admin) with loopback-only DB/admin ports.
