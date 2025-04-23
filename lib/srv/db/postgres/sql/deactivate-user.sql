CREATE OR REPLACE PROCEDURE pg_temp.teleport_deactivate_user(username varchar)
LANGUAGE plpgsql
AS $$
DECLARE
    role_ varchar;
BEGIN
    -- Only deactivate if the user doesn't have other active sessions.
    IF EXISTS (SELECT usename FROM pg_stat_activity WHERE usename = username) THEN
        RAISE NOTICE 'User has active connections';
    ELSE
        -- Revoke all role memberships except teleport-auto-user group.
        FOR role_ IN SELECT a.rolname FROM pg_roles a WHERE pg_has_role(username, a.oid, 'member') AND a.rolname NOT IN (username, 'teleport-auto-user')
        LOOP
            EXECUTE FORMAT('REVOKE %I FROM %I', role_, username);
        END LOOP;
        -- Disable ability to login for the user.
        EXECUTE FORMAT('ALTER USER %I WITH NOLOGIN', username);
    END IF;
END;$$;
