CREATE OR REPLACE PROCEDURE teleport_delete_user(username varchar)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Only drop if the user doesn't have other active sessions.
    -- Use the user ID because stv_sessions truncates IAM-prefixed usernames
    -- to 50 characters.
    -- https://docs.aws.amazon.com/redshift/latest/dg/SYS_SESSION_HISTORY.html
    IF EXISTS (SELECT 1 FROM sys_session_history WHERE user_id = (SELECT usesysid FROM pg_user WHERE usename = username) AND status = 'active') THEN
        RAISE EXCEPTION 'TP000: User has active connections';
    ELSE
        BEGIN
            EXECUTE 'DROP USER IF EXISTS ' || QUOTE_IDENT(username);
        EXCEPTION WHEN OTHERS THEN
            -- Redshift only support OTHERS as exception condition, so we handle
            -- any error that might happen.

            -- Drop user/role will fail if user has dependent objects.
            -- In this scenario, fallback into disabling the user.
            CALL teleport_deactivate_user(username);
        END;
    END IF;
END;$$;
