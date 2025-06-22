CREATE PROCEDURE teleport_deactivate_user(IN username VARCHAR(32))
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

        -- Call a callback procedure once a user is deactivated (if the procedure exists)
        -- The signature of the procedure should be:
        -- CREATE PROCEDURE teleport_user_deactivated_callback(IN username VARCHAR(32))
        IF EXISTS (
            SELECT 1
            FROM information_schema.routines
            WHERE routine_type = 'procedure'
              AND routine_schema = 'teleport'
              AND routine_name = 'teleport_user_deactivated_callback'
        ) THEN
            CALL teleport_user_deactivated_callback(username);
        END IF;
    END IF;
END
