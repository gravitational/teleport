create or replace procedure teleport_deactivate_user(username varchar)
language plpgsql
as $$
declare
    role_ varchar;
begin
    -- Only deactivate if the user doesn't have other active sessions.
    if exists (select usename from pg_stat_activity where usename = username) then
        raise notice 'User has active connections';
    else
        -- Revoke all role memberships except teleport-auto-user group.
        for role_ in select a.rolname from pg_roles a where pg_has_role(username, a.oid, 'member') and a.rolname not in (username, 'teleport-auto-user')
        loop
            execute format('revoke %I from %I', role_, username);
        end loop;
        -- Disable ability to login for the user.
        execute format('alter user %I with nologin', username);
    end if;
end;$$;
