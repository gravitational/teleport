CREATE OR REPLACE PROCEDURE teleport_delete_user(username varchar)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Only drop if the user doesn't have other active sessions.
    IF EXISTS (SELECT usename FROM pg_stat_activity WHERE usename = username) THEN
        RAISE NOTICE 'User has active connections';
    ELSE
        BEGIN
            EXECUTE 'DROP USER ' || QUOTE_IDENT(username);
        EXCEPTION WHEN OTHERS THEN
            -- Redshift only support OTHERS as exception condition, so we handle
            -- any error that might happen.

            -- Drop user/role will fail if user has dependent objects.
            -- In this scenario, fallback into disabling the user.
            CALL teleport_deactivate_user(username);
        END;
    END IF;
END;$$;
