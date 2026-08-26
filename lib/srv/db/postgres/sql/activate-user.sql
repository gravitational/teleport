CREATE OR REPLACE PROCEDURE pg_temp.teleport_activate_user(username varchar, roles varchar[])
LANGUAGE plpgsql
AS $$
DECLARE
    role_ varchar;
    cur_roles varchar[];
BEGIN
    -- If the user already exists and was provisioned by Teleport, reactivate
    -- it, otherwise provision a new one.
    IF EXISTS (SELECT * FROM pg_auth_members WHERE roleid = (SELECT oid FROM pg_roles WHERE rolname = 'teleport-auto-user') and member = (SELECT oid FROM pg_roles WHERE rolname = username)) THEN
        -- If the user has active connections, make sure the provided roles
        -- match what the user currently has.
        IF EXISTS (SELECT usename FROM pg_stat_activity WHERE usename = username) THEN
            SELECT CAST(array_agg(rolname) as varchar[]) INTO cur_roles FROM pg_auth_members JOIN pg_roles ON roleid = pg_roles.oid WHERE member=(SELECT oid FROM pg_roles WHERE rolname = username) AND rolname != 'teleport-auto-user';
            -- both `cur_roles` and `roles` may be NULL; we want to work with empty arrays instead.
            cur_roles := COALESCE(cur_roles, ARRAY[]::varchar[]);
            roles := COALESCE(roles, ARRAY[]::varchar[]);
            -- "a <@ b" checks if all unique elements in "a" are contained by
            -- "b". Using length check plus "contains" check to avoid sorting.
            IF cardinality(roles) = cardinality(cur_roles) AND roles <@ cur_roles THEN
                RETURN;
            END IF;
            RAISE EXCEPTION SQLSTATE 'TP002' USING MESSAGE = 'TP002: User has active connections and roles have changed';
        END IF;
        -- Otherwise reactivate the user, but first strip if of all roles to
        -- account for scenarios with left-over roles if database agent crashed
        -- and failed to cleanup upon session termination.
        CALL pg_temp.teleport_deactivate_user(username);
        EXECUTE FORMAT('ALTER USER %I WITH LOGIN', username);
    ELSE
        EXECUTE FORMAT('CREATE USER %I IN ROLE "teleport-auto-user"', username);
    END IF;
    -- Assign all roles to the created/activated user.
    FOREACH role_ IN ARRAY roles
    LOOP
        EXECUTE FORMAT('GRANT %I TO %I', role_, username);
    END LOOP;
END;$$;
