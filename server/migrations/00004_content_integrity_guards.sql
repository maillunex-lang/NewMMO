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
    IF TG_OP = 'INSERT' THEN
        IF NEW.status <> 'staged' THEN
            RAISE EXCEPTION 'new content builds must be staged' USING ERRCODE = '55000';
        END IF;
        RETURN NEW;
    END IF;

    IF TG_OP = 'DELETE' THEN
        IF OLD.status <> 'staged' THEN
            RAISE EXCEPTION 'non-staged content builds cannot be deleted' USING ERRCODE = '55000';
        END IF;
        RETURN OLD;
    END IF;

    IF (NEW.build_id, NEW.schema_version, NEW.minimum_client_version, NEW.manifest_sha256,
        NEW.entity_count, NEW.ingest_digest, NEW.manifest, NEW.created_at)
       IS DISTINCT FROM
       (OLD.build_id, OLD.schema_version, OLD.minimum_client_version, OLD.manifest_sha256,
        OLD.entity_count, OLD.ingest_digest, OLD.manifest, OLD.created_at) THEN
        RAISE EXCEPTION 'content build data is immutable' USING ERRCODE = '55000';
    END IF;

    IF NOT ((OLD.status IN ('staged', 'retired') AND NEW.status = 'published') OR
            (OLD.status = 'published' AND NEW.status = 'retired')) THEN
        RAISE EXCEPTION 'invalid content build transition' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER content_builds_immutability
BEFORE INSERT OR UPDATE OR DELETE ON public.content_builds
FOR EACH ROW EXECUTE FUNCTION public.guard_content_build_mutation();

-- +goose StatementBegin
CREATE FUNCTION public.guard_content_active() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.id IS DISTINCT FROM OLD.id OR NEW.build_id IS NULL OR NEW.build_id IS NOT DISTINCT FROM OLD.build_id
       OR NEW.previous_build_id IS DISTINCT FROM OLD.build_id OR NOT EXISTS (
           SELECT 1 FROM public.content_builds WHERE build_id = NEW.build_id AND status = 'published'
       ) THEN
        RAISE EXCEPTION 'invalid active content pointer' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER content_active_guard
BEFORE UPDATE ON public.content_active FOR EACH ROW EXECUTE FUNCTION public.guard_content_active();

-- +goose StatementBegin
CREATE FUNCTION public.reject_content_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'content registry operation is forbidden' USING ERRCODE = '55000';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER content_active_no_delete
BEFORE DELETE ON public.content_active FOR EACH ROW EXECUTE FUNCTION public.reject_content_mutation();
CREATE TRIGGER content_audit_append_only
BEFORE UPDATE OR DELETE ON public.content_audit FOR EACH ROW EXECUTE FUNCTION public.reject_content_mutation();
CREATE TRIGGER content_builds_no_truncate
BEFORE TRUNCATE ON public.content_builds FOR EACH STATEMENT EXECUTE FUNCTION public.reject_content_mutation();
CREATE TRIGGER content_entities_no_truncate
BEFORE TRUNCATE ON public.content_entities FOR EACH STATEMENT EXECUTE FUNCTION public.reject_content_mutation();
CREATE TRIGGER content_active_no_truncate
BEFORE TRUNCATE ON public.content_active FOR EACH STATEMENT EXECUTE FUNCTION public.reject_content_mutation();
CREATE TRIGGER content_audit_no_truncate
BEFORE TRUNCATE ON public.content_audit FOR EACH STATEMENT EXECUTE FUNCTION public.reject_content_mutation();

GRANT SELECT ON public.content_builds, public.content_entities, public.content_active, public.content_audit
    TO newmmo_app, newmmo_content_publisher;
GRANT INSERT ON public.content_builds, public.content_audit TO newmmo_content_publisher;
GRANT INSERT, DELETE ON public.content_entities TO newmmo_content_publisher;
GRANT UPDATE (status, published_at, retired_at) ON public.content_builds TO newmmo_content_publisher;
GRANT UPDATE (build_id, previous_build_id, switched_at, switched_by) ON public.content_active TO newmmo_content_publisher;
GRANT USAGE ON SEQUENCE public.content_audit_audit_id_seq TO newmmo_content_publisher;

-- +goose Down
REVOKE USAGE ON SEQUENCE public.content_audit_audit_id_seq FROM newmmo_content_publisher;
REVOKE UPDATE (build_id, previous_build_id, switched_at, switched_by) ON public.content_active FROM newmmo_content_publisher;
REVOKE UPDATE (status, published_at, retired_at) ON public.content_builds FROM newmmo_content_publisher;
REVOKE INSERT, DELETE ON public.content_entities FROM newmmo_content_publisher;
REVOKE INSERT ON public.content_builds, public.content_audit FROM newmmo_content_publisher;
REVOKE SELECT ON public.content_builds, public.content_entities, public.content_active, public.content_audit
    FROM newmmo_app, newmmo_content_publisher;

DROP TRIGGER content_audit_no_truncate ON public.content_audit;
DROP TRIGGER content_active_no_truncate ON public.content_active;
DROP TRIGGER content_entities_no_truncate ON public.content_entities;
DROP TRIGGER content_builds_no_truncate ON public.content_builds;
DROP TRIGGER content_audit_append_only ON public.content_audit;
DROP TRIGGER content_active_no_delete ON public.content_active;
DROP FUNCTION public.reject_content_mutation();
DROP TRIGGER content_active_guard ON public.content_active;
DROP FUNCTION public.guard_content_active();
DROP TRIGGER content_builds_immutability ON public.content_builds;
DROP FUNCTION public.guard_content_build_mutation();
DROP TRIGGER content_entities_immutability ON public.content_entities;
DROP FUNCTION public.guard_content_entity_mutation();
