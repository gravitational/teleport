/*
 * Media contexts backend for DB objects
 *
 * Author: KaiGai Kohei <kaigai@ak.jp.nec.com>
 */

#include <sys/stat.h>
#include <string.h>
#include <stdio.h>
#include <stdio_ext.h>
#include <ctype.h>
#include <errno.h>
#include <limits.h>
#include <fnmatch.h>
#include "callbacks.h"
#include "label_internal.h"

/*
 * Regular database object's security context interface
 *
 * It provides applications a regular security context for the given
 * database objects. The pair of object's name and a security context
 * are described in the specfile. In the default, it shall be stored
 * in the /etc/selinux/$POLICYTYPE/contexts/sepgsql_contexts .
 * (It assumes SE-PostgreSQL in the default. For other RDBMS, use the
 * SELABEL_OPT_PATH option to specify different specfile.)
 *
 * Each line has the following format:
 *   <object class> <object name/identifier> <security context>
 *
 * For example:
 * ----------------------------------------
 * #
 * # It is an example specfile for database objects
 * #
 * db_database  template1           system_u:object_r:sepgsql_db_t:s0
 *
 * db_schema    *.pg_catalog        system_u:object_r:sepgsql_sys_schema_t:s0
 *
 * db_table     *.pg_catalog.*	    system_u:object_r:sepgsql_sysobj_t:s0
 * db_column    *.pg_catalog.*.*    system_u:object_r:sepgsql_sysobj_t:s0
 * ----------------------------------------
 *
 * All the characters after the '#' are dealt as comments.
 *
 * The first token is object class. SELABEL_DB_* declared in label.h are
 * corresponding to a certain database object.
 *
 * The object name/identifier is compared to the given key.
 * A database object can have its own namespace hierarchy.
 * In the case of SE-PgSQL, database is the top level object, and schema
 * is deployed just under a database. A schema can contains various kind
 * of objects, such as tables, procedures and so on.
 * Thus, when we lookup an expected security context for a table of
 * "pg_class", it is necessary to assume selabel_lookup() is called with
 * "postgres.pg_catalog.pg_class", not just a "pg_class".
 *
 * Wildcards ('*' or '?') are available on the patterns, so if you want
 * to match a table within any schema, you should set '*' on the upper
 * namespaces of the table.
 *
 * The structure of namespace depends on RDBMS.
 * For example, Trusted-RUBIX has an idea of "catalog" which performs
 * as a namespace between a database and individual schemas. In this
 * case, a table has upper three layers.
 */

/*
 * spec_t : It holds a pair of a key and an expected security context
 */
typedef struct spec {
	struct selabel_lookup_rec lr;
	char	       *key;
	int		type;
	int		matches;
} spec_t;

/*
 * catalog_t : An array of spec_t
 */
typedef struct catalog {
	unsigned int	nspec;	/* number of specs in use */
	unsigned int	limit;	/* physical limitation of specs[] */
	spec_t		specs[0];
} catalog_t;

/*
 * Helper function to parse a line read from the specfile
 */
static int
process_line(const char *path, char *line_buf, unsigned int line_num,
	     catalog_t *catalog)
{
	spec_t	       *spec = &catalog->specs[catalog->nspec];
	char	       *type, *key, *context, *temp;
	int		items;

	/* Cut off comments */
	temp = strchr(line_buf, '#');
	if (temp)
		*temp = '\0';

	/*
	 * Every entry must have the following format
	 *   <object class> <object name> <security context>
	 */
	type = key = context = temp = NULL;
	items = sscanf(line_buf, "%ms %ms %ms %ms",
		       &type, &key, &context, &temp);
	if (items != 3) {
		if (items > 0)
			selinux_log(SELINUX_WARNING,
				    "%s:  line %u has invalid format, skipped",
				    path, line_num);
		goto skip;
	}

	/*
	 * Set up individual spec entry
	 */
	memset(spec, 0, sizeof(spec_t));

	if (!strcmp(type, "db_database"))
		spec->type = SELABEL_DB_DATABASE;
	else if (!strcmp(type, "db_schema"))
		spec->type = SELABEL_DB_SCHEMA;
	else if (!strcmp(type, "db_table"))
		spec->type = SELABEL_DB_TABLE;
	else if (!strcmp(type, "db_column"))
		spec->type = SELABEL_DB_COLUMN;
	else if (!strcmp(type, "db_sequence"))
		spec->type = SELABEL_DB_SEQUENCE;
	else if (!strcmp(type, "db_view"))
		spec->type = SELABEL_DB_VIEW;
	else if (!strcmp(type, "db_procedure"))
		spec->type = SELABEL_DB_PROCEDURE;
	else if (!strcmp(type, "db_blob"))
		spec->type = SELABEL_DB_BLOB;
	else if (!strcmp(type, "db_tuple"))
		spec->type = SELABEL_DB_TUPLE;
	else if (!strcmp(type, "db_language"))
		spec->type = SELABEL_DB_LANGUAGE;
	else if (!strcmp(type, "db_exception"))
		spec->type = SELABEL_DB_EXCEPTION;
	else if (!strcmp(type, "db_datatype"))
		spec->type = SELABEL_DB_DATATYPE;
	else {
		selinux_log(SELINUX_WARNING,
			    "%s:  line %u has invalid object type %s\n",
			    path, line_num, type);
		goto skip;
	}

	free(type);
	spec->key = key;
	spec->lr.ctx_raw = context;

	catalog->nspec++;

	return 0;

skip:
	free(type);
	free(key);
	free(context);
	free(temp);

	return 0;
}

/*
 * selabel_close() handler
 */
static void
db_close(struct selabel_handle *rec)
{
	catalog_t      *catalog = (catalog_t *)rec->data;
	spec_t	       *spec;
	unsigned int	i;

	if (!catalog)
		return;

	for (i = 0; i < catalog->nspec; i++) {
		spec = &catalog->specs[i];
		free(spec->key);
		free(spec->lr.ctx_raw);
		free(spec->lr.ctx_trans);
		__pthread_mutex_destroy(&spec->lr.lock);
	}
	free(catalog);
}

/*
 * selabel_lookup() handler
 */
static struct selabel_lookup_rec *
db_lookup(struct selabel_handle *rec, const char *key, int type)
{
	catalog_t      *catalog = (catalog_t *)rec->data;
	spec_t	       *spec;
	unsigned int	i;

	for (i = 0; i < catalog->nspec; i++) {
		spec = &catalog->specs[i];

		if (spec->type != type)
			continue;
		if (!fnmatch(spec->key, key, 0)) {
			spec->matches++;

			return &spec->lr;
		}
	}

	/* No found */
	errno = ENOENT;
	return NULL;
}

/*
 * selabel_stats() handler
 */
static void
db_stats(struct selabel_handle *rec)
{
	catalog_t      *catalog = (catalog_t *)rec->data;
	unsigned int	i, total = 0;

	for (i = 0; i < catalog->nspec; i++)
		total += catalog->specs[i].matches;

	selinux_log(SELINUX_INFO, "%u entries, %u matches made\n",
		    catalog->nspec, total);
}

/*
 * selabel_open() handler
 */
static catalog_t *
db_init(const struct selinux_opt *opts, unsigned nopts,
			    struct selabel_handle *rec)
{
	catalog_t      *catalog;
	FILE	       *filp;
	const char     *path = NULL;
	char	       *line_buf = NULL;
	size_t		line_len = 0;
	unsigned int	line_num = 0;
	unsigned int	i;
	struct stat sb;

	/*
	 * Initialize catalog data structure
	 */
	catalog = malloc(sizeof(catalog_t) + 32 * sizeof(spec_t));
	if (!catalog)
		return NULL;
	catalog->limit = 32;
	catalog->nspec = 0;

	/*
	 * Process arguments
	 *
	 * SELABEL_OPT_PATH:
	 *   It allows to specify an alternative specification file instead of
	 *   the default one. If RDBMS is not SE-PostgreSQL, it may need to
	 *   specify an explicit specfile for database objects.
	 */
	while (nopts) {
		nopts--;
		switch (opts[nopts].type) {
		case SELABEL_OPT_PATH:
			path = opts[nopts].value;
			break;
		case SELABEL_OPT_UNUSED:
		case SELABEL_OPT_VALIDATE:
		case SELABEL_OPT_DIGEST:
			break;
		default:
			free(catalog);
			errno = EINVAL;
			return NULL;
		}
	}

	/*
	 * Open the specification file
	 */
	if (!path)
		path = selinux_sepgsql_context_path();

	if ((filp = fopen(path, "re")) == NULL) {
		free(catalog);
		return NULL;
	}
	if (fstat(fileno(filp), &sb) < 0) {
		free(catalog);
		fclose(filp);
		return NULL;
	}
	if (!S_ISREG(sb.st_mode)) {
		free(catalog);
		fclose(filp);
		errno = EINVAL;
		return NULL;
	}
	rec->spec_file = strdup(path);
	if (!rec->spec_file) {
                free(catalog);
                fclose(filp);
                return NULL;
	}

	/*
	 * Parse for each lines
	 */
	while (getline(&line_buf, &line_len, filp) > 0) {
		/*
		 * Expand catalog array, if necessary
		 */
		if (catalog->limit == catalog->nspec) {
			size_t		length;
			unsigned int	new_limit = 2 * catalog->limit;
			catalog_t      *new_catalog;

			length = sizeof(catalog_t)
				+ new_limit * sizeof(spec_t);
			new_catalog = realloc(catalog, length);
			if (!new_catalog)
				goto out_error;

			catalog = new_catalog;
			catalog->limit = new_limit;
		}

		/*
		 * Parse a line
		 */
		if (process_line(path, line_buf, ++line_num, catalog) < 0)
			goto out_error;
	}

	if (digest_add_specfile(rec->digest, filp, NULL, sb.st_size, path) < 0)
		goto out_error;

	digest_gen_hash(rec->digest);

	free(line_buf);
	fclose(filp);

	return catalog;

out_error:
	free(line_buf);
	for (i = 0; i < catalog->nspec; i++) {
		spec_t	       *spec = &catalog->specs[i];

		free(spec->key);
		free(spec->lr.ctx_raw);
		free(spec->lr.ctx_trans);
		__pthread_mutex_destroy(&spec->lr.lock);
	}
	free(catalog);
	fclose(filp);

	return NULL;
}

/*
 * Initialize selabel_handle and load the entries of specfile
 */
int selabel_db_init(struct selabel_handle *rec,
		    const struct selinux_opt *opts, unsigned nopts)
{
	rec->func_close = &db_close;
	rec->func_lookup = &db_lookup;
	rec->func_stats = &db_stats;
	rec->data = db_init(opts, nopts, rec);

	return !rec->data ? -1 : 0;
}
