# Contract: gameplay tables for EX-P04 (D-004)

**Owner:** director · **Version:** 1 · Adapts `14_CONTENT_INTEGRATION.md` §16 (reference SQL) and `db-content.md` conventions (goose, roles, triggers). Names below are final; types/CHECKs are the required shape. Migrations continue the numbering (`00006…`), one concern per file.

## 1. Roles
`newmmo_app` (runtime) keeps `SELECT` on content tables and gets **DML on the gameplay tables below** (no DDL; **no UPDATE/DELETE** on append-only tables). `newmmo_content_publisher` and `newmmo_migrator` unchanged. The runtime never uses the publisher role.

## 2. Tables
```
accounts(account_id uuid PK, login text UNIQUE CHECK (login ~ '^[a-z0-9_]{3,24}$'), password_hash text NOT NULL,
         created_at timestamptz NOT NULL DEFAULT now())
auth_tokens(token_hash char(64) PK CHECK (token_hash ~ '^[0-9a-f]{64}$'), account_id uuid NOT NULL REFERENCES accounts,
         expires_at timestamptz NOT NULL, revoked_at timestamptz, created_at timestamptz NOT NULL DEFAULT now())
login_attempts(attempt_id bigserial PK, login text NOT NULL, ip inet NOT NULL, ok boolean NOT NULL, at timestamptz NOT NULL DEFAULT now())   -- rate limiting; pruned after 24 h
characters(character_id bigint GENERATED ALWAYS AS IDENTITY PK, account_id uuid NOT NULL REFERENCES accounts,
         name text NOT NULL, name_ci text NOT NULL UNIQUE,                        -- lower(name)
         class_id text NOT NULL CHECK (class_id ~ '^CLS-0[1-5]$'),
         xp bigint NOT NULL DEFAULT 0 CHECK (xp >= 0), gold bigint NOT NULL DEFAULT 0 CHECK (gold >= 0),
         state_revision bigint NOT NULL DEFAULT 0, flags jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(flags)='object'),
         created_at timestamptz NOT NULL DEFAULT now())          -- + CHECK/trigger: ≤ 3 characters per account (enforced in the creating tx under an account row lock)
ws_tickets(ticket_hash char(64) PK, account_id uuid NOT NULL REFERENCES accounts, character_id bigint NOT NULL REFERENCES characters,
         expires_at timestamptz NOT NULL, used_at timestamptz, created_at timestamptz NOT NULL DEFAULT now())
character_reputation(character_id bigint REFERENCES characters, faction_id text CHECK (faction_id ~ '^FAC-0[1-8]$'),
         value integer NOT NULL DEFAULT 0, PRIMARY KEY (character_id, faction_id))
character_quest_states(character_id bigint REFERENCES characters, quest_id text,
         state text NOT NULL CHECK (state IN ('ACTIVE','READY_TO_TURN_IN','COMPLETED')),
         objective_progress jsonb NOT NULL DEFAULT '{}'::jsonb,        -- {"OBJ-…": {"have": n}}
         content_build_id text NOT NULL REFERENCES content_builds(build_id),      -- pinned at accept (R-08)
         revision bigint NOT NULL DEFAULT 0, accepted_at timestamptz NOT NULL DEFAULT now(), completed_at timestamptz,
         PRIMARY KEY (character_id, quest_id))
quest_objective_instances(character_id bigint, objective_id text, instance_id text, at timestamptz NOT NULL DEFAULT now(),
         PRIMARY KEY (character_id, objective_id, instance_id))          -- distinct interaction instances (R-OBJ-SPAM)
reward_claims(claim_id uuid PK, character_id bigint NOT NULL REFERENCES characters, source_kind text NOT NULL,   -- 'quest'
         source_instance_id text NOT NULL, reward_bundle_id text NOT NULL, content_build_id text NOT NULL REFERENCES content_builds(build_id),
         granted_at timestamptz NOT NULL DEFAULT now(), UNIQUE (character_id, source_kind, source_instance_id))
character_ledger(ledger_id bigserial PK, character_id bigint NOT NULL REFERENCES characters, claim_id uuid NOT NULL REFERENCES reward_claims,
         kind text NOT NULL CHECK (kind IN ('xp','gold','reputation','quest_item')), ref text,        -- FAC-08 / ITEM-Q-…
         delta bigint NOT NULL, balance_after bigint, created_at timestamptz NOT NULL DEFAULT now())  -- append-only
character_quest_items(character_id bigint REFERENCES characters, item_id text CHECK (item_id ~ '^ITEM-(Q|SQ)-'), claim_id uuid NOT NULL REFERENCES reward_claims,
         PRIMARY KEY (character_id, item_id))                             -- virtual QUEST_KEY evidence (not a physical inventory)
idempotency_keys(character_id bigint REFERENCES characters, request_id uuid, message_type text NOT NULL,
         payload_sha256 char(64) NOT NULL, result jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
         PRIMARY KEY (character_id, request_id))                           -- retention ≥ 24 h, pruned by a job
```
`character_dialogue_choices` — EX-P05 / T-024 (`00009_dialogue_choices.sql`, `dialogue-engine.md`). Deferred: `dungeon_instances`, `world_event_instances` (EX-P09/P10; canon §16 `world_event_instances` gets a `content_build_id` FK there), inventory (EX-P08).

## 3. Integrity rules
- `reward_claims`, `character_ledger`, `character_quest_items`: **append-only** (UPDATE/DELETE/TRUNCATE rejected by triggers, like `content_audit`). `reward_claims` is immutable once inserted.
- Balances: `characters.xp/gold` are the only mutable aggregates; every change must be accompanied, **in the same transaction**, by matching `character_ledger` rows (kind, delta, `balance_after`); a periodic/test check asserts `characters.xp = sum(ledger xp)` and the same for gold (the ledger is the audit truth).
- `COMPLETED` quest rows are never changed again (trigger); `state` transitions only along `ACTIVE → READY_TO_TURN_IN → COMPLETED`.
- `content_build_id` columns reference `content_builds` — builds referenced by gameplay rows are never deleted (retention rule of `db-content.md` §3).

## 4. Transaction protocol (per character serialization)
Every mutating operation (accept / interact / turn-in) runs in **one short transaction**, in this lock order: (1) `SELECT … FROM characters WHERE character_id=$1 FOR UPDATE` (serializes all mutations of that character and guards `state_revision`); (2) quest rows of that character; (3) inserts. `state_revision` is incremented **once per successful mutation**. Idempotency: first `SELECT` of `idempotency_keys (character_id, request_id)` under the character lock — same payload hash ⇒ return the stored `result` (no effects); different hash ⇒ `IDEMPOTENCY_CONFLICT`; otherwise perform the effects and insert the key + result in the same transaction. Deadlock/serialization retries ≤ 3. No network calls inside the transaction; WebSocket messages are sent **after commit**.

## 5. Acceptance
Migrations up→reset→up in CI (existing `server-db`), triggers/privileges tested **as `newmmo_app`** (cannot UPDATE/DELETE ledger/claims/COMPLETED rows, cannot TRUNCATE), ledger sums equal aggregates after randomized operation sequences, concurrent turn-in of the same quest (two goroutines/two connections) ⇒ exactly one reward, character limit under concurrent creation, ticket one-time under concurrency.
