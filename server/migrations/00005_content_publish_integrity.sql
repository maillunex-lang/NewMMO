-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION public.content_entity_fingerprint(p_build_id text)
RETURNS TABLE (entity_count bigint, counts jsonb, ingest_digest text)
LANGUAGE sql VOLATILE AS $$
    WITH entities AS MATERIALIZED (
        SELECT entity_id, kind, region_id, source_file, source_line, payload
        FROM public.content_entities WHERE build_id = p_build_id
    )
    SELECT count(*),
        (SELECT coalesce(jsonb_object_agg(kind, total), '{}'::jsonb)
            FROM (SELECT kind, count(*) AS total FROM entities GROUP BY kind) AS kinds),
        coalesce(encode(sha256(convert_to(string_agg(
            entity_id || E'\x1f' || kind || E'\x1f' || coalesce(region_id, '') || E'\x1f' ||
            source_file || E'\x1f' || source_line::text || E'\x1f' || payload::text,
            E'\x1e' ORDER BY kind, entity_id), 'UTF8')), 'hex'), '')
    FROM entities;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION public.guard_content_publish_integrity() RETURNS trigger
LANGUAGE plpgsql VOLATILE AS $$
DECLARE
    stored_count bigint;
    stored_digest text;
BEGIN
    -- Entity mutations lock the same build with FOR SHARE. This UPDATE
    -- holds the conflicting row lock before the fingerprint is read.
    SELECT entity_count, ingest_digest INTO stored_count, stored_digest
    FROM public.content_entity_fingerprint(OLD.build_id);

    IF stored_count IS DISTINCT FROM OLD.entity_count OR stored_digest IS DISTINCT FROM OLD.ingest_digest THEN
        RAISE EXCEPTION 'content build fingerprint or entity count mismatch' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER content_builds_publish_integrity
BEFORE UPDATE OF status ON public.content_builds FOR EACH ROW
WHEN (OLD.status = 'staged' AND NEW.status = 'published')
EXECUTE FUNCTION public.guard_content_publish_integrity();

-- +goose Down
DROP TRIGGER content_builds_publish_integrity ON public.content_builds;
DROP FUNCTION public.guard_content_publish_integrity();
DROP FUNCTION public.content_entity_fingerprint(text);
