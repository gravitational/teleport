CREATE OR REPLACE PROCEDURE teleport_reassign_objects(username varchar, orphaned_resource_owner varchar)
LANGUAGE plpgsql
AS $$
BEGIN
    -- This is simply a stub for now. It is required because a similar
    -- procedure has been created for non-redshift postgres, and the two
    -- use the same interface to call user auto-provisioning procedures.
    RAISE NOTICE 'Object reassignment for auto-provisioned users is not currently supported for redshift.';
END;$$;
