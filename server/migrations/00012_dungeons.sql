-- +goose Up
-- EX-P09 dungeon instances (docs/contracts/dungeon-engine.md). Solo slice: owner = one character.
CREATE TABLE public.dungeon_instances (
    instance_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    character_id bigint NOT NULL REFERENCES public.characters(character_id),
    dungeon_id text NOT NULL CHECK (dungeon_id ~ '^DNG-0[1-6]-\d{3}$'),
    mode text NOT NULL CHECK (mode IN ('NORMAL', 'STORY_SOLO', 'VETERAN')),
    status text NOT NULL CHECK (status IN ('ACTIVE', 'BOSS_ACTIVE', 'COMPLETED', 'CLOSED', 'EXPIRED', 'ABANDONED')),
    room_id text NOT NULL,
    checkpoint_room_id text NOT NULL,
    event_seq bigint NOT NULL DEFAULT 0,
    state jsonb NOT NULL CHECK (jsonb_typeof(state) = 'object'),
    boss_attempts integer NOT NULL DEFAULT 0 CHECK (boss_attempts >= 0),
    seed bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);

-- One live instance per character (a COMPLETED one stays live until its reward is claimed).
CREATE UNIQUE INDEX dungeon_one_live_per_character
    ON public.dungeon_instances (character_id) WHERE status IN ('ACTIVE', 'BOSS_ACTIVE', 'COMPLETED');
CREATE INDEX dungeon_instances_status_seen ON public.dungeon_instances (status, last_seen_at);

CREATE TABLE public.dungeon_event_log (
    instance_id uuid NOT NULL REFERENCES public.dungeon_instances(instance_id),
    seq bigint NOT NULL,
    event_type text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (instance_id, seq)
);

CREATE TABLE public.dungeon_progress (
    character_id bigint NOT NULL REFERENCES public.characters(character_id),
    dungeon_id text NOT NULL,
    first_clear_at timestamptz,
    normal_clear_count integer NOT NULL DEFAULT 0 CHECK (normal_clear_count >= 0),
    PRIMARY KEY (character_id, dungeon_id)
);

CREATE TABLE public.dungeon_reward_claims (
    character_id bigint NOT NULL REFERENCES public.characters(character_id),
    instance_id uuid NOT NULL REFERENCES public.dungeon_instances(instance_id),
    reward_kind text NOT NULL,
    payload jsonb NOT NULL,
    claimed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, instance_id, reward_kind)
);

CREATE TRIGGER dungeon_event_log_no_update BEFORE UPDATE ON public.dungeon_event_log
    FOR EACH ROW EXECUTE FUNCTION public.reject_gameplay_mutation();
CREATE TRIGGER dungeon_event_log_no_delete BEFORE DELETE ON public.dungeon_event_log
    FOR EACH ROW EXECUTE FUNCTION public.reject_gameplay_mutation();
CREATE TRIGGER dungeon_event_log_no_truncate BEFORE TRUNCATE ON public.dungeon_event_log
    FOR EACH STATEMENT EXECUTE FUNCTION public.reject_gameplay_mutation();
CREATE TRIGGER dungeon_reward_claims_no_update BEFORE UPDATE ON public.dungeon_reward_claims
    FOR EACH ROW EXECUTE FUNCTION public.reject_gameplay_mutation();
CREATE TRIGGER dungeon_reward_claims_no_delete BEFORE DELETE ON public.dungeon_reward_claims
    FOR EACH ROW EXECUTE FUNCTION public.reject_gameplay_mutation();
CREATE TRIGGER dungeon_reward_claims_no_truncate BEFORE TRUNCATE ON public.dungeon_reward_claims
    FOR EACH STATEMENT EXECUTE FUNCTION public.reject_gameplay_mutation();

GRANT SELECT, INSERT, UPDATE ON public.dungeon_instances, public.dungeon_progress TO newmmo_app;
GRANT SELECT, INSERT ON public.dungeon_event_log, public.dungeon_reward_claims TO newmmo_app;

-- +goose Down
REVOKE SELECT, INSERT ON public.dungeon_event_log, public.dungeon_reward_claims FROM newmmo_app;
REVOKE SELECT, INSERT, UPDATE ON public.dungeon_instances, public.dungeon_progress FROM newmmo_app;
DROP TABLE IF EXISTS public.dungeon_reward_claims;
DROP TABLE IF EXISTS public.dungeon_progress;
DROP TABLE IF EXISTS public.dungeon_event_log;
DROP TABLE IF EXISTS public.dungeon_instances;
