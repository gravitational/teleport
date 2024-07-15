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
        -- Loop through table permissions
        FOR row_data IN (SELECT DISTINCT table_schema, table_name FROM information_schema.table_privileges WHERE grantee = username)
        LOOP
            EXECUTE 'REVOKE ALL PRIVILEGES ON ' || quote_ident(row_data.table_schema) || '.' || quote_ident(row_data.table_name) || ' FROM ' || quote_ident(username);
        END LOOP;
    END IF;
END;
$$;