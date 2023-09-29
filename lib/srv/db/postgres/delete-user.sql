create or replace procedure teleport_delete_user(username varchar)
language plpgsql
as $$
declare
    role_ varchar;
begin
    -- Only drop if the user doesn't have other active sessions.
    if exists (select usename from pg_stat_activity where usename = username) then
        raise notice 'User has active connections';
    else
        begin
            execute format('drop user %I', username);
        exception
            when SQLSTATE '2BP01' then
                -- Drop user/role will fail if user has dependent objects.
                -- In this scenario, fallback into disabling the user.
                call teleport_deactivate_user(username);
        end;
    end if;
end;$$;
