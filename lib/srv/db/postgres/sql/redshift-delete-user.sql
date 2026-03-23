CREATE OR REPLACE PROCEDURE teleport_delete_user(username varchar, reassignment_user varchar)
LANGUAGE plpgsql
AS $$
BEGIN
    -- Note: reassignment_user is not used because redshift does not support
    -- REASSIGN OWNED BY. It is here because redshift and postgres are called
    -- via the same procedure, so it is required to avoid errors.

    -- Only drop if the user doesn't have other active sessions.
    -- Update to pg_stat_activity is delayed for a few hundred ms. Use
    -- stv_sessions instead:
    -- https://docs.aws.amazon.com/redshift/latest/dg/r_STV_SESSIONS.html
    IF EXISTS (SELECT user_name FROM stv_sessions WHERE user_name = CONCAT('IAM:', username)) THEN
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
