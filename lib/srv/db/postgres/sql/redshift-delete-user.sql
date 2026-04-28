CREATE OR REPLACE PROCEDURE teleport_delete_user(username varchar, orphaned_resource_owner varchar)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Only drop if the user doesn't have other active sessions.
    -- Update to pg_stat_activity is delayed for a few hundred ms. Use
    -- stv_sessions instead:
    -- https://docs.aws.amazon.com/redshift/latest/dg/r_STV_SESSIONS.html
    IF EXISTS (SELECT user_name FROM stv_sessions WHERE user_name = CONCAT('IAM:', username)) THEN
        RAISE EXCEPTION 'TP000: User has active connections';
        RETURN;
    END IF;

    IF orphaned_resource_owner != '' THEN
        CALL teleport_reassign_objects(username, orphaned_resource_owner);
    END IF;

    BEGIN
        EXECUTE 'DROP USER IF EXISTS ' || QUOTE_IDENT(username);
    EXCEPTION WHEN OTHERS THEN
        -- Redshift only support OTHERS as exception condition, so we handle
        -- any error that might happen.

        -- Drop user/role will fail if user has dependent objects.
        -- In this scenario, fallback into disabling the user.
        CALL teleport_deactivate_user(username);
    END;
END;$$;
