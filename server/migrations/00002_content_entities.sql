-- +goose Up
CREATE TABLE public.content_entities (
    build_id text NOT NULL,
    entity_id text NOT NULL,
    kind text NOT NULL,
    region_id text CHECK (region_id IS NULL OR region_id ~ '^REG-0[1-6]$'),
    source_file text NOT NULL,
    source_line integer NOT NULL CHECK (source_line > 0),
    payload jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
    CONSTRAINT content_entities_build_fk FOREIGN KEY (build_id)
        REFERENCES public.content_builds(build_id) DEFERRABLE,
    -- Preserve unambiguous field boundaries in the storage digest.
    CHECK (translate(entity_id || kind || source_file, E'\x1e\x1f', '') = entity_id || kind || source_file),
    PRIMARY KEY (build_id, entity_id)
);

CREATE INDEX content_entities_build_kind_idx ON public.content_entities (build_id, kind);
CREATE INDEX content_entities_build_region_idx ON public.content_entities (build_id, region_id);

-- +goose Down
DROP TABLE public.content_entities;
