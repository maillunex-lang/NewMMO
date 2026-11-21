-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION public.guard_content_entity_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    protected_status text;
BEGIN
    -- Lock both builds when moving an entity, in build ID order. FOR SHARE
    -- conflicts with a publisher changing status, so a write cannot race it.
    FOR protected_status IN
        SELECT status
        FROM public.content_builds
        WHERE build_id IN (
            CASE WHEN TG_OP <> 'INSERT' THEN OLD.build_id END,
            CASE WHEN TG_OP <> 'DELETE' THEN NEW.build_id END
        )
        ORDER BY build_id
        FOR SHARE
    LOOP
        IF protected_status <> 'staged' THEN
            RAISE EXCEPTION 'content entities are immutable for non-staged builds' USING ERRCODE = '55000';
        END IF;
    END LOOP;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER content_entities_immutability
BEFORE INSERT OR UPDATE OR DELETE ON public.content_entities
FOR EACH ROW EXECUTE FUNCTION public.guard_content_entity_mutation();

-- +goose StatementBegin
CREATE FUNCTION public.guard_content_build_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status <> 'staged' THEN
        IF TG_OP = 'DELETE' THEN
            RAISE EXCEPTION 'non-staged content builds cannot be deleted' USING ERRCODE = '55000';
        END IF;
        IF NEW.status = 'staged' OR
           (NEW.build_id, NEW.schema_version, NEW.minimum_client_version,
            NEW.manifest_sha256, NEW.entity_count, NEW.manifest, NEW.created_at)
           IS DISTINCT FROM
           (OLD.build_id, OLD.schema_version, OLD.minimum_client_version,
            OLD.manifest_sha256, OLD.entity_count, OLD.manifest, OLD.created_at) THEN
            RAISE EXCEPTION 'non-staged content build data is immutable' USING ERRCODE = '55000';
        END IF;
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER content_builds_immutability
BEFORE UPDATE OR DELETE ON public.content_builds
FOR EACH ROW EXECUTE FUNCTION public.guard_content_build_mutation();

-- +goose Down
DROP TRIGGER content_builds_immutability ON public.content_builds;
DROP FUNCTION public.guard_content_build_mutation();
DROP TRIGGER content_entities_immutability ON public.content_entities;
DROP FUNCTION public.guard_content_entity_mutation();
