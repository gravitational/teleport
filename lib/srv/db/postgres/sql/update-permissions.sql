CREATE OR REPLACE PROCEDURE pg_temp.teleport_update_permissions(username VARCHAR, permissions_ JSONB)
LANGUAGE plpgsql
AS $$
DECLARE
    grant_data JSONB;
    grant_item JSONB;
    diff_count_1 INTEGER;
    diff_count_2 INTEGER;
BEGIN
    grant_data = COALESCE(NULLIF(permissions_->'tables', 'null'), '[]'::JSONB);

    -- If the user has active connections to current database, verify that permissions haven't changed.
    IF EXISTS (SELECT usename FROM pg_stat_activity WHERE usename = username AND datname = current_database()) THEN
        CREATE TEMPORARY TABLE cur_perms AS SELECT table_schema, table_name, privilege_type FROM information_schema.table_privileges WHERE grantee = username;
        CREATE TEMPORARY TABLE new_perms AS SELECT item->>'schema' as table_schema, item->>'table' as table_name, item->>'privilege' as privilege_type FROM jsonb_array_elements(grant_data) as item;

        SELECT COUNT(*) INTO diff_count_1 FROM (SELECT * FROM cur_perms EXCEPT SELECT * FROM new_perms) q1;
        SELECT COUNT(*) INTO diff_count_2 FROM (SELECT * FROM new_perms EXCEPT SELECT * FROM cur_perms) q2;

        IF (diff_count_1 > 0) OR (diff_count_2 > 0) THEN
            RAISE WARNING 'Permission changes: removed=%, added=%', diff_count_1, diff_count_2;
            RAISE EXCEPTION SQLSTATE 'TP005' USING MESSAGE = 'TP005: User has active connections and permissions have changed';
        END IF;
    ELSE
        CALL pg_temp.teleport_remove_permissions(username);

        -- Assign all roles to the created/activated user if grants are provided.
        IF grant_data <> 'null'::JSONB THEN
            FOR grant_item IN SELECT * FROM jsonb_array_elements(grant_data)
            LOOP
                EXECUTE 'GRANT ' || text(grant_item->>'privilege') || ' ON TABLE ' || QUOTE_IDENT(grant_item->>'schema') || '.' || QUOTE_IDENT(grant_item->>'table') || ' TO ' || QUOTE_IDENT(username);
            END LOOP;
        END IF;
    END IF;
END;
$$;