CREATE OR REPLACE PROCEDURE teleport_deactivate_user(username varchar)
LANGUAGE plpgsql
AS $$
DECLARE
    rec record;
BEGIN
    -- Only deactivate if the user doesn't have other active sessions.
    -- Update to pg_stat_activity is delayed for a few hundred ms. Use
    -- stv_sessions instead:
    -- https://docs.aws.amazon.com/redshift/latest/dg/r_STV_SESSIONS.html
    IF EXISTS (SELECT user_name FROM stv_sessions WHERE user_name = CONCAT('IAM:', username)) THEN
        RAISE EXCEPTION 'TP000: User has active connections';
    ELSE
        -- Disable ability to login for the user.
        -- We do this before revoking roles so that the pg_shadow table
        -- (oid 1260) lock is acquired before the pg_role (oid 4775) and
        -- pg_identity (oid 4771) table locks, so that the locks are acquired in
        -- the same order in the activate/deactivate/delete procedures.
        EXECUTE 'ALTER USER ' || QUOTE_IDENT(username) || ' WITH CONNECTION LIMIT 0';
        -- Revoke all role memberships except teleport-auto-user.
        FOR rec IN select role_name FROM svv_user_grants WHERE user_name = username AND admin_option = false AND role_name != 'teleport-auto-user' LOOP
             EXECUTE 'REVOKE ROLE ' || QUOTE_IDENT(rec.role_name) || ' FROM ' || QUOTE_IDENT(username);
        END LOOP;
    END IF;
END;$$;
