-- +goose Up
CREATE TABLE public.characters (
    character_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id uuid NOT NULL REFERENCES public.accounts(account_id),
    name text NOT NULL CHECK (name ~ '^[A-Za-zА-Яа-яЁё]{3,16}$'),
    name_ci text NOT NULL UNIQUE,
    class_id text NOT NULL CHECK (class_id ~ '^CLS-0[1-5]$'),
    xp bigint NOT NULL DEFAULT 0 CHECK (xp >= 0),
    gold bigint NOT NULL DEFAULT 0 CHECK (gold >= 0),
    state_revision bigint NOT NULL DEFAULT 0,
    flags jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(flags) = 'object'),
    content_build_id text REFERENCES public.content_builds(build_id),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (account_id, character_id)
);

-- +goose StatementBegin
CREATE FUNCTION public.guard_character_limit() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM 1 FROM public.accounts WHERE account_id = NEW.account_id FOR UPDATE;
    IF (SELECT count(*) FROM public.characters
        WHERE account_id = NEW.account_id AND character_id <> NEW.character_id) >= 3 THEN
        RAISE EXCEPTION 'character limit reached'
            USING ERRCODE = '23514', CONSTRAINT = 'characters_account_limit';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER characters_account_limit
BEFORE INSERT OR UPDATE OF account_id ON public.characters
FOR EACH ROW EXECUTE FUNCTION public.guard_character_limit();

CREATE TABLE public.ws_tickets (
    ticket_hash char(64) PRIMARY KEY CHECK (ticket_hash ~ '^[0-9a-f]{64}$'),
    account_id uuid NOT NULL REFERENCES public.accounts(account_id),
    character_id bigint NOT NULL REFERENCES public.characters(character_id),
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (account_id, character_id) REFERENCES public.characters(account_id, character_id)
);

CREATE INDEX ws_tickets_created ON public.ws_tickets (created_at);
CREATE INDEX ws_tickets_character ON public.ws_tickets (character_id);

GRANT SELECT ON public.characters, public.ws_tickets TO newmmo_app;
GRANT INSERT (account_id, name, name_ci, class_id, flags, content_build_id) ON public.characters TO newmmo_app;
GRANT INSERT (ticket_hash, account_id, character_id, expires_at), UPDATE (used_at), DELETE ON public.ws_tickets TO newmmo_app;

-- +goose Down
DROP TABLE public.ws_tickets;
DROP TABLE public.characters;
DROP FUNCTION public.guard_character_limit();
