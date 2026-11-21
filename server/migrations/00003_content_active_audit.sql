-- +goose Up
CREATE TABLE public.content_active (
    id smallint PRIMARY KEY CHECK (id = 1),
    build_id text NOT NULL REFERENCES public.content_builds(build_id),
    previous_build_id text REFERENCES public.content_builds(build_id),
    switched_at timestamptz NOT NULL DEFAULT now(),
    switched_by text NOT NULL
);

CREATE TABLE public.content_audit (
    audit_id bigserial PRIMARY KEY,
    at timestamptz NOT NULL DEFAULT now(),
    action text NOT NULL CHECK (action IN ('stage', 'reject', 'publish', 'rollback')),
    build_id text,
    previous_build_id text,
    actor text NOT NULL,
    details jsonb NOT NULL DEFAULT '{}'::jsonb
);

-- +goose Down
DROP TABLE public.content_audit;
DROP TABLE public.content_active;
