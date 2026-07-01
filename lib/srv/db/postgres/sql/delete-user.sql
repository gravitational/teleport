CREATE OR REPLACE PROCEDURE pg_temp.teleport_delete_user(username varchar, object_inheritor_user varchar, inout state varchar default 'TP003')
LANGUAGE plpgsql
AS $$
DECLARE
    has_active_connections_to_current_db boolean;
    has_active_connections_anywhere boolean;
BEGIN
    -- Prevent any new sessions for the user in any database while the
    -- procedure runs, to prevent them creating (and thus owning) an object
    -- after reassignment is done.
    EXECUTE FORMAT('ALTER USER %I WITH NOLOGIN', username);
    COMMIT;

    SELECT
        COUNT(*) FILTER (WHERE datname = current_database()) > 0,
        COUNT(*) > 0
    INTO has_active_connections_to_current_db,
         has_active_connections_anywhere
    FROM pg_stat_activity
    WHERE usename = username;

    IF has_active_connections_to_current_db THEN
        EXECUTE FORMAT('ALTER USER %I WITH LOGIN', username);
        RAISE NOTICE 'User % has active connections on database %', username, current_database();
        state := 'TP000';
        RETURN;
    END IF;

    BEGIN
        BEGIN
            IF object_inheritor_user != '' THEN
                CALL teleport_objects.teleport_reassign_objects(username, object_inheritor_user);
            END IF;
        EXCEPTION
            WHEN OTHERS THEN
                RAISE WARNING 'Failed to reassign objects owned by user %: %, SQLSTATE=%', username, SQLERRM, SQLSTATE;
        END;

        IF has_active_connections_anywhere THEN
            EXECUTE FORMAT('ALTER USER %I WITH LOGIN', username);
            RAISE NOTICE 'User % has active connections', username;
            state := 'TP000';
            RETURN;
        END IF;

        EXECUTE FORMAT('DROP USER IF EXISTS %I', username);
    EXCEPTION
        WHEN SQLSTATE '2BP01' THEN
            state := 'TP004';
            -- Drop user/role will fail if user still has dependent objects.
            -- In this scenario, fallback into disabling the user.
            CALL pg_temp.teleport_deactivate_user(username);
    END;
END;$$;
