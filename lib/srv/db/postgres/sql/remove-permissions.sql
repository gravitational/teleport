CREATE OR REPLACE PROCEDURE teleport_remove_permissions(username VARCHAR)
LANGUAGE plpgsql
AS $$
DECLARE
    schema_name_ VARCHAR;
BEGIN
    -- Loop through all schemas
    FOR schema_name_ IN (SELECT schema_name FROM information_schema.schemata)
    LOOP
        EXECUTE 'REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA ' || quote_ident(schema_name_) || ' FROM ' || quote_ident(username);
    END LOOP;
END;
$$;