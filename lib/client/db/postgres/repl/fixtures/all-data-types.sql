-- Reset schema and set search path
DROP SCHEMA IF EXISTS all_pg_types CASCADE;
CREATE SCHEMA all_pg_types;
SET search_path = all_pg_types, public;

-- user-defined examples to cover ENUM, DOMAIN, COMPOSITE
CREATE TYPE my_enum AS ENUM ('foo','bar');
CREATE DOMAIN zip5 AS text CHECK (char_length(VALUE) = 5);
CREATE TYPE coord AS (x int, y int);

-- table with one column per type
CREATE TABLE all_types_demo (
  -- numeric
  t_smallint           smallint,
  t_integer            integer,
  t_bigint             bigint,
  t_decimal            decimal(10,2),
  t_numeric            numeric(8,3),
  t_real               real,
  t_double             double precision,
  t_money              money,

  -- boolean
  t_boolean            boolean,

  -- character
  t_char               char(3),
  t_varchar            varchar(10),
  t_text               text,

  -- binary
  t_bytea              bytea,

  -- date/time
  t_date               date,
  t_time               time,                      -- without time zone
  t_timetz             time with time zone,
  t_timestamp          timestamp,                 -- without time zone
  t_timestamptz        timestamp with time zone,
  t_interval           interval,

  -- UUID/XML/JSON
  t_uuid               uuid,
  t_xml                xml,
  t_json               json,
  t_jsonb              jsonb,

  -- bit strings
  t_bit                bit(8),
  t_varbit             bit varying(10),

  -- network
  t_cidr               cidr,
  t_inet               inet,
  t_macaddr            macaddr,
  t_macaddr8           macaddr8,

  -- geometric
  t_point              point,
  t_line               line,
  t_lseg               lseg,
  t_box                box,
  t_path_open          path,
  t_polygon            polygon,
  t_circle             circle,

  -- text search
  t_tsvector           tsvector,
  t_tsquery            tsquery,

  -- arrays
  t_int_array          integer[],
  t_text_array         text[],

  -- ranges
  t_int4range          int4range,
  t_int8range          int8range,
  t_numrange           numrange,
  t_tsrange            tsrange,
  t_tstzrange          tstzrange,
  t_daterange          daterange,

  -- system/OID-family
  t_oid                oid,
  t_regclass           regclass,
  t_regtype            regtype,
  t_pg_lsn             pg_lsn,
  t_txid_snapshot      txid_snapshot,

  -- user-defined
  t_enum               my_enum,
  t_domain             zip5,
  t_composite          coord
);

INSERT INTO all_types_demo VALUES (
  -- numeric
  32767,
  2147483647,
  9223372036854775807,
  12345.67,
  123.456,
  3.14,
  2.7182818284,
  12.34::money,

  -- boolean
  TRUE,

  -- character
  'abc',
  'abcdefghij',
  'hello',

  -- binary (hex bytea)
  '\xDEADBEEF',

  -- date/time
  DATE '2024-07-09',
  TIME '12:34:56',
  TIME '12:34:56+02',
  TIMESTAMP '2024-07-09 12:34:56',
  TIMESTAMPTZ '2024-07-09 12:34:56+00',
  INTERVAL '1 year 2 mons 3 days 4 hours 5 mins 6 secs',

  -- UUID/XML/JSON
  'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
  '<root attr="1">text</root>'::xml,
  '{"x":1,"y":2}',
  '{"a":true,"b":[1,2,3]}'::jsonb,

  -- bit strings
  B'10101010',
  B'10101'::varbit,

  -- Network
  CIDR '10.0.0.0/8',
  INET '192.168.1.5/24',
  MACADDR '08:00:2b:01:02:03',
  MACADDR8 '08:00:2b:01:02:03:04:05',

  -- geometric
  POINT '(1,2)',
  LINE  '{1,2,3}',
  LSEG  '[(0,0),(1,1)]',
  BOX   '((0,0),(1,1))',
  PATH  '[(0,0),(1,1),(1,0)]',    -- open path
  POLYGON '((0,0),(1,0),(1,1),(0,1))',
  CIRCLE '<(1,2),3>',

  -- text search
  to_tsvector('simple','a b c'),
  to_tsquery('simple','a & b'),

  -- arrays
  ARRAY[1,2,3],
  ARRAY['a','b','c'],

  -- ranges
  INT4RANGE(1, 10, '[]'),
  INT8RANGE(100, 1000, '[]'),
  NUMRANGE(1.5, 2.5, '[)'),
  TSRANGE(TIMESTAMP '2024-07-09 12:00:00', TIMESTAMP '2024-07-09 13:00:00', '[)'),
  TSTZRANGE(TIMESTAMPTZ '2024-07-09 12:00:00+00', TIMESTAMPTZ '2024-07-09 13:00:00+00', '[)'),
  DATERANGE(DATE '2024-07-01', DATE '2024-07-31', '[]'),

  -- system/OID-family
  12345::oid,
  'pg_type'::regclass,
  'int4'::regtype,
  '0/16B6C50'::pg_lsn,
  '100:200:150'::txid_snapshot,

  -- user-defined
  'foo',
  '12345',
  ROW(1,2)::coord
);

SELECT t_smallint, t_integer, t_bigint FROM all_types_demo;
SELECT t_decimal, t_numeric, t_real FROM all_types_demo;
SELECT t_double, t_money, t_boolean FROM all_types_demo;
SELECT t_char, t_varchar, t_text FROM all_types_demo;
SELECT t_bytea, t_date, t_time FROM all_types_demo;
SELECT t_timetz, t_timestamp, t_timestamptz FROM all_types_demo;
SELECT t_interval, t_uuid, t_xml FROM all_types_demo;
SELECT t_json, t_jsonb, t_bit FROM all_types_demo;
SELECT t_varbit, t_cidr, t_inet FROM all_types_demo;
SELECT t_macaddr, t_macaddr8, t_point FROM all_types_demo;
SELECT t_line, t_lseg, t_box FROM all_types_demo;
SELECT t_path_open, t_polygon, t_circle FROM all_types_demo;
SELECT t_tsvector, t_tsquery, t_int_array FROM all_types_demo;
SELECT t_text_array, t_int4range, t_int8range FROM all_types_demo;
SELECT t_numrange, t_tsrange, t_tstzrange FROM all_types_demo;
SELECT t_daterange, t_oid, t_regclass FROM all_types_demo;
SELECT t_regtype, t_pg_lsn, t_txid_snapshot FROM all_types_demo;
SELECT t_enum, t_domain, t_composite FROM all_types_demo;

SET search_path = public;
DROP SCHEMA all_pg_types CASCADE;
