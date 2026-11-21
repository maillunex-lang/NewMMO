-- +goose Up
CREATE TABLE public.content_builds (
    build_id text PRIMARY KEY CHECK (build_id ~ '^[0-9a-f]{64}$'),
    schema_version integer NOT NULL,
    minimum_client_version text NOT NULL,
    manifest_sha256 char(64) NOT NULL CHECK (manifest_sha256 = build_id),
    entity_count integer NOT NULL CHECK (entity_count > 0),
    manifest jsonb NOT NULL,
    status text NOT NULL CHECK (status IN ('staged', 'published', 'retired')),
    created_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    retired_at timestamptz
);

-- +goose Down
DROP TABLE public.content_builds;
