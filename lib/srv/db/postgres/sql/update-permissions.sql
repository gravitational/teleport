CREATE OR REPLACE PROCEDURE pg_temp.teleport_update_permissions(username VARCHAR, permissions_ JSONB)
LANGUAGE plpgsql
AS $$
DECLARE
    grant_data JSONB;
    row_data RECORD;
    diff_removed INTEGER;
    diff_added INTEGER;
BEGIN
    grant_data = COALESCE(NULLIF(permissions_->'tables', 'null'), '[]'::JSONB);
    -- create temp table with ON COMMIT DROP so that this procedure can be
    -- retried within a single session without getting an error that the
    -- table already exists.
    CREATE TEMPORARY TABLE new_perms ON COMMIT DROP AS (
        SELECT
            item->>'schema' as table_schema,
            item->>'table' as table_name,
            item->>'privilege' as privilege_type
        FROM
            jsonb_array_elements(grant_data) as item
    );

    -- If the user has active connections to current database, verify that permissions haven't changed.
    IF EXISTS (SELECT usename FROM pg_stat_activity WHERE usename = username AND datname = current_database()) THEN
        WITH
            cur_perms AS (
                SELECT DISTINCT
                    pg_namespace.nspname::information_schema.sql_identifier AS table_schema,
                    obj.relname::information_schema.sql_identifier AS table_name,
                    acl.privilege_type::information_schema.character_data AS privilege_type
                FROM
                    pg_class as obj
                INNER JOIN
                    pg_namespace ON obj.relnamespace = pg_namespace.oid
                INNER JOIN LATERAL
                    aclexplode(COALESCE(obj.relacl, acldefault('r'::"char", obj.relowner))) AS acl ON true
                INNER JOIN
                    pg_roles AS grantee ON acl.grantee = grantee.oid
                WHERE
                    -- only objects that are one of r=ordinary table, v=view, f=foreign table, p=partitioned table.
                    (obj.relkind = ANY (ARRAY['r', 'v', 'f', 'p']))
                    -- only privileges we support provisioning.
                    AND (acl.privilege_type = ANY (ARRAY['DELETE'::text, 'INSERT'::text, 'REFERENCES'::text, 'SELECT'::text, 'TRUNCATE'::text, 'TRIGGER'::text, 'UPDATE'::text]))
                    -- only the user we are checking permissions for.
                    AND grantee.rolname = username
            ),
            removed AS (
                SELECT * FROM cur_perms
                EXCEPT
                SELECT * FROM new_perms
            ),
            added AS (
                SELECT * FROM new_perms
                EXCEPT
                SELECT * FROM cur_perms
            )
        SELECT
            (SELECT COUNT(*) FROM removed),
            (SELECT COUNT(*) FROM added)
        INTO
            diff_removed,
            diff_added;

        IF (diff_removed > 0) OR (diff_added > 0) THEN
            RAISE WARNING 'Permission changes: removed=%, added=%', diff_removed, diff_added;
            RAISE EXCEPTION SQLSTATE 'TP005' USING MESSAGE = 'TP005: User has active connections and permissions have changed';
        END IF;
    ELSE
        CALL pg_temp.teleport_remove_permissions(username);

        -- Assign all privileges to the created/activated user if grants are provided.
        FOR row_data IN SELECT * FROM new_perms
        LOOP
            EXECUTE format('GRANT %s ON %I.%I TO %I',
                        row_data.privilege_type,
                        row_data.table_schema,
                        row_data.table_name,
                        username
                    );
        END LOOP;

        -- Grant USAGE on any schemas for which we made table privilege grants
        -- See: https://github.com/gravitational/teleport/issues/51851
        FOR row_data IN (
            SELECT DISTINCT
                table_schema
            FROM
                new_perms
        )
        LOOP
            EXECUTE format('GRANT USAGE ON SCHEMA %I TO %I',
                        row_data.table_schema,
                        username
                    );
        END LOOP;
    END IF;
END;
$$;
