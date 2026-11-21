-- +goose Up
-- EX-P08 inventory stacks (docs/contracts/inventory-engine.md).

ALTER TABLE public.characters
    ADD COLUMN IF NOT EXISTS inventory_revision bigint NOT NULL DEFAULT 0;

CREATE TABLE public.character_inventory (
    character_id bigint NOT NULL REFERENCES public.characters(character_id),
    container text NOT NULL CHECK (container IN ('bag', 'materials')),
    slot integer NOT NULL CHECK (slot >= 0),
    item_id text NOT NULL,
    quantity integer NOT NULL CHECK (quantity > 0),
    inventory_revision bigint NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, container, slot)
);

CREATE UNIQUE INDEX character_inventory_stack_unique
    ON public.character_inventory (character_id, container, item_id);

CREATE INDEX character_inventory_char ON public.character_inventory (character_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON public.character_inventory TO newmmo_app;
GRANT UPDATE (inventory_revision) ON public.characters TO newmmo_app;

-- +goose Down
REVOKE UPDATE (inventory_revision) ON public.characters FROM newmmo_app;
REVOKE SELECT, INSERT, UPDATE, DELETE ON public.character_inventory FROM newmmo_app;
DROP INDEX IF EXISTS public.character_inventory_stack_unique;
DROP TABLE IF EXISTS public.character_inventory;
ALTER TABLE public.characters DROP COLUMN IF EXISTS inventory_revision;
