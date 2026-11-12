CREATE PROCEDURE teleport_activate_user(IN username TEXT, IN details JSON)
proc_label:BEGIN
    DECLARE is_auto_user INT DEFAULT 0;
    DECLARE num_active INT DEFAULT 0;
    DECLARE is_same_user INT DEFAULT 0; 
    DECLARE role_index INT DEFAULT 0;
    DECLARE role TEXT DEFAULT '';
    DECLARE cur_roles TEXT DEFAULT '';
    SET @roles = details->"$.roles";
    SET @teleport_user = details->>"$.attributes.user";

    -- If the user already exists and was provisioned by Teleport, reactivate
    -- it, otherwise provision a new one.
    SELECT COUNT(TO_USER) INTO is_auto_user FROM mysql.role_edges WHERE FROM_USER = 'teleport-auto-user' AND TO_USER = username;
    IF is_auto_user = 1 THEN
        SELECT COUNT(USER) INTO is_same_user FROM INFORMATION_SCHEMA.USER_ATTRIBUTES WHERE USER = username AND ATTRIBUTE->"$.user" = @teleport_user;
        IF is_same_user = 0 THEN
            SIGNAL SQLSTATE 'TP001' SET MESSAGE_TEXT = 'Teleport username does not match user attributes';
        END IF;

        SELECT COUNT(USER) INTO num_active FROM information_schema.processlist WHERE USER = username;

        -- If the user has active connections, make sure the provided roles
        -- match what the user currently has.
        IF num_active >= 1 THEN
            SELECT COALESCE(JSON_ARRAYAGG(FROM_USER), JSON_ARRAY()) INTO cur_roles FROM mysql.role_edges WHERE FROM_USER != 'teleport-auto-user' AND TO_USER = username;
            -- Compare roles as sets (order-independent) rather than exact
            -- string match, since role ordering is not deterministic.
            IF NOT (JSON_LENGTH(@roles) <=> JSON_LENGTH(cur_roles)) THEN
                SIGNAL SQLSTATE 'TP002' SET MESSAGE_TEXT = 'user has active connections and roles have changed';
            END IF;
            SET role_index = 0;
            WHILE role_index < JSON_LENGTH(@roles) DO
                -- Preserve JSON encoding for role names containing quotes or backslashes.
                SELECT JSON_EXTRACT(@roles, CONCAT('$[',role_index,']')) INTO role;
                IF NOT JSON_CONTAINS(cur_roles, role) THEN
                    SIGNAL SQLSTATE 'TP002' SET MESSAGE_TEXT = 'user has active connections and roles have changed';
                END IF;
                SET role_index = role_index + 1;
            END WHILE;
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
        SELECT JSON_UNQUOTE(JSON_EXTRACT(@roles, CONCAT('$[',role_index,']'))) INTO role;
        SELECT role_index + 1 INTO role_index;

        SET @sql := CONCAT_WS(' ', 'GRANT', QUOTE(role), 'TO', QUOTE(username));
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
