CREATE PROCEDURE teleport_activate_user(IN username VARCHAR(80), IN details JSON)
proc_label:BEGIN
    DECLARE is_auto_user INT DEFAULT 0;
    DECLARE is_active INT DEFAULT 0;
    DECLARE is_same_user INT DEFAULT 0;
    DECLARE role_index INT DEFAULT 0;
    DECLARE cur_role VARCHAR(128) DEFAULT '';
    DECLARE cur_roles TEXT DEFAULT '';
    SET @roles = JSON_EXTRACT(details, "$.roles");
    SET @teleport_user = JSON_VALUE(details, "$.attributes.user");
    SET @all_in_one_role := CONCAT("tp-role-", username);

    -- If the user already exists and was provisioned by Teleport, reactivate
    -- it, otherwise provision a new one.
    SELECT COUNT(USER) INTO is_auto_user FROM mysql.roles_mapping WHERE Role = 'teleport-auto-user' AND USER = username AND Admin_option = 'N';
    IF is_auto_user = 1 THEN
        SELECT COUNT(USER) INTO is_same_user FROM user_attributes WHERE USER = username AND JSON_VALUE(Attributes, "$.user") = @teleport_user;
        IF is_same_user = 0 THEN
            SIGNAL SQLSTATE 'TP001' SET MESSAGE_TEXT = 'Teleport username does not match user attributes';
        END IF;

        SELECT COUNT(USER) INTO is_active FROM information_schema.processlist WHERE USER = username;
        -- If the user has active connections, make sure the provided roles
        -- match what the user currently has. JSON_EQUALS was added in MariaDB
        -- 10.7.0. Use loop and JSON_CONTAINS instead of JSON_EQUALS to support
        -- older versions.
        IF is_active = 1 THEN
            SELECT JSON_ARRAYAGG(Role) INTO cur_roles FROM mysql.roles_mapping WHERE USER = @all_in_one_role AND Admin_option = 'N';
            IF JSON_LENGTH(@roles) != JSON_LENGTH(cur_roles) THEN
                SIGNAL SQLSTATE 'TP002' SET MESSAGE_TEXT = 'user has active connections and roles have changed';
            END IF;
            SET role_index = 0;
            WHILE role_index < JSON_LENGTH(@roles) DO
                SELECT JSON_EXTRACT(@roles, CONCAT('$[',role_index,']')) INTO cur_role;
                SELECT role_index + 1 INTO role_index;
                IF !JSON_CONTAINS(cur_roles, cur_role) THEN
                    SIGNAL SQLSTATE 'TP002' SET MESSAGE_TEXT = 'user has active connections and roles have changed';
                END IF;
            END WHILE;
            LEAVE proc_label;
        END IF;

        -- Ensure the user is unlocked. User is locked at deactivation.
        SET @sql := CONCAT_WS(' ', 'ALTER USER', QUOTE(username), 'ACCOUNT UNLOCK');
        PREPARE stmt FROM @sql;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;
    ELSE
        SET @sql := CONCAT_WS(' ', 'CREATE USER', QUOTE(username), JSON_VALUE(details, "$.auth_options"));
        PREPARE stmt FROM @sql;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;

        SET @sql := CONCAT_WS(' ', 'GRANT', QUOTE('teleport-auto-user'), 'TO', QUOTE(username));
        PREPARE stmt FROM @sql;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;

        -- Set up the all-in-one role.
        SET @sql := CONCAT_WS(' ', 'CREATE OR REPLACE ROLE', QUOTE(@all_in_one_role), 'WITH ADMIN CURRENT_USER');
        PREPARE stmt FROM @sql;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;

        -- Set up user attributes.
        CREATE TABLE IF NOT EXISTS user_attributes(User char(128), Attributes JSON, Primary Key (User));
        SET @attributes := JSON_EXTRACT(details,"$.attributes");
        INSERT INTO user_attributes VALUES(username, @attributes) ON DUPLICATE KEY UPDATE Attributes=@attributes;
    END IF;

    -- Strip current roles and assign new roles to all-in-one role.
    CALL teleport_revoke_roles(username);
    SET role_index = 0;
    WHILE role_index < JSON_LENGTH(@roles) DO
        SELECT JSON_EXTRACT(@roles, CONCAT('$[',role_index,']')) INTO cur_role;
        SELECT role_index + 1 INTO role_index;

        -- role extracted from JSON already has double quotes.
        SET @sql := CONCAT_WS(' ', 'GRANT', cur_role, 'TO', QUOTE(@all_in_one_role));
        PREPARE stmt FROM @sql;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;
    END WHILE;

    -- Assign all-in-one role to user as default role.
    SET @sql := CONCAT_WS(' ', 'GRANT', QUOTE(@all_in_one_role), 'TO', QUOTE(username));
    PREPARE stmt FROM @sql;
    EXECUTE stmt;
    DEALLOCATE PREPARE stmt;
    SET @sql := CONCAT_WS(' ', 'SET DEFAULT ROLE', QUOTE(@all_in_one_role), 'FOR', QUOTE(username));
    PREPARE stmt FROM @sql;
    EXECUTE stmt;
    DEALLOCATE PREPARE stmt;
END
