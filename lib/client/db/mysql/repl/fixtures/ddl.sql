-- temporarily disable note warnings for test consistency
set session sql_notes = 0;
DROP DATABASE IF EXISTS test_ddl_db;
CREATE DATABASE test_ddl_db;
set session sql_notes = 1;
-- this should produce a note level warning because it does exist
CREATE DATABASE if not exists test_ddl_db;
-- we clear the warning by selecting the warning count
select @@warning_count;
USE test_ddl_db;

CREATE TABLE test_table (
  id INT PRIMARY KEY AUTO_INCREMENT,
  value VARCHAR(50) NOT NULL,
  note VARCHAR(100) DEFAULT NULL
) ENGINE=InnoDB;

CREATE VIEW test_view AS (
    SELECT id, value
    FROM test_table
);

DELIMITER $$
CREATE FUNCTION test_func(x INT) RETURNS INT DETERMINISTIC
BEGIN
  RETURN x * 2;
END $$
DELIMITER ;

DELIMITER $$
CREATE PROCEDURE test_proc()
BEGIN
  INSERT INTO test_table(value) VALUES ('from proc');
END $$
DELIMITER ;

CALL test_proc();

DELIMITER $$
CREATE TRIGGER test_trigger_before_insert
BEFORE INSERT ON test_table
FOR EACH ROW
BEGIN
  SET NEW.value = UPPER(NEW.value);
END $$
DELIMITER ;

CALL test_proc();

CREATE EVENT test_event
  ON SCHEDULE AT CURRENT_TIMESTAMP + INTERVAL 1 HOUR
  DO INSERT INTO test_table(value) VALUES ('from event');

set session sql_notes = 0;
-- roles are not database scoped, let's disable note warnings in case some other
-- test has already created the role or user.
CREATE ROLE IF NOT EXISTS test_role;
DROP ROLE test_role;

CREATE USER IF NOT EXISTS test_user@'localhost'
  IDENTIFIED BY 'some password'
  ACCOUNT LOCK;
ALTER USER test_user@'localhost' PASSWORD EXPIRE;
DROP USER test_user@'localhost';
set session sql_notes = 1;

ALTER DATABASE test_ddl_db CHARACTER SET utf8mb4;
ALTER TABLE test_table ADD COLUMN note2 VARCHAR(100) DEFAULT NULL;
ALTER EVENT test_event DISABLE;
ALTER FUNCTION test_func COMMENT 'Test function';
ALTER PROCEDURE test_proc COMMENT 'Test procedure';

RENAME TABLE test_table
       TO test_table_renamed;
SELECT * from test_table_renamed;

RENAME
  TABLE test_table_renamed
  TO test_table;

SELECT * from test_table;

INSERT INTO test_table (value)
WITH RECURSIVE numbers(n) AS (
  SELECT 4
  UNION ALL
  SELECT n + 1 FROM numbers WHERE n < 120
)
SELECT 'especially wide column' AS value
UNION ALL
SELECT CONCAT('value_', n) FROM numbers;

SELECT * from test_table;

-- there is no "test_server", this should make an error.
ALTER SERVER test_server OPTIONS (HOST '127.0.0.1');

DROP TRIGGER test_trigger_before_insert;

TRUNCATE TABLE test_table    ;
DROP TABLE test_table;

DROP DATABASE test_ddl_db
;
