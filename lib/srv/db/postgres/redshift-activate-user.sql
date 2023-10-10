create or replace procedure teleport_activate_user(username varchar, roles text)
language plpgsql
as $$
declare
    roles_length integer;
    cur_roles_length integer;
begin
    roles_length := JSON_ARRAY_LENGTH(roles);

    -- If the user already exists and was provisioned by Teleport, reactivate
    -- it, otherwise provision a new one.
    if exists (select user_id from svv_user_grants where user_name = username and admin_option = false and role_name = 'teleport-auto-user') then
        -- If the user has active connections, make sure the provided roles
        -- match what the user currently has.
        if exists (select user_name from stv_sessions where user_name = concat('IAM:', username)) then
          select into cur_roles_length count(role_name) from svv_user_grants where user_name = username and admin_option=false and role_name != 'teleport-auto-user';
          if roles_length != cur_roles_length then
            raise exception 'User has active connections and roles have changed';
          end if;
          for i in 0..roles_length-1 loop
            if not exists (select role_name from svv_user_grants where user_name = username and admin_option=false and role_name = JSON_EXTRACT_ARRAY_ELEMENT_TEXT(roles,i)) then
                raise exception 'User has active connections and roles have changed';
            end if;
          end loop;
          return;
        end if;
        -- Otherwise reactivate the user, but first strip if of all roles to
        -- account for scenarios with left-over roles if database agent crashed
        -- and failed to cleanup upon session termination.
        call teleport_deactivate_user(username);
        execute 'alter user ' || quote_ident(username) || ' connection limit UNLIMITED';
    else
        execute 'create user ' || quote_ident(username) || 'with password disable';
        execute 'grant role "teleport-auto-user" to ' || quote_ident(username);
    end if;
    -- Assign all roles to the created/activated user.
    for i in 0..roles_length-1 loop
        execute 'grant role ' || quote_ident(JSON_EXTRACT_ARRAY_ELEMENT_TEXT(roles,i)) || ' to ' || quote_ident(username);
    end loop;
end;$$;
