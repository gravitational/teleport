CREATE PROCEDURE teleport_revoke_roles(IN username TEXT)
BEGIN
    DECLARE role TEXT DEFAULT '';
    DECLARE done INT DEFAULT 0;
    DECLARE role_cursor CURSOR FOR SELECT FROM_USER FROM mysql.role_edges WHERE FROM_USER != 'teleport-auto-user' AND TO_USER = username;
    DECLARE CONTINUE HANDLER FOR NOT FOUND SET done = 1;
    OPEN role_cursor;

    revoke_roles: LOOP
        FETCH role_cursor INTO role;
        IF done = 1 THEN
            LEAVE revoke_roles;
        END IF;

        SET @sql := CONCAT_WS(' ', 'REVOKE', QUOTE(role), 'FROM', QUOTE(username));
        PREPARE stmt FROM @sql;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;
    END LOOP revoke_roles;

    CLOSE role_cursor;
END
