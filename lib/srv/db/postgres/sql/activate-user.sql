create or replace procedure teleport_activate_user(username varchar, roles varchar[])
language plpgsql
as $$
declare
    role_ varchar;
    cur_roles varchar[];
begin
    -- If the user already exists and was provisioned by Teleport, reactivate
    -- it, otherwise provision a new one.
    if exists (select * from pg_auth_members where roleid = (select oid from pg_roles where rolname = 'teleport-auto-user') and member = (select oid from pg_roles where rolname = username)) then
        -- If the user has active connections, make sure the provided roles
        -- match what the user currently has.
        if exists (select usename from pg_stat_activity where usename = username) then
            select cast(array_agg(rolname) as varchar[]) into cur_roles from pg_auth_members join pg_roles on roleid = oid where member=(select oid from pg_roles where rolname = username) and rolname != 'teleport-auto-user';
            if roles <@ cur_roles AND cur_roles <@roles then
                return;
            end if;
            RAISE EXCEPTION SQLSTATE 'TP002' USING MESSAGE = 'TP002: User has active connections and roles have changed';
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
