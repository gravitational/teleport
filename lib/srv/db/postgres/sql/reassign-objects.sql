-- Make the below changes atomically. If we made changes step-by-step, a malicious
-- user could call the reassignment procedure before EXECUTE was revoked.
BEGIN;

-- Prevent shadowing of built-in functions that are used in the install script.
SET LOCAL search_path = pg_catalog, pg_temp;

-- Create a schema to hold the reassignment procedure. This provides
-- another layer of security over simply revoking all grants on the procedure
-- from all users and granting EXECUTE to teleport-admin. If the schema already
-- exists, fail.
CREATE SCHEMA teleport_objects;
-- Restrict the schema to teleport-admin USAGE only. A server-level
-- ALTER DEFAULT PRIVILEGES can grant on new schemas to arbitrary roles,
-- landing directly in the ACL rather than through PUBLIC, so revoke the
-- whole ACL before granting.
DO $$
DECLARE
    grantees text[];
    target_role text;
BEGIN
    REVOKE ALL ON SCHEMA teleport_objects FROM PUBLIC;

    SELECT array_agg(DISTINCT r.rolname)
    INTO grantees
    FROM pg_namespace n
    CROSS JOIN LATERAL aclexplode(n.nspacl) AS a
    JOIN pg_roles r ON r.oid = a.grantee
    WHERE n.nspname = 'teleport_objects';

    IF grantees IS NOT NULL THEN
        FOREACH target_role IN ARRAY grantees LOOP
            EXECUTE format('REVOKE ALL ON SCHEMA teleport_objects FROM %I', target_role);
        END LOOP;
    END IF;
END$$;
GRANT USAGE ON SCHEMA teleport_objects TO "teleport-admin";

-- THE SECURITY OF YOUR DATABASE DEPENDS ON THE SECURITY OF THIS PROCEDURE.
-- DO NOT ALLOW ANYONE YOU DO NOT TRUST TO EXECUTE IT. USERS/ROLES WITH THE
-- ABILITY TO EXECUTE IT CAN REASSIGN OWNERSHIP OF IMPORTANT DATABASE OBJECTS,
-- MAKING THEM INACCESSIBLE, USELESS, OR READABLE BY ANYONE.
--
-- IF YOU CHOOSE TO MODIFY THIS PROCEDURE, BE AWARE OF THE RISK YOU ARE TAKING.
-- BE ABSOLUTELY SURE THAT YOUR CHANGES ARE SECURE. MALICIOUS USERS CAN EASILY
-- ESCALATE PRIVILEGES BY CREATING OBJECTS THAT ARE THEN REASSIGNED TO A MORE
-- PRIVILEGED USER.
CREATE OR REPLACE PROCEDURE teleport_objects.teleport_reassign_objects(source_user varchar, destination_user varchar)
LANGUAGE plpgsql
SECURITY INVOKER
SET search_path = pg_catalog, pg_temp
SET lock_timeout = '5s'
AS $$
DECLARE
    user_oid oid;
    current_db_oid oid;
    obj record;
    remaining_objects text[];
    grantee_list text;
    unexpected_columns text;
BEGIN
    -- Only teleport-object-inheritor may receive reassigned objects. Any other
    -- destination could hand ownership to a more privileged role.
    IF destination_user != 'teleport-object-inheritor' THEN
        RAISE EXCEPTION 'destination_user must be teleport-object-inheritor, got %', destination_user;
    END IF;

    SELECT oid INTO user_oid FROM pg_roles WHERE rolname = source_user;
    IF user_oid IS NULL THEN
        RAISE EXCEPTION 'User % not found', source_user;
    END IF;

    -- Refuse to reassign objects owned by anything other than a Teleport
    -- auto-provisioned user.
    IF NOT EXISTS (
        SELECT 1 FROM pg_auth_members
        WHERE roleid = (SELECT oid FROM pg_roles WHERE rolname = 'teleport-auto-user')
        AND member = user_oid
    ) THEN
        RAISE EXCEPTION 'User % is not a Teleport-managed user (not a member of teleport-auto-user)', source_user;
    END IF;

    SELECT oid INTO current_db_oid FROM pg_database WHERE datname = current_database();

    -- This is a guard against features added to future versions of postgres
    -- that allow arbitrary code to be run via tables. We whitelist every column
    -- name for pg_* tables we currently account for so that added columns,
    -- which would be required for new features, cause this procedure to fail.
    -- Lists are the union of column names across postgres 14, 15, 16, 17 and
    -- 18.
    --
    -- Only unknown columns are flagged, so releases that do not have the listed
    -- columns do not cause the procedure to fail. This is important not only for
    -- older versions of postgres, but newer ones, where columns may have been
    -- removed.
    --
    -- When validating a new PostgreSQL major, expect this to fail closed until
    -- any added columns are reviewed and appended.
    SELECT string_agg(format('%s.%s', expected.catalog, a.attname), ', ' ORDER BY 1)
    INTO unexpected_columns
    FROM (VALUES
        ('pg_class'::regclass, ARRAY[
            'oid','relname','relnamespace','reltype','reloftype','relowner','relam',
            'relfilenode','reltablespace','relpages','reltuples','relallvisible',
            'relallfrozen','reltoastrelid','relhasindex','relisshared','relpersistence',
            'relkind','relnatts','relchecks','relhasrules','relhastriggers',
            'relhassubclass','relrowsecurity','relforcerowsecurity','relispopulated',
            'relreplident','relispartition','relrewrite','relfrozenxid','relminmxid',
            'relacl','reloptions','relpartbound']),
        ('pg_attribute'::regclass, ARRAY[
            'attrelid','attname','atttypid','attstattarget','attlen','attnum','attndims',
            'attcacheoff','atttypmod','attbyval','attalign','attstorage','attcompression',
            'attnotnull','atthasdef','atthasmissing','attidentity','attgenerated',
            'attisdropped','attislocal','attinhcount','attcollation','attacl','attoptions',
            'attfdwoptions','attmissingval']),
        ('pg_type'::regclass, ARRAY[
            'oid','typname','typnamespace','typowner','typlen','typbyval','typtype',
            'typcategory','typispreferred','typisdefined','typdelim','typrelid',
            'typsubscript','typelem','typarray','typinput','typoutput','typreceive',
            'typsend','typmodin','typmodout','typanalyze','typalign','typstorage',
            'typnotnull','typbasetype','typtypmod','typndims','typcollation',
            'typdefaultbin','typdefault','typacl']),
        ('pg_index'::regclass, ARRAY[
            'indexrelid','indrelid','indnatts','indnkeyatts','indisunique',
            'indnullsnotdistinct','indisprimary','indisexclusion','indimmediate',
            'indisclustered','indisvalid','indcheckxmin','indisready','indislive',
            'indisreplident','indkey','indcollation','indclass','indoption','indexprs',
            'indpred']),
        ('pg_constraint'::regclass, ARRAY[
            'oid','conname','connamespace','contype','condeferrable','condeferred',
            'conenforced','convalidated','conrelid','contypid','conindid','conparentid',
            'confrelid','confupdtype','confdeltype','confmatchtype','conislocal',
            'coninhcount','connoinherit','conperiod','conkey','confkey','conpfeqop',
            'conppeqop','conffeqop','confdelsetcols','conexclop','conbin']),
        ('pg_opclass'::regclass, ARRAY[
            'oid','opcmethod','opcname','opcnamespace','opcowner','opcfamily',
            'opcintype','opcdefault','opckeytype']),
        ('pg_trigger'::regclass, ARRAY[
            'oid','tgrelid','tgparentid','tgname','tgfoid','tgtype','tgenabled',
            'tgisinternal','tgconstrrelid','tgconstrindid','tgconstraint','tgdeferrable',
            'tginitdeferred','tgnargs','tgattr','tgargs','tgqual','tgoldtable','tgnewtable']),
        ('pg_rewrite'::regclass, ARRAY[
            'oid','rulename','ev_class','ev_type','ev_enabled','is_instead','ev_qual',
            'ev_action']),
        ('pg_depend'::regclass, ARRAY[
            'classid','objid','objsubid','refclassid','refobjid','refobjsubid','deptype'])
    ) AS expected(catalog, cols)
    JOIN pg_attribute a ON a.attrelid = expected.catalog
    WHERE a.attnum > 0
      AND NOT a.attisdropped
      AND a.attname::text <> ALL (expected.cols);

    IF unexpected_columns IS NOT NULL THEN
        RAISE EXCEPTION
            'teleport_reassign_objects: refusing to run; unaudited catalog column(s): %. A newer PostgreSQL may carry a feature this procedure does not screen; review each column and extend the allow-list.',
            unexpected_columns;
    END IF;

    -- Reassign safe tables. This is a whitelist as much as possible: a table
    -- is reassigned only if every feature it carries is allowed. Anything not
    -- listed, such as a feature we overlooked, or one a future Postgres adds,
    -- disqualifies the table, because such a feature can carry code that was
    -- defined by the untrusted auto-provisioned user.
    FOR obj IN
        SELECT c.oid AS reloid, n.nspname, c.relname
        FROM pg_shdepend sd
        JOIN pg_class c ON c.oid = sd.objid
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE sd.refobjid = user_oid
        AND sd.refclassid = 'pg_authid'::regclass
        AND sd.deptype = 'o'
        AND sd.dbid = current_db_oid
        AND sd.classid = 'pg_class'::regclass
        AND c.relkind = 'r'
    LOOP
        -- Lock the table before checking it so its definition cannot change
        -- between the whitelist check and the reassignment. Postgres has no
        -- mid-transaction unlock, so the lock is held until the transaction
        -- ends -- strictly safer than releasing right after the check/reassign.
        -- The lock is taken via a SECURITY DEFINER helper because this
        -- procedure runs as the (unprivileged) invoker, which cannot take an
        -- ACCESS EXCLUSIVE lock on a table it does not own. The lock persists
        -- to transaction end, so it still covers the whitelist re-check below.
        EXECUTE FORMAT('CALL teleport_objects.teleport_lock_table(%L, %L)',
            obj.nspname, obj.relname);

        -- Whitelist, re-evaluated under the lock so a concurrent change cannot
        -- introduce a disqualifying feature after the check but before reassign.
        IF EXISTS (
            SELECT 1 FROM pg_class c
            WHERE c.oid = obj.reloid
            -- pg_class: ordinary, standalone, heap, RLS-free table
            AND c.relkind = 'r'
            -- permanent or unlogged; not temp
            AND c.relpersistence IN ('p', 'u')
            -- not a partition child
            AND NOT c.relispartition
            -- RLS not enabled or forced
            AND NOT c.relrowsecurity
            AND NOT c.relforcerowsecurity
            -- not a typed table (OF a type)
            AND c.reloftype = 0
            -- no custom table access methods
            AND c.relam = (SELECT oid FROM pg_am WHERE amname = 'heap')
            -- No inheritance or partition relationship, as parent OR child. This
            -- single check covers both legacy INHERITS and declarative partitioning
            -- in either direction; relkind = 'r' separately excludes an empty
            -- partitioned parent (relkind 'p').
            AND NOT EXISTS (
                SELECT 1 FROM pg_inherits inh
                WHERE inh.inhrelid = c.oid OR inh.inhparent = c.oid
            )
            -- Every live column is a built-in, non-composite type. Rejects
            -- user-defined domains, enums, ranges, and composite types (any of which
            -- can run code with the owner's identity); allows built-in arrays such as
            -- int[]. A composite type is rejected even if it lives in pg_catalog.
            AND NOT EXISTS (
                SELECT 1
                FROM pg_attribute a
                JOIN pg_type t ON t.oid = a.atttypid
                WHERE a.attrelid = c.oid
                AND a.attnum > 0
                AND NOT a.attisdropped
                AND (t.typnamespace <> 'pg_catalog'::regnamespace OR t.typtype = 'c')
            )
            -- No column defaults and no generated columns. pg_attrdef stores both the
            -- default expression and the generation expression, so emptiness here
            -- rules out both. Identity columns are unaffected: they record no
            -- pg_attrdef row.
            AND NOT EXISTS (SELECT 1 FROM pg_attrdef ad WHERE ad.adrelid = c.oid)
            -- Constraints are limited to PRIMARY KEY, UNIQUE, FOREIGN KEY, and
            -- NOT NULL; a foreign key's referenced columns must themselves be
            -- built-in types (the referencing columns are this table's, already
            -- required built-in above). NOT NULL ('n') is catalogued in
            -- pg_constraint as of PostgreSQL 18. Rejects CHECK ('c'), EXCLUDE ('x'),
            -- and constraint triggers ('t').
            AND NOT EXISTS (
                SELECT 1 FROM pg_constraint con
                WHERE con.conrelid = c.oid
                AND (
                    con.contype NOT IN ('p', 'u', 'f', 'n')
                    OR (con.contype = 'f' AND EXISTS (
                        SELECT 1
                        FROM pg_attribute fa
                        JOIN pg_type ft ON ft.oid = fa.atttypid
                        WHERE fa.attrelid = con.confrelid
                        AND fa.attnum = ANY (con.confkey)
                        AND (ft.typnamespace <> 'pg_catalog'::regnamespace OR ft.typtype = 'c')
                    ))
                )
            )
            -- Indexes must be plain: no expression columns and no WHERE (i.e. partial-index
            -- predicate).
            AND NOT EXISTS (
                SELECT 1 FROM pg_index i
                WHERE i.indrelid = c.oid
                AND (i.indexprs IS NOT NULL OR i.indpred IS NOT NULL)
            )
            -- Indexes use only built-in operator classes.
            AND NOT EXISTS (
                SELECT 1
                FROM pg_index i
                JOIN pg_opclass oc
                    ON oc.oid = ANY (string_to_array(i.indclass::text, ' ')::oid[])
                WHERE i.indrelid = c.oid
                AND oc.opcnamespace <> 'pg_catalog'::regnamespace
            )
            -- No user-defined triggers. Internal triggers, such as those enforcing
            -- foreign-key constraints, run in the invoking user's context and are
            -- ignored.
            AND NOT EXISTS (
                SELECT 1 FROM pg_trigger tg
                WHERE tg.tgrelid = c.oid
                AND NOT tg.tgisinternal
            )
            -- No rewrite rules.
            AND NOT EXISTS (
                SELECT 1 FROM pg_rewrite rw
                WHERE rw.ev_class = c.oid
            )
            -- No row-level security policies, active or dormant.
            AND NOT EXISTS (
                SELECT 1 FROM pg_policy pol
                WHERE pol.polrelid = c.oid
            )
            -- No extended statistics (their expressions are evaluated under ANALYZE
            -- with the owner's identity).
            AND NOT EXISTS (
                SELECT 1 FROM pg_statistic_ext sx
                WHERE sx.stxrelid = c.oid
            )
            -- No security label, table- or column-level.
            AND NOT EXISTS (
                SELECT 1 FROM pg_seclabel sl
                WHERE sl.classoid = 'pg_class'::regclass
                AND sl.objoid = c.oid
            )
            -- Not part of an extension.
            AND NOT EXISTS (
                SELECT 1 FROM pg_depend dep
                WHERE dep.classid = 'pg_class'::regclass
                AND dep.objid = c.oid
                AND dep.deptype = 'e'
            )
            -- Backstop: every object that is PART OF this table (an auto or
            -- internal dependent) must be of a known, allowed catalog class.
            -- Any other class, whether it is simply not enumerated above, or
            -- a future postgres feature, trips this guard. Note that deptype
            -- is limited to 'a'/'i' so that objects which merely reference the
            -- table (an external view or inbound foreign key, deptype 'n') do
            -- not falsely disqualify it.
            AND NOT EXISTS (
                SELECT 1 FROM pg_depend dep
                WHERE dep.refclassid = 'pg_class'::regclass
                AND dep.refobjid = c.oid
                AND dep.deptype IN ('a', 'i')
                AND dep.classid NOT IN (
                    -- indexes, TOAST table, owned sequences
                    'pg_class'::regclass,
                    -- PRIMARY KEY / UNIQUE / FOREIGN KEY
                    'pg_constraint'::regclass,
                    -- the table's row type
                    'pg_type'::regclass,
                    -- internal FK/constraint triggers
                    'pg_trigger'::regclass
                )
            )
        ) THEN
            EXECUTE FORMAT('CALL teleport_objects.teleport_reassign_table(%L, %L, %L)', obj.nspname, obj.relname, destination_user);
        END IF;
    END LOOP;

    -- Reassign standalone sequences. Sequences attached to a table are
    -- already reassigned by ALTER TABLE above.
    FOR obj IN
        SELECT n.nspname, c.relname
        FROM pg_shdepend sd
        JOIN pg_class c ON c.oid = sd.objid
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE sd.refobjid = user_oid
        AND sd.refclassid = 'pg_authid'::regclass
        AND sd.deptype = 'o'
        AND sd.dbid = current_db_oid
        AND sd.classid = 'pg_class'::regclass
        AND c.relkind = 'S'
    LOOP
        EXECUTE FORMAT('CALL teleport_objects.teleport_reassign_sequence(%L, %L, %L)',
            obj.nspname, obj.relname, destination_user);
    END LOOP;

    -- Verify the user no longer owns anything.
    SELECT ARRAY_AGG(
        COALESCE(c.relname, n.nspname, p.proname, t.typname, sd.objid::text)
        ORDER BY 1
    ) INTO remaining_objects
    FROM pg_shdepend sd
    LEFT JOIN pg_class     c ON sd.classid = 'pg_class'::regclass     AND c.oid = sd.objid
    LEFT JOIN pg_namespace n ON sd.classid = 'pg_namespace'::regclass AND n.oid = sd.objid
    LEFT JOIN pg_proc      p ON sd.classid = 'pg_proc'::regclass      AND p.oid = sd.objid
    LEFT JOIN pg_type      t ON sd.classid = 'pg_type'::regclass      AND t.oid = sd.objid
    WHERE sd.refobjid = user_oid
    AND sd.refclassid = 'pg_authid'::regclass
    AND sd.deptype = 'o'
    AND sd.dbid = current_db_oid
    AND NOT (
        sd.classid = 'pg_type'::regclass AND (
            -- Exclude a relation's auto-generated row type (a table's is relkind
            -- 'r'): it is owned via, and reassigned with, its relation, so it is
            -- not a leftover. A standalone CREATE TYPE ... AS (...) is also
            -- typtype 'c' with a typrelid, but its pg_class row is relkind 'c' and
            -- nothing reassigns it, so it must still be flagged.
            (t.typtype = 'c' AND t.typrelid != 0 AND EXISTS (
                SELECT 1 FROM pg_class rc
                WHERE rc.oid = t.typrelid AND rc.relkind <> 'c'
            ))
            OR EXISTS (SELECT 1 FROM pg_type base WHERE base.typarray = t.oid)
        )
    );

    IF remaining_objects IS NOT NULL THEN
        RAISE EXCEPTION 'User % still owns object(s) after reassignment: %',
            source_user, array_to_string(remaining_objects, ', ');
    END IF;
END;$$;

-- Restrict EXECUTE on the reassignment procedure to teleport-admin only, with
-- no grant option. Note that REVOKE FROM PUBLIC is insufficient: a server-level
-- ALTER DEFAULT PRIVILEGES can grant EXECUTE to arbitrary roles at creation
-- time, landing directly in the ACL rather than through PUBLIC. We must revoke
-- ALL grants from all roles to be safe.
DO $$
DECLARE
    grantees text[];
    target_role text;
BEGIN
    -- PUBLIC's default EXECUTE is absent from proacl until the ACL is touched,
    -- so revoke it explicitly; this also materializes proacl for the scan below.
    REVOKE ALL ON PROCEDURE teleport_objects.teleport_reassign_objects(varchar, varchar) FROM PUBLIC;

    -- Snapshot every grantee before revoking, so the revokes cannot disturb
    -- the scan.
    SELECT array_agg(DISTINCT r.rolname)
    INTO grantees
    FROM pg_proc p
    CROSS JOIN LATERAL aclexplode(p.proacl) AS a
    JOIN pg_roles r ON r.oid = a.grantee
    WHERE p.oid = 'teleport_objects.teleport_reassign_objects(varchar, varchar)'::regprocedure;

    IF grantees IS NOT NULL THEN
        FOREACH target_role IN ARRAY grantees LOOP
            EXECUTE format(
                'REVOKE ALL ON PROCEDURE teleport_objects.teleport_reassign_objects(varchar, varchar) FROM %I',
                target_role);
        END LOOP;
    END IF;
END$$;

-- Ensure teleport-admin can execute the procedure.
GRANT EXECUTE ON PROCEDURE teleport_objects.teleport_reassign_objects(varchar, varchar) TO "teleport-admin";

-- Take an ACCESS EXCLUSIVE lock on a table. Runs SECURITY DEFINER so the
-- unprivileged invoker of teleport_reassign_objects can lock a table it does
-- not own. The lock is held until the surrounding transaction ends.
CREATE OR REPLACE PROCEDURE teleport_objects.teleport_lock_table(schema_name varchar, table_name varchar)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
SET lock_timeout = '5s'
AS $$
BEGIN
    EXECUTE FORMAT('LOCK TABLE %I.%I IN ACCESS EXCLUSIVE MODE',
        schema_name, table_name);
END$$;

CREATE OR REPLACE PROCEDURE teleport_objects.teleport_reassign_table(schema_name varchar, table_name varchar, destination_user varchar)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
SET lock_timeout = '5s'
AS $$
DECLARE
    grantee_list text;
BEGIN
    -- Revoke every privilege from all grantees but the owner, so no
    -- grant (e.g. TRIGGER, which would let the grantee add a malicious
    -- trigger to the reassigned table) remains.
    SELECT string_agg(DISTINCT quote_ident(r.rolname), ', ')
    INTO grantee_list
    FROM pg_class c
    CROSS JOIN LATERAL aclexplode(c.relacl) AS a
    JOIN pg_roles r ON r.oid = a.grantee
    WHERE c.oid = FORMAT('%I.%I', schema_name, table_name)::regclass
      AND a.grantee <> c.relowner;

    EXECUTE FORMAT('REVOKE ALL ON TABLE %I.%I FROM PUBLIC%s CASCADE',
        schema_name, table_name,
        CASE WHEN grantee_list IS NULL THEN '' ELSE ', ' || grantee_list END);

    -- ALTER TABLE OWNER TO also reassigns the table's indexes, TOAST table,
    -- composite row type, and identity sequences.
    EXECUTE FORMAT('ALTER TABLE %I.%I OWNER TO %I',
        schema_name, table_name, destination_user);
END$$;

CREATE OR REPLACE PROCEDURE teleport_objects.teleport_reassign_sequence(schema_name varchar, table_name varchar, destination_user varchar)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
SET lock_timeout = '5s'
AS $$
BEGIN
    EXECUTE FORMAT('ALTER SEQUENCE %I.%I OWNER TO %I',
        schema_name, table_name, destination_user);
END$$;

COMMIT;
