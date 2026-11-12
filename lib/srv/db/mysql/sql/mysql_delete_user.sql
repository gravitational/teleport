CREATE PROCEDURE teleport_delete_user(IN username TEXT)
BEGIN
    -- Defaults to dropping user.
    DECLARE state TEXT DEFAULT 'TP003';
    DECLARE num_active INT DEFAULT 0;

    -- Views and procedures rely on the definer to work correctly. Dropping the
    -- definer causes them to stop working. Given this, the DROP USER command
    -- returns a error code 4006 (ER_CANNOT_USER_REFERENCED_AS_DEFINER).
    -- In this scenario, fallbacks to deactivating the user (using
    -- teleport_deactivate_user procedure).
    DECLARE CONTINUE HANDLER FOR 4006 SET state = 'TP004';

    SELECT COUNT(USER) INTO num_active FROM information_schema.processlist WHERE USER = username;
    IF num_active >= 1 THEN
        -- Throw a custom error code when user is still active from other sessions.
        SIGNAL SQLSTATE 'TP000' SET MESSAGE_TEXT = 'User has active connections';
    ELSE
        SET @sql := CONCAT('DROP USER IF EXISTS', QUOTE(username));
        PREPARE stmt FROM @sql;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;

        IF state = 'TP004' THEN
            CALL teleport_deactivate_user(username);
        END IF;
    END IF;

    -- Issue this as the last query so we can retrieve if the user was dropped
    -- or deactivated.
    SELECT state;
END
