CREATE OR REPLACE PROCEDURE pg_temp.teleport_remove_permissions(username VARCHAR)
LANGUAGE plpgsql
AS $$
DECLARE
    row_data RECORD;
BEGIN
    -- Check active connections.
    IF EXISTS (SELECT usename FROM pg_stat_activity WHERE usename = username AND datname = current_database()) THEN
       RAISE NOTICE 'User has active connections to current database';
    ELSE
        -- create temp table with ON COMMIT DROP so that this procedure can be
        -- retried within a single session without getting an error that the
        -- table already exists.
        CREATE TEMPORARY TABLE cur_perms ON COMMIT DROP AS (
            SELECT DISTINCT
                pg_namespace.nspname::information_schema.sql_identifier AS table_schema,
                obj.relname::information_schema.sql_identifier AS table_name
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
                -- only the user in we are revoking permissions from.
                AND grantee.rolname = username
        );

        -- Loop through and revoke all table permissions
        FOR row_data IN (
            SELECT
                table_schema,
                table_name
            FROM
                cur_perms
        )
        LOOP
            EXECUTE format('REVOKE ALL PRIVILEGES ON %I.%I FROM %I',
                        row_data.table_schema,
                        row_data.table_name,
                        username
                    );
        END LOOP;

        -- We implicitly grant USAGE on any schema for which we make table privilege grants.
        -- Loop through and revoke all schema USAGE.
        -- See: https://github.com/gravitational/teleport/issues/51851
        FOR row_data IN (
            SELECT DISTINCT
                table_schema
            FROM
                cur_perms
        )
        LOOP
            EXECUTE format('REVOKE USAGE ON SCHEMA %I FROM %I',
                        row_data.table_schema,
                        username
                    );
        END LOOP;
    END IF;
END;
$$;
