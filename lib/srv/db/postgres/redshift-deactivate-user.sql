create or replace procedure teleport_deactivate_user(username varchar)
language plpgsql
as $$
declare
    rec record;
begin
    -- Only deactivate if the user doesn't have other active sessions.
    -- Update to pg_stat_activity is delayed for a few hundred ms. Use
    -- stv_sessions instead.
    if exists (select user_name from stv_sessions where user_name = concat('IAM:', username)) then
        raise exception 'User has active connections';
    else
        -- Revoke all role memberships except teleport-auto-user.
        for rec in select role_name from svv_user_grants where user_name = username and admin_option = false and role_name != 'teleport-auto-user'
        loop
             execute 'revoke role ' || quote_ident(rec.role_name) || ' from ' || quote_ident(username);
        end loop;
        -- Disable ability to login for the user.
        execute 'alter user ' || quote_ident(username) || 'with connection limit 0';
    end if;
end;$$;
