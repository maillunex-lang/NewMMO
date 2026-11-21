-- +goose Up
-- Gameplay quest/reward tables (docs/contracts/db-gameplay.md §2) + character mutation grants.

CREATE TABLE public.character_reputation (
    character_id bigint NOT NULL REFERENCES public.characters(character_id),
    faction_id text NOT NULL CHECK (faction_id ~ '^FAC-0[1-8]$'),
    value integer NOT NULL DEFAULT 0,
    PRIMARY KEY (character_id, faction_id)
);

CREATE TABLE public.character_quest_states (
    character_id bigint NOT NULL REFERENCES public.characters(character_id),
    quest_id text NOT NULL,
    state text NOT NULL CHECK (state IN ('ACTIVE', 'READY_TO_TURN_IN', 'COMPLETED')),
    objective_progress jsonb NOT NULL DEFAULT '{}'::jsonb
        CHECK (jsonb_typeof(objective_progress) = 'object'),
    content_build_id text NOT NULL REFERENCES public.content_builds(build_id),
    revision bigint NOT NULL DEFAULT 0,
    accepted_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    PRIMARY KEY (character_id, quest_id)
);

CREATE INDEX character_quest_states_char_state ON public.character_quest_states (character_id, state);

CREATE TABLE public.quest_objective_instances (
    character_id bigint NOT NULL,
    objective_id text NOT NULL,
    instance_id text NOT NULL,
    at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, objective_id, instance_id)
);

CREATE TABLE public.reward_claims (
    claim_id uuid PRIMARY KEY,
    character_id bigint NOT NULL REFERENCES public.characters(character_id),
    source_kind text NOT NULL,
    source_instance_id text NOT NULL,
    reward_bundle_id text NOT NULL,
    content_build_id text NOT NULL REFERENCES public.content_builds(build_id),
    granted_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (character_id, source_kind, source_instance_id)
);

CREATE TABLE public.character_ledger (
    ledger_id bigserial PRIMARY KEY,
    character_id bigint NOT NULL REFERENCES public.characters(character_id),
    claim_id uuid NOT NULL REFERENCES public.reward_claims(claim_id),
    kind text NOT NULL CHECK (kind IN ('xp', 'gold', 'reputation', 'quest_item')),
    ref text,
    delta bigint NOT NULL,
    balance_after bigint,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX character_ledger_char ON public.character_ledger (character_id);

CREATE TABLE public.character_quest_items (
    character_id bigint NOT NULL REFERENCES public.characters(character_id),
    item_id text NOT NULL CHECK (item_id ~ '^ITEM-(Q|SQ)-'),
    claim_id uuid NOT NULL REFERENCES public.reward_claims(claim_id),
    PRIMARY KEY (character_id, item_id)
);

CREATE TABLE public.idempotency_keys (
    character_id bigint NOT NULL REFERENCES public.characters(character_id),
    request_id uuid NOT NULL,
    message_type text NOT NULL,
    payload_sha256 char(64) NOT NULL CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
    result jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, request_id)
);

CREATE INDEX idempotency_keys_created ON public.idempotency_keys (created_at);

-- Append-only + COMPLETED immutability (db-gameplay.md §3).
-- +goose StatementBegin
CREATE FUNCTION public.reject_gameplay_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'gameplay table % is append-only', TG_TABLE_NAME
        USING ERRCODE = '42501';
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION public.guard_quest_state_transition() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'UPDATE' AND OLD.state = 'COMPLETED' THEN
        RAISE EXCEPTION 'COMPLETED quest rows are immutable'
            USING ERRCODE = '42501';
    END IF;
    IF TG_OP = 'UPDATE' AND OLD.state = 'ACTIVE' AND NEW.state NOT IN ('ACTIVE', 'READY_TO_TURN_IN') THEN
        RAISE EXCEPTION 'invalid quest state transition'
            USING ERRCODE = '23514';
    END IF;
    IF TG_OP = 'UPDATE' AND OLD.state = 'READY_TO_TURN_IN' AND NEW.state NOT IN ('READY_TO_TURN_IN', 'COMPLETED') THEN
        RAISE EXCEPTION 'invalid quest state transition'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER reward_claims_no_update BEFORE UPDATE ON public.reward_claims
    FOR EACH ROW EXECUTE FUNCTION public.reject_gameplay_mutation();
CREATE TRIGGER reward_claims_no_delete BEFORE DELETE ON public.reward_claims
    FOR EACH ROW EXECUTE FUNCTION public.reject_gameplay_mutation();
CREATE TRIGGER reward_claims_no_truncate BEFORE TRUNCATE ON public.reward_claims
    FOR EACH STATEMENT EXECUTE FUNCTION public.reject_gameplay_mutation();

CREATE TRIGGER character_ledger_no_update BEFORE UPDATE ON public.character_ledger
    FOR EACH ROW EXECUTE FUNCTION public.reject_gameplay_mutation();
CREATE TRIGGER character_ledger_no_delete BEFORE DELETE ON public.character_ledger
    FOR EACH ROW EXECUTE FUNCTION public.reject_gameplay_mutation();
CREATE TRIGGER character_ledger_no_truncate BEFORE TRUNCATE ON public.character_ledger
    FOR EACH STATEMENT EXECUTE FUNCTION public.reject_gameplay_mutation();

CREATE TRIGGER character_quest_items_no_update BEFORE UPDATE ON public.character_quest_items
    FOR EACH ROW EXECUTE FUNCTION public.reject_gameplay_mutation();
CREATE TRIGGER character_quest_items_no_delete BEFORE DELETE ON public.character_quest_items
    FOR EACH ROW EXECUTE FUNCTION public.reject_gameplay_mutation();
CREATE TRIGGER character_quest_items_no_truncate BEFORE TRUNCATE ON public.character_quest_items
    FOR EACH STATEMENT EXECUTE FUNCTION public.reject_gameplay_mutation();

CREATE TRIGGER character_quest_states_transition
BEFORE UPDATE ON public.character_quest_states
FOR EACH ROW EXECUTE FUNCTION public.guard_quest_state_transition();

GRANT SELECT ON public.character_reputation, public.character_quest_states,
    public.quest_objective_instances, public.reward_claims, public.character_ledger,
    public.character_quest_items, public.idempotency_keys TO newmmo_app;

GRANT INSERT, UPDATE ON public.character_reputation TO newmmo_app;
GRANT INSERT, UPDATE ON public.character_quest_states TO newmmo_app;
GRANT INSERT ON public.quest_objective_instances TO newmmo_app;
GRANT INSERT ON public.reward_claims TO newmmo_app;
GRANT INSERT ON public.character_ledger TO newmmo_app;
GRANT USAGE ON SEQUENCE public.character_ledger_ledger_id_seq TO newmmo_app;
GRANT INSERT ON public.character_quest_items TO newmmo_app;
GRANT INSERT, SELECT, DELETE ON public.idempotency_keys TO newmmo_app;

-- Character aggregates must be updatable for rewards (was insert-only in 00007).
GRANT UPDATE (xp, gold, state_revision, flags) ON public.characters TO newmmo_app;

-- +goose Down
REVOKE UPDATE (xp, gold, state_revision, flags) ON public.characters FROM newmmo_app;
REVOKE INSERT, SELECT, DELETE ON public.idempotency_keys FROM newmmo_app;
REVOKE INSERT ON public.character_quest_items FROM newmmo_app;
REVOKE USAGE ON SEQUENCE public.character_ledger_ledger_id_seq FROM newmmo_app;
REVOKE INSERT ON public.character_ledger FROM newmmo_app;
REVOKE INSERT ON public.reward_claims FROM newmmo_app;
REVOKE INSERT ON public.quest_objective_instances FROM newmmo_app;
REVOKE INSERT, UPDATE ON public.character_quest_states FROM newmmo_app;
REVOKE INSERT, UPDATE ON public.character_reputation FROM newmmo_app;
REVOKE SELECT ON public.character_reputation, public.character_quest_states,
    public.quest_objective_instances, public.reward_claims, public.character_ledger,
    public.character_quest_items, public.idempotency_keys FROM newmmo_app;

DROP TRIGGER IF EXISTS character_quest_states_transition ON public.character_quest_states;
DROP TRIGGER IF EXISTS character_quest_items_no_truncate ON public.character_quest_items;
DROP TRIGGER IF EXISTS character_quest_items_no_delete ON public.character_quest_items;
DROP TRIGGER IF EXISTS character_quest_items_no_update ON public.character_quest_items;
DROP TRIGGER IF EXISTS character_ledger_no_truncate ON public.character_ledger;
DROP TRIGGER IF EXISTS character_ledger_no_delete ON public.character_ledger;
DROP TRIGGER IF EXISTS character_ledger_no_update ON public.character_ledger;
DROP TRIGGER IF EXISTS reward_claims_no_truncate ON public.reward_claims;
DROP TRIGGER IF EXISTS reward_claims_no_delete ON public.reward_claims;
DROP TRIGGER IF EXISTS reward_claims_no_update ON public.reward_claims;

DROP FUNCTION IF EXISTS public.guard_quest_state_transition();
DROP FUNCTION IF EXISTS public.reject_gameplay_mutation();

DROP TABLE IF EXISTS public.idempotency_keys;
DROP TABLE IF EXISTS public.character_quest_items;
DROP TABLE IF EXISTS public.character_ledger;
DROP TABLE IF EXISTS public.reward_claims;
DROP TABLE IF EXISTS public.quest_objective_instances;
DROP TABLE IF EXISTS public.character_quest_states;
DROP TABLE IF EXISTS public.character_reputation;
