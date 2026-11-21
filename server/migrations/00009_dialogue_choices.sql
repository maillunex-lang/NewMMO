-- +goose Up
-- EX-P05 dialogue choice persistence (docs/contracts/dialogue-engine.md).

CREATE TABLE public.character_dialogue_choices (
    character_id bigint NOT NULL REFERENCES public.characters(character_id),
    dialogue_id text NOT NULL,
    node_id text NOT NULL,
    option_id text NOT NULL,
    content_build_id text NOT NULL REFERENCES public.content_builds(build_id),
    chosen_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, dialogue_id, node_id)
);

CREATE INDEX character_dialogue_choices_char ON public.character_dialogue_choices (character_id);

-- +goose StatementBegin
CREATE FUNCTION public.reject_dialogue_choice_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'character_dialogue_choices is append-only'
        USING ERRCODE = '42501';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER character_dialogue_choices_no_update
BEFORE UPDATE ON public.character_dialogue_choices
FOR EACH ROW EXECUTE FUNCTION public.reject_dialogue_choice_mutation();
CREATE TRIGGER character_dialogue_choices_no_delete
BEFORE DELETE ON public.character_dialogue_choices
FOR EACH ROW EXECUTE FUNCTION public.reject_dialogue_choice_mutation();
CREATE TRIGGER character_dialogue_choices_no_truncate
BEFORE TRUNCATE ON public.character_dialogue_choices
FOR EACH STATEMENT EXECUTE FUNCTION public.reject_dialogue_choice_mutation();

GRANT SELECT, INSERT ON public.character_dialogue_choices TO newmmo_app;

-- +goose Down
REVOKE SELECT, INSERT ON public.character_dialogue_choices FROM newmmo_app;
DROP TRIGGER IF EXISTS character_dialogue_choices_no_truncate ON public.character_dialogue_choices;
DROP TRIGGER IF EXISTS character_dialogue_choices_no_delete ON public.character_dialogue_choices;
DROP TRIGGER IF EXISTS character_dialogue_choices_no_update ON public.character_dialogue_choices;
DROP FUNCTION IF EXISTS public.reject_dialogue_choice_mutation();
DROP TABLE IF EXISTS public.character_dialogue_choices;
