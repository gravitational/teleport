CREATE PROCEDURE teleport_revoke_roles(IN username VARCHAR(80))
BEGIN
    DECLARE cur_role CHAR(128);
    DECLARE done INT DEFAULT FALSE;
    DECLARE role_cursor CURSOR FOR SELECT Role FROM mysql.roles_mapping WHERE User = CONCAT("tp-role-", username) AND Admin_option = 'N';
    DECLARE CONTINUE HANDLER FOR NOT FOUND SET done = TRUE;
    SET @all_in_one_role := CONCAT("tp-role-", username);

    OPEN role_cursor;
    revoke_roles: LOOP
        FETCH role_cursor INTO cur_role;
        IF done THEN
            LEAVE revoke_roles;
        END IF;

        SET @sql := CONCAT_WS(' ', 'REVOKE', QUOTE(cur_role), 'FROM', QUOTE(@all_in_one_role));
        PREPARE stmt FROM @sql;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;
    END LOOP revoke_roles;

    CLOSE role_cursor;
END
