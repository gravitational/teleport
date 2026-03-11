CREATE OR REPLACE PROCEDURE pg_temp.teleport_delete_user(username varchar, reassignment_user varchar, inout state varchar default 'TP003')
LANGUAGE plpgsql
AS $$
BEGIN
    -- Only drop if the user doesn't have other active sessions.
    IF EXISTS (SELECT usename FROM pg_stat_activity WHERE usename = username) THEN
        RAISE NOTICE 'User has active connections';
        RETURN;
    END IF;

    BEGIN
        IF reassignment_user != '' THEN
            EXECUTE FORMAT('REASSIGN OWNED BY %I TO %I', username, reassignment_user);
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
