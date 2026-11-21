-- +goose Up
-- T-039a equipment instances (docs/contracts/equipment-engine.md): unique rolled items, bag position via
-- character_inventory.instance_id, worn items in character_equipped.

CREATE TABLE public.character_equipment_items (
    instance_id uuid PRIMARY KEY,
    character_id bigint NOT NULL REFERENCES public.characters(character_id),
    template_id text NOT NULL CHECK (template_id ~ '^EQ-0[1-5]-0[1-6]-(MH|OH|HD|CH|HA|LG|FT|RG|AM)-0[12]$'),
    rarity text NOT NULL CHECK (rarity IN ('COMMON','UNCOMMON','RARE','EPIC','LEGENDARY','MYTHIC')),
    item_level integer NOT NULL CHECK (item_level BETWEEN 1 AND 60),
    required_level integer NOT NULL CHECK (required_level BETWEEN 1 AND 60),
    stats jsonb NOT NULL,
    bind_state text NOT NULL DEFAULT 'BIND_ON_EQUIP'
        CHECK (bind_state IN ('UNBOUND','BIND_ON_EQUIP','BOUND_CHARACTER','BOUND_ACCOUNT')),
    source text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX character_equipment_items_char ON public.character_equipment_items (character_id);

CREATE TABLE public.character_equipped (
    character_id bigint NOT NULL REFERENCES public.characters(character_id),
    slot text NOT NULL CHECK (slot IN ('MH','OH','HD','CH','HA','LG','FT','RG','AM')),
    instance_id uuid NOT NULL UNIQUE REFERENCES public.character_equipment_items(instance_id),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, slot)
);

ALTER TABLE public.character_inventory
    ADD COLUMN instance_id uuid UNIQUE REFERENCES public.character_equipment_items(instance_id);

-- Stack merging applies to plain stacks only; unique gear rows share a template id.
DROP INDEX public.character_inventory_stack_unique;
CREATE UNIQUE INDEX character_inventory_stack_unique
    ON public.character_inventory (character_id, container, item_id) WHERE instance_id IS NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON public.character_equipment_items TO newmmo_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON public.character_equipped TO newmmo_app;

-- +goose Down
REVOKE SELECT, INSERT, UPDATE, DELETE ON public.character_equipped FROM newmmo_app;
REVOKE SELECT, INSERT, UPDATE, DELETE ON public.character_equipment_items FROM newmmo_app;
DELETE FROM public.character_inventory WHERE instance_id IS NOT NULL;
DROP INDEX public.character_inventory_stack_unique;
CREATE UNIQUE INDEX character_inventory_stack_unique
    ON public.character_inventory (character_id, container, item_id);
ALTER TABLE public.character_inventory DROP COLUMN instance_id;
DROP TABLE public.character_equipped;
DROP TABLE public.character_equipment_items;
