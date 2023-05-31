create or replace procedure teleport_activate_user(username varchar, roles varchar[])
language plpgsql
as $$
declare
    role_ varchar;
begin
    -- If the user already exists and was provisioned by Teleport, reactivate
    -- it, otherwise provision a new one.
    if exists (select * from pg_auth_members where roleid = (select oid from pg_roles where rolname = 'teleport-auto-user') and member = (select oid from pg_roles where rolname = username)) then
        -- If the user has active connections, just use it to avoid messing up
        -- its existing roles.
        if exists (select usename from pg_stat_activity where usename = username) then
          return;
        end if;
        -- Otherwise reactivate the user, but first strip if of all roles to
        -- account for scenarios with left-over roles if database agent crashed
        -- and failed to cleanup upon session termination.
        call teleport_deactivate_user(username);
        execute format('alter user %I with login', username);
    else
        execute format('create user %I in role "teleport-auto-user"', username);
    end if;
    -- Assign all roles to the created/activated user.
    foreach role_ in array roles
    loop
        execute format('grant %I to %I', role_, username);
    end loop;
end;$$;
