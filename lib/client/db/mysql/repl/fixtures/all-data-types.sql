-- temporarily disable note warnings for test consistency
set session sql_notes = 0;
CREATE DATABASE IF NOT EXISTS all_data_types;
set session sql_notes = 1;
USE all_data_types;

-- without ANSI_QUOTES double quotes are literals
SET SESSION sql_mode='';
SELECT "thing";
-- with ANSI_QUOTES double quotes are identifiers, which is what we want when
-- selecting from the table below
SET SESSION sql_mode='ANSI_QUOTES';
SELECT "thing";

CREATE TABLE all_types_demo (
    "c tinyint"         TINYINT,
    c_smallint        SMALLINT,
    c_mediumint       MEDIUMINT,
    c_int             INT,
    c_bigint          BIGINT,
    c_decimal         DECIMAL(10,2),
    c_float           FLOAT,
    c_double          DOUBLE,
    c_bit             BIT(8),
    c_bool            BOOL,
    c_date            DATE,
    c_time            TIME,
    c_datetime        DATETIME,
    c_timestamp       TIMESTAMP NULL DEFAULT NULL,
    c_year            YEAR,
    c_char            CHAR(5),
    c_varchar         VARCHAR(10),
    c_binary          BINARY(3),
    c_varbinary       VARBINARY(4),
    c_tinyblob        TINYBLOB,
    c_blob            BLOB,
    c_mediumblob      MEDIUMBLOB,
    c_longblob        LONGBLOB,
    c_tinytext        TINYTEXT,
    c_text            TEXT,
    c_mediumtext      MEDIUMTEXT,
    c_longtext        LONGTEXT,
    c_enum            ENUM('foo','bar','baz'),
    c_set             SET('a','b','c'),
    c_geometry        GEOMETRY,
    c_point           POINT,
    c_linestring      LINESTRING,
    c_polygon         POLYGON,
    c_multipoint      MULTIPOINT,
    c_multilinestring MULTILINESTRING,
    c_multipolygon    MULTIPOLYGON,
    c_geometrycoll    GEOMETRYCOLLECTION,
    c_json            JSON
);

INSERT INTO all_types_demo VALUES (
    127,
    32767,
    8388607,
    2147483647,
    9223372036854775807,
    12345.67,
    3.14,
    2.7182818284,
    b'10101010',
    1,
    '2024-07-09',
    '12:34:56',
    '2024-07-09 12:34:56',
    '2024-07-09 12:34:56',
    2025,
    'abcde',
    'abcdefghij',
    'abc',
    'defg',
    'a',
    'blob',
    'medblob',
    'longblob',
    'tiny',
    'text',
    'mediumtext',
    'longtext',
    'foo',
    'a,b',
    ST_GeomFromText('POINT(1 1)'),
    ST_GeomFromText('POINT(2 2)'),
    ST_GeomFromText('LINESTRING(0 0,1 1,2 2)'),
    ST_GeomFromText('POLYGON((0 0,1 0,1 1,0 1,0 0))'),
    ST_GeomFromText('MULTIPOINT((0 0),(1 1))'),
    ST_GeomFromText('MULTILINESTRING((0 0,1 1),(2 2,3 3))'),
    ST_GeomFromText('MULTIPOLYGON(((0 0,1 0,1 1,0 1,0 0)))'),
    ST_GeomFromText('GEOMETRYCOLLECTION(POINT(1 2),LINESTRING(0 0,1 1))'),
    JSON_OBJECT('x', 1, 'y', 2)
);

SELECT "c tinyint", `c_smallint`, "c_mediumint" FROM `all_types_demo`;
SELECT c_int, c_bigint, c_decimal FROM all_types_demo;
SELECT c_float, c_double, c_bit FROM all_types_demo;
SELECT c_bool, c_date, c_time FROM all_types_demo;
SELECT c_datetime, c_timestamp, c_year FROM all_types_demo;
SELECT c_char, c_varchar, c_binary FROM all_types_demo;
SELECT c_varbinary, c_tinyblob, c_blob FROM all_types_demo;
SELECT "c_mediumblob",
       `c_longblob`,
       c_tinytext
FROM all_types_demo;
SELECT c_text, c_mediumtext, c_longtext FROM all_types_demo;
SELECT c_enum, c_set, ST_AsText(c_geometry) AS c_geometry FROM all_types_demo;
SELECT ST_AsText(c_point) AS c_point, ST_AsText(c_linestring) AS c_linestring, ST_AsText(c_polygon) AS c_polygon FROM all_types_demo;
SELECT ST_AsText(c_multipoint) AS c_multipoint, ST_AsText(c_multilinestring) AS c_multilinestring, ST_AsText(c_multipolygon) AS c_multipolygon FROM all_types_demo;
SELECT ST_AsText(c_geometrycoll) AS c_geometrycoll, c_json FROM all_types_demo;
DROP DATABASE all_data_types;
