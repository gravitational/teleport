-- with ANSI_QUOTES double quotes are identifiers
SET SESSION sql_mode='ANSI_QUOTES';
SELECT "thing";
-- without ANSI_QUOTES double quotes are literals
SET SESSION sql_mode='';
SELECT "thing";
select 1,  -1,1.24, -1.24, 'foo', null, 'null';
select
    1 as 'unsigned int',
    -1 as 'negative int',
    1.24 as "float",
    -1.24 as 'negative float',
    'foo' as 'string',
    null as 'actual NULL',
    "null" as 'string null';

select 1 as 'multiple rows of literals'
union
    select -1
union
    select 1.24
union
    select -1.24
union
    select 'foo'
union
    select null
union
    select 'null';

select 'a
b
c' as 'multiline-string';

select 'a
    ;
    b
    ;
c' as 'multiline with delimiters';

/* foo bar */
select 1;
/* foo bar */select /*
baz */ 42;

delimiter $$
select '$$1$$'$$
select '$$;$$';$$
delimiter ;
select 1;
