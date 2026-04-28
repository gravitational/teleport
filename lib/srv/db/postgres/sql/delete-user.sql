CREATE OR REPLACE PROCEDURE pg_temp.teleport_delete_user(username varchar, orphaned_resource_owner varchar, inout state varchar default 'TP003')
LANGUAGE plpgsql
AS $$
DECLARE
    has_active_connections_to_current_db boolean;
    has_active_connections_anywhere boolean;
BEGIN
    SELECT
        COUNT(*) FILTER (WHERE datname = current_database()) > 0,
        COUNT(*) > 0
    INTO has_active_connections_to_current_db,
         has_active_connections_anywhere
    FROM pg_stat_activity
    WHERE usename = username;

    IF NOT has_active_connections_to_current_db AND orphaned_resource_owner != '' THEN
        CALL pg_temp.teleport_reassign_objects(username, orphaned_resource_owner);
    END IF;

    IF has_active_connections_anywhere THEN
        RAISE NOTICE 'User % has active connections', username;
        state := 'TP000';
        RETURN;
    END IF;

    BEGIN
        EXECUTE FORMAT('DROP USER IF EXISTS %I', username);
    EXCEPTION
        WHEN SQLSTATE '2BP01' THEN
            state := 'TP004';
            -- Drop user/role will fail if user still has dependent objects.
            -- In this scenario, fallback into disabling the user.
            CALL pg_temp.teleport_deactivate_user(username);
    END;
END;$$;
