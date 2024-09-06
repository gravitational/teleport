CREATE OR REPLACE PROCEDURE teleport_activate_user(username varchar, roles text)
LANGUAGE plpgsql
AS $$
DECLARE
    roles_length integer;
    cur_roles_length integer;
BEGIN
    roles_length := JSON_ARRAY_LENGTH(roles);

    -- If the user already exists and was provisioned by Teleport, reactivate
    -- it, otherwise provision a new one.
    IF EXISTS (SELECT user_id FROM svv_user_grants WHERE user_name = username AND admin_option = false AND role_name = 'teleport-auto-user') THEN
        -- If the user has active connections, make sure the provided roles
        -- match what the user currently has. Update to pg_stat_activity is
        -- delayed for a few hundred ms. Use stv_sessions instead:
        -- https://docs.aws.amazon.com/redshift/latest/dg/r_STV_SESSIONS.html
        IF EXISTS (SELECT user_name FROM stv_sessions WHERE user_name = CONCAT('IAM:', username)) THEN
          SELECT INTO cur_roles_length COUNT(role_name) FROM svv_user_grants WHERE user_name = username AND admin_option=false AND role_name != 'teleport-auto-user';
          IF roles_length != cur_roles_length THEN
            RAISE EXCEPTION 'TP002: User has active connections and roles have changed';
          END IF;
          FOR i IN 0..roles_length-1 LOOP
            IF NOT EXISTS (SELECT role_name FROM svv_user_grants WHERE user_name = username AND admin_option=false AND role_name = JSON_EXTRACT_ARRAY_ELEMENT_TEXT(roles,i)) THEN
                RAISE EXCEPTION 'TP002: User has active connections and roles have changed';
            END IF;
          END LOOP;
          RETURN;
        END IF;
        -- Otherwise reactivate the user, but first strip it of all roles to
        -- account for scenarios with left-over roles if database agent crashed
        -- and failed to cleanup upon session termination.
        CALL teleport_deactivate_user(username);
        EXECUTE 'ALTER USER ' || QUOTE_IDENT(username) || ' CONNECTION LIMIT UNLIMITED';
    ELSE
        EXECUTE 'CREATE USER ' || QUOTE_IDENT(username) || ' WITH PASSWORD DISABLE';
        EXECUTE 'GRANT ROLE "teleport-auto-user" TO ' || QUOTE_IDENT(username);
    END IF;
    -- Assign all roles to the created/activated user.
    FOR i in 0..roles_length-1 LOOP
        EXECUTE 'GRANT ROLE ' || QUOTE_IDENT(JSON_EXTRACT_ARRAY_ELEMENT_TEXT(roles,i)) || ' TO ' || QUOTE_IDENT(username);
    END LOOP;
END;$$;
