CREATE OR REPLACE PROCEDURE pg_temp.teleport_reassign_objects(source_user varchar, destination_user varchar)
LANGUAGE plpgsql
AS $$
DECLARE
    user_oid oid;
    current_db_oid oid;
    unsupported_objects text[];
    non_bare_tables text[];
    problematic_schemas text[];
BEGIN
    SELECT oid INTO user_oid FROM pg_roles WHERE rolname = source_user;
    IF user_oid IS NULL THEN
        RAISE WARNING 'User % not found, skipping object reassignment', source_user;
        RETURN;
    END IF;

    SELECT oid INTO current_db_oid FROM pg_database WHERE datname = current_database();

    -- Use pg_shdepend (deptype='o') as the authoritative, future-proof source
    -- for all objects owned by the user. REASSIGN OWNED BY uses the same
    -- mechanism internally: any new PostgreSQL object type must call
    -- recordDependencyOnOwner(), which inserts into pg_shdepend.
    SELECT ARRAY_AGG(
        COALESCE(c.relname, n.nspname, p.proname, t.typname, sd.objid::text)
        ORDER BY 1
    ) INTO unsupported_objects
    FROM pg_shdepend sd
    LEFT JOIN pg_class     c ON sd.classid = 'pg_class'::regclass     AND c.oid = sd.objid
    LEFT JOIN pg_namespace n ON sd.classid = 'pg_namespace'::regclass AND n.oid = sd.objid
    LEFT JOIN pg_proc      p ON sd.classid = 'pg_proc'::regclass      AND p.oid = sd.objid
    LEFT JOIN pg_type      t ON sd.classid = 'pg_type'::regclass      AND t.oid = sd.objid
    WHERE sd.refobjid = user_oid
    AND sd.refclassid = 'pg_authid'::regclass
    AND sd.deptype = 'o'
    AND sd.dbid = current_db_oid
    AND NOT (
        -- Supported: tables, sequences, indexes, TOAST tables
        (sd.classid = 'pg_class'::regclass AND c.relkind IN ('r', 'S', 'i', 't'))
        OR
        -- Supported: schemas (safety-checked separately below)
        (sd.classid = 'pg_namespace'::regclass)
        OR
        -- Composite row types are reassigned automatically by REASSIGN OWNED BY;
        -- auto-created array types are suppressed in favour of their base type.
        (sd.classid = 'pg_type'::regclass AND (
            (t.typtype = 'c' AND t.typrelid != 0)
            OR t.typname LIKE '\_%'
        ))
    );

    IF unsupported_objects IS NOT NULL THEN
        RAISE WARNING 'User % owns object(s) of unsupported type, skipping object reassignment: %',
            source_user, array_to_string(unsupported_objects, ', ');
        RETURN;
    END IF;

    -- Check tables for non-bare conditions: partition children, RLS-enabled
    -- tables, and tables with SECURITY DEFINER trigger functions.
    SELECT ARRAY_AGG(DISTINCT c.relname ORDER BY c.relname) INTO non_bare_tables
    FROM pg_shdepend sd
    JOIN pg_class c ON c.oid = sd.objid
    WHERE sd.refobjid = user_oid
    AND sd.refclassid = 'pg_authid'::regclass
    AND sd.deptype = 'o'
    AND sd.dbid = current_db_oid
    AND sd.classid = 'pg_class'::regclass
    AND c.relkind = 'r'
    AND (
        c.relispartition
        OR c.relrowsecurity
        OR EXISTS (
            SELECT 1 FROM pg_trigger tg
            JOIN pg_proc p ON p.oid = tg.tgfoid
            WHERE tg.tgrelid = c.oid
            AND NOT tg.tgisinternal
            AND p.prosecdef = true
        )
    );

    IF non_bare_tables IS NOT NULL THEN
        RAISE WARNING 'User % owns non-bare table(s), skipping object reassignment: %',
            source_user, array_to_string(non_bare_tables, ', ');
        RETURN;
    END IF;

    -- Check schemas for unsafe reassignment. The "public" schema is in most
    -- users' default search_path; owning it would let teleport-object-inheritor
    -- members plant shadow objects. System schemas are a sanity check.
    SELECT ARRAY_AGG(n.nspname ORDER BY n.nspname) INTO problematic_schemas
    FROM pg_shdepend sd
    JOIN pg_namespace n ON n.oid = sd.objid
    WHERE sd.refobjid = user_oid
    AND sd.refclassid = 'pg_authid'::regclass
    AND sd.deptype = 'o'
    AND sd.dbid = current_db_oid
    AND sd.classid = 'pg_namespace'::regclass
    AND (n.nspname = 'public' OR n.nspname LIKE 'pg_%' OR n.nspname = 'information_schema');

    IF problematic_schemas IS NOT NULL THEN
        RAISE WARNING 'User % owns schema(s) that cannot be safely reassigned, skipping object reassignment: %',
            source_user, array_to_string(problematic_schemas, ', ');
        RETURN;
    END IF;

    -- All checks passed: every owned object is safe to reassign.
    BEGIN
        EXECUTE FORMAT('REASSIGN OWNED BY %I TO %I', source_user, destination_user
    );
    EXCEPTION
        WHEN others THEN
            RAISE WARNING 'Failed to reassign object ownership from % to %: %, SQLSTATE=%',
                source_user, destination_user
            , SQLERRM, SQLSTATE;
    END;
END;$$;

