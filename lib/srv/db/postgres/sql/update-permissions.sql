CREATE OR REPLACE PROCEDURE teleport_update_permissions(username VARCHAR, permissions_ JSONB)
LANGUAGE plpgsql
AS $$
DECLARE
    grant_data JSONB;
    grant_item JSONB;
BEGIN
    CALL teleport_remove_permissions(username);

    -- Assign all roles to the created/activated user if grants are provided.
    grant_data = permissions_->'tables';
    IF grant_data != 'null'::jsonb THEN
        FOR grant_item IN SELECT * FROM jsonb_array_elements(grant_data)
        LOOP
            EXECUTE 'GRANT ' || text(grant_item->>'privilege') || ' ON TABLE ' || QUOTE_IDENT(grant_item->>'schema') || '.' || QUOTE_IDENT(grant_item->>'table') || ' TO ' || QUOTE_IDENT(username);
        END LOOP;
    END IF;
END;
$$;
