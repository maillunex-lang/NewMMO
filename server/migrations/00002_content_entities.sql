-- +goose Up
CREATE TABLE public.content_entities (
    build_id text NOT NULL REFERENCES public.content_builds(build_id),
    entity_id text NOT NULL,
    kind text NOT NULL,
    region_id text,
    source_file text NOT NULL,
    source_line integer NOT NULL,
    payload jsonb NOT NULL,
    PRIMARY KEY (build_id, entity_id)
);

CREATE INDEX content_entities_build_kind_idx ON public.content_entities (build_id, kind);
CREATE INDEX content_entities_build_region_idx ON public.content_entities (build_id, region_id);

-- +goose Down
DROP TABLE public.content_entities;
