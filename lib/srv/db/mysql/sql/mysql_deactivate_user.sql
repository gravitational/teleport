CREATE PROCEDURE teleport_deactivate_user(IN username TEXT)
BEGIN
    DECLARE is_active INT DEFAULT 0;
    SELECT COUNT(USER) INTO is_active FROM information_schema.processlist WHERE USER = username;
    IF is_active = 1 THEN
        -- Throw a custom error code when user is still active from other sessions.
        SIGNAL SQLSTATE 'TP000' SET MESSAGE_TEXT = 'User has active connections';
    ELSE
        -- Lock the user then revoke all the roles.
        SET @sql := CONCAT_WS(' ', 'ALTER USER', QUOTE(username), 'ACCOUNT LOCK');
        PREPARE stmt FROM @sql;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;

        CALL teleport_revoke_roles(username);
    END IF;
END
