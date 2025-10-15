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
        FOR role_ IN
        SELECT r.rolname
        FROM pg_roles r
        WHERE r.rolname NOT IN (username, 'teleport-auto-user') AND
              r.oid IN (select m.roleid from pg_auth_members m where m.member = to_regrole(username)::oid)
        LOOP
            EXECUTE FORMAT('REVOKE %I FROM %I', role_, username);
        END LOOP;
        -- Disable ability to login for the user.
        EXECUTE FORMAT('ALTER USER %I WITH NOLOGIN', username);
    END IF;
END;$$;
