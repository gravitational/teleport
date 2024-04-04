CREATE OR REPLACE PROCEDURE teleport_delete_user(username varchar, inout state varchar default 'TP003')
LANGUAGE plpgsql
AS $$
DECLARE
    role_ varchar;
BEGIN
    -- Only drop if the user doesn't have other active sessions.
    IF EXISTS (SELECT usename FROM pg_stat_activity WHERE usename = username) THEN
        RAISE NOTICE 'User has active connections';
    ELSE
        BEGIN
            EXECUTE FORMAT('DROP USER IF EXISTS %I', username);
        EXCEPTION
            WHEN SQLSTATE '2BP01' THEN
                state := 'TP004';
                -- Drop user/role will fail if user has dependent objects.
                -- In this scenario, fallback into disabling the user.
                CALL teleport_deactivate_user(username);
        END;
    END IF;
END;$$;
