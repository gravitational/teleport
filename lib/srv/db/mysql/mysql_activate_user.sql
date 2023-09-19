CREATE PROCEDURE teleport_activate_user(IN username VARCHAR(32), IN details JSON)
proc_label:BEGIN
    DECLARE is_auto_user INT DEFAULT 0;
    DECLARE is_active INT DEFAULT 0;
    DECLARE role_index INT DEFAULT 0;
    DECLARE role VARCHAR(32) DEFAULT '';
    SET @roles = details->"$.roles";

    -- If the user already exists and was provisioned by Teleport, reactivate
    -- it, otherwise provision a new one.
    SELECT COUNT(TO_USER) INTO is_auto_user FROM mysql.role_edges WHERE FROM_USER = 'teleport-auto-user' AND TO_USER = username;
    IF is_auto_user = 1 THEN
        SELECT COUNT(USER) INTO is_active FROM information_schema.processlist WHERE USER = username;

        -- If the user has active connections, just use it to avoid messing up
        -- its existing roles.
        IF is_active = 1 THEN
            LEAVE proc_label;
        END IF;

        -- Otherwise reactivate the user, but first strip if of all roles to
        -- account for scenarios with left-over roles if database agent crashed
        -- and failed to cleanup upon session termination.
        CALL teleport_revoke_roles(username);

        -- Ensure the user is unlocked. User is locked at deactivation. 
        SET @sql := CONCAT_WS(' ', 'ALTER USER', QUOTE(username), 'ACCOUNT UNLOCK');
        PREPARE stmt FROM @sql;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;
    ELSE
        SET @sql := CONCAT_WS(' ', 'CREATE USER', QUOTE(username), details->>"$.auth_options", 'ATTRIBUTE', QUOTE(details->"$.attributes"));
        PREPARE stmt FROM @sql;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;

        SET @sql := CONCAT_WS(' ', 'GRANT', QUOTE('teleport-auto-user'), 'TO', QUOTE(username));
        PREPARE stmt FROM @sql;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;
    END IF;

    -- Assign roles.
    WHILE role_index < JSON_LENGTH(@roles) DO
        SELECT JSON_EXTRACT(@roles, CONCAT('$[',role_index,']')) INTO role;
        SELECT role_index + 1 INTO role_index;

        -- role extracted from JSON already has double quotes.
        SET @sql := CONCAT_WS(' ', 'GRANT', role, 'TO', QUOTE(username));
        PREPARE stmt FROM @sql;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;
    END WHILE;

    -- Ensure all assigned roles are available to use right after connection.
    SET @sql := CONCAT('SET DEFAULT ROLE ALL TO ', QUOTE(username));
    PREPARE stmt FROM @sql;
    EXECUTE stmt;
    DEALLOCATE PREPARE stmt;
END
