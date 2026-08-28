-- simple literals
SELECT 'thing';
SELECT 'thing'; ;;

-- multiline query
SELECT
1
;

-- query with special characters
SELECT 'special_chars_!@#$%^&*()';

-- leading and trailing whitespace
   SELECT 1;

-- multiline with excessive whitespace
   SELECT
   1
   ;

-- with command in the middle treats the command as a literal
SELECT \? 1;

-- multiline with command in the middle treats the command as a literal
SELECT
\?
1;

-- multiline with command in the last line is not supported
SELECT
1
\?;

SELECT 1, -1, 1.24, -1.24, 'foo', NULL, 'null' ORDER BY 1;

-- aliases
SELECT
    1     AS "unsigned int",
    -1    AS "negative int",
    1.24  AS "float",
    -1.24 AS "negative float",
    'foo' AS "string",
    NULL  AS "actual NULL",
    'null' AS "string null"
    ORDER BY 1;

SELECT '1'::text AS "multiple rows of literals"
UNION
SELECT '-1'
UNION
SELECT '1.24'
UNION
SELECT '-1.24'
UNION
SELECT 'foo'
UNION
SELECT NULL
UNION
SELECT 'null' ORDER BY 1;

-- multiline strings
SELECT 'a
b
c' AS "multiline-string";

/* foo bar */
SELECT 1;
/* foo bar */ SELECT /* baz */ 42;

SELECT '$$1$$';
SELECT '$$;$$';

SHOW search_path;
EXPLAIN select 1;

-- TODO(gavin): fix the parser to handle trailing comments
SELECT 'thing'; ;;  -- a trailing space and comment

;

-- this fails to parse because delimiters are in the string literal
-- TODO(gavin): fix the parser to handle delimiter quotation properly
SELECT 'a
    ;
    b
    ;
c' AS "multiline with delimiters";

-- TODO(gavin): fix the parser to handle command/query mixed usage (delimited by semicolons)
-- with command following a statement
SELECT 1;\?
