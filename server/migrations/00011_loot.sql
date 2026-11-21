-- +goose Up
-- EX-P08 combat loot (docs/contracts/loot-engine.md): append-only ledger of server-rolled drops.
CREATE TABLE public.character_loot_ledger (
    ledger_id bigserial PRIMARY KEY,
    character_id bigint NOT NULL REFERENCES public.characters(character_id),
    request_id uuid NOT NULL,
    enemy_id text NOT NULL CHECK (enemy_id ~ '^ENM-0[1-6]-\d{3}$'),
    loot_table_id text NOT NULL CHECK (loot_table_id ~ '^LOOT-ENM-0[1-6]-\d{3}$'),
    gold bigint NOT NULL CHECK (gold >= 0),
    items jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(items) = 'array'),
    gold_after bigint NOT NULL CHECK (gold_after >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (character_id, request_id)
);

CREATE INDEX character_loot_ledger_recent ON public.character_loot_ledger (character_id, created_at);

CREATE TRIGGER character_loot_ledger_no_update BEFORE UPDATE ON public.character_loot_ledger
    FOR EACH ROW EXECUTE FUNCTION public.reject_gameplay_mutation();
CREATE TRIGGER character_loot_ledger_no_delete BEFORE DELETE ON public.character_loot_ledger
    FOR EACH ROW EXECUTE FUNCTION public.reject_gameplay_mutation();
CREATE TRIGGER character_loot_ledger_no_truncate BEFORE TRUNCATE ON public.character_loot_ledger
    FOR EACH STATEMENT EXECUTE FUNCTION public.reject_gameplay_mutation();

GRANT SELECT, INSERT ON public.character_loot_ledger TO newmmo_app;
GRANT USAGE ON SEQUENCE public.character_loot_ledger_ledger_id_seq TO newmmo_app;

-- +goose Down
REVOKE USAGE ON SEQUENCE public.character_loot_ledger_ledger_id_seq FROM newmmo_app;
REVOKE SELECT, INSERT ON public.character_loot_ledger FROM newmmo_app;
DROP TABLE IF EXISTS public.character_loot_ledger;
