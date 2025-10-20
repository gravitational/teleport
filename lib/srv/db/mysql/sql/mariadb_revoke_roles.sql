CREATE PROCEDURE teleport_revoke_roles(IN username TEXT)
BEGIN
    DECLARE cur_user TEXT;
    DECLARE cur_role TEXT;
    DECLARE done INT DEFAULT FALSE;
    -- Revoke all roles assigned to the all-in-one role, and all roles assigned
    -- to the username (expect 'teleport-auto-user')
    DECLARE role_cursor CURSOR FOR
        (SELECT User,Role FROM mysql.roles_mapping WHERE User = CONCAT("tp-role-", username) AND Admin_option = 'N')
        UNION
        (SELECT User,Role FROM mysql.roles_mapping WHERE Role != 'teleport-auto-user' AND User = username AND Admin_option = 'N')
    ;
    DECLARE CONTINUE HANDLER FOR NOT FOUND SET done = TRUE;

    OPEN role_cursor;
    revoke_roles: LOOP
        FETCH role_cursor INTO cur_user, cur_role;
        IF done THEN
            LEAVE revoke_roles;
        END IF;

        SET @sql := CONCAT_WS(' ', 'REVOKE', QUOTE(cur_role), 'FROM', QUOTE(cur_user));
        PREPARE stmt FROM @sql;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;
    END LOOP revoke_roles;

    CLOSE role_cursor;
END
