CREATE OR REPLACE PROCEDURE pg_temp.teleport_reassign_objects(username varchar, orphaned_resource_owner varchar)
LANGUAGE plpgsql
AS $$
DECLARE
    admin_user_is_member_of_orphaned_resource_owner boolean;
    admin_user_is_superuser boolean;
    security_definer_count integer;
BEGIN
    -- For REASSIGN OWNED BY to work, two preconditions are required:
    -- 1. The admin user must be a member of orphaned_resource_owner. This permission
    --    must exist prior to invoking this procedure because it has a lifecycle that
    --    is separate from a single user session.
    -- 2. The admin user must be a member of username. This permission is only necessary
    --    for this specific procedure, so it is granted and revoked in the nested
    --    transaction block below.

    -- Raise warning and return without reassigning if orphaned_resource_owner
    -- does not exist as a role.
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = orphaned_resource_owner) THEN
        RAISE WARNING 'orphaned_resource_owner (%) does not exist as a database role. Skipping object reassignment.', orphaned_resource_owner;
        RETURN;
    END IF;

    -- Raise warning and return without reassigning if admin user does not have
    -- sufficient privileges.
    SELECT pg_has_role(CURRENT_USER, orphaned_resource_owner, 'MEMBER')
    INTO admin_user_is_member_of_orphaned_resource_owner;
    SELECT rolsuper FROM pg_roles WHERE rolname = CURRENT_USER
    INTO admin_user_is_superuser;
    IF NOT admin_user_is_member_of_orphaned_resource_owner AND NOT admin_user_is_superuser THEN
        RAISE WARNING 'Admin user (%) must either be a superuser, or be a member of orphaned_resource_owner (%) to reassign ownership of database objects to it.', CURRENT_USER, orphaned_resource_owner;
        RETURN;
    END IF;

    -- Raise warning and return without reassigning if the user owns any SECURITY
    -- DEFINER routines. Reassigning such routines would allow attacker-controlled
    -- code to execute with orphaned_resource_owner's privileges.
    SELECT COUNT(*) INTO security_definer_count
    FROM pg_proc
    WHERE proowner = (SELECT oid FROM pg_roles WHERE rolname = username)
    AND prosecdef = true;
    IF security_definer_count > 0 THEN
        RAISE WARNING 'User % owns % SECURITY DEFINER routine(s). Skipping object reassignment to prevent privilege escalation.', username, security_definer_count;
        RETURN;
    END IF;

    -- We use a nested transaction block here so that permissions are never left
    -- dangling, even if one of the statements fails.
    BEGIN
        EXECUTE FORMAT('GRANT %I TO CURRENT_USER', username);
        EXECUTE FORMAT('REASSIGN OWNED BY %I TO %I', username, orphaned_resource_owner);
        EXECUTE FORMAT('REVOKE %I FROM CURRENT_USER', username);
    EXCEPTION
        WHEN others THEN
            RAISE WARNING 'Failed to reassign object ownership from % to %: %, SQLSTATE=%', username, orphaned_resource_owner, SQLERRM, SQLSTATE;
    END;
END;$$;
