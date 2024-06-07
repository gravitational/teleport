CREATE PROCEDURE teleport_delete_user(IN username VARCHAR(80))
BEGIN
    -- Defaults to dropping user.
    DECLARE state VARCHAR(5);
    DECLARE is_active INT DEFAULT 0;
    DECLARE view_count INT DEFAULT 0;
    DECLARE procedure_count INT DEFAULT 0;

    SELECT COUNT(USER) INTO is_active FROM information_schema.processlist WHERE USER = username;
    IF is_active = 1 THEN
        -- Throw a custom error code when user is still active from other sessions.
        SIGNAL SQLSTATE 'TP000' SET MESSAGE_TEXT = 'User has active connections';
    ELSE
        -- MariaDB DROP USER doesn't fail if the user owns views/procedures.
        -- However,dropping the user causes the view/procedure to no longer be
        -- usable, even if a user with same name recreated.
        SELECT COUNT(*) INTO procedure_count FROM information_schema.routines WHERE routine_type = 'PROCEDURE' AND DEFINER = CONCAT(username, '@%');
        SELECT COUNT(*) INTO view_count FROM information_schema.views WHERE DEFINER = CONCAT(username, '@%');

        IF procedure_count > 0 OR view_count > 0 THEN
            SET state = 'TP004';
            CALL teleport_deactivate_user(username);
        ELSE
            SET state = 'TP003';
            SET @sql := CONCAT('DROP ROLE IF EXISTS', QUOTE(CONCAT("tp-role-", username)));
            PREPARE stmt FROM @sql;
            EXECUTE stmt;
            DEALLOCATE PREPARE stmt;

            SET @sql := CONCAT('DROP USER IF EXISTS', QUOTE(username));
            PREPARE stmt FROM @sql;
            EXECUTE stmt;
            DEALLOCATE PREPARE stmt;
        END IF;
    END IF;

    -- Issue this as the last query so we can retrieve if the user was dropped
    -- or deactivated.
    SELECT state;
END
