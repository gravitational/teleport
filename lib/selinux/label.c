/*
 * Generalized labeling frontend for userspace object managers.
 *
 * Author : Eamon Walsh <ewalsh@epoch.ncsc.mil>
 */

#include <sys/types.h>
#include <ctype.h>
#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <selinux/selinux.h>
#include "callbacks.h"
#include "label_internal.h"

#define ARRAY_SIZE(x) (sizeof(x) / sizeof((x)[0]))

#ifdef NO_MEDIA_BACKEND
#define CONFIG_MEDIA_BACKEND(fnptr) NULL
#else
#define CONFIG_MEDIA_BACKEND(fnptr) &fnptr
#endif

#ifdef NO_X_BACKEND
#define CONFIG_X_BACKEND(fnptr) NULL
#else
#define CONFIG_X_BACKEND(fnptr) &fnptr
#endif

#ifdef NO_DB_BACKEND
#define CONFIG_DB_BACKEND(fnptr) NULL
#else
#define CONFIG_DB_BACKEND(fnptr) &fnptr
#endif

#ifdef NO_ANDROID_BACKEND
#define CONFIG_ANDROID_BACKEND(fnptr) NULL
#else
#define CONFIG_ANDROID_BACKEND(fnptr) (&(fnptr))
#endif

typedef int (*selabel_initfunc)(struct selabel_handle *rec,
				const struct selinux_opt *opts,
				unsigned nopts);

static const selabel_initfunc initfuncs[] = {
	&selabel_file_init,
	CONFIG_MEDIA_BACKEND(selabel_media_init),
	CONFIG_X_BACKEND(selabel_x_init),
	CONFIG_DB_BACKEND(selabel_db_init),
	CONFIG_ANDROID_BACKEND(selabel_property_init),
	CONFIG_ANDROID_BACKEND(selabel_service_init),
};

static inline struct selabel_digest *selabel_is_digest_set
				    (const struct selinux_opt *opts,
				    unsigned n)
{
	struct selabel_digest *digest = NULL;

	while (n) {
		n--;
		if (opts[n].type == SELABEL_OPT_DIGEST &&
					    !!opts[n].value) {
			digest = calloc(1, sizeof(*digest));
			if (!digest)
				goto err;

			digest->digest = calloc(1, DIGEST_SPECFILE_SIZE + 1);
			if (!digest->digest)
				goto err;

			digest->specfile_list = calloc(DIGEST_FILES_MAX,
							    sizeof(char *));
			if (!digest->specfile_list)
				goto err;

			return digest;
		}
	}
	return NULL;

err:
	if (digest) {
		free(digest->digest);
		free(digest->specfile_list);
		free(digest);
	}
	return NULL;
}

static void selabel_digest_fini(struct selabel_digest *ptr)
{
	int i;

	free(ptr->digest);
	free(ptr->hashbuf);

	if (ptr->specfile_list) {
		for (i = 0; ptr->specfile_list[i]; i++)
			free(ptr->specfile_list[i]);
		free(ptr->specfile_list);
	}
	free(ptr);
}

/*
 * Validation functions
 */

static inline int selabel_is_validate_set(const struct selinux_opt *opts,
					  unsigned n)
{
	while (n) {
		n--;
		if (opts[n].type == SELABEL_OPT_VALIDATE)
			return !!opts[n].value;
	}

	return 0;
}

int selabel_validate(struct selabel_lookup_rec *contexts)
{
	bool validated;
	int rc;

	validated = __atomic_load_n(&contexts->validated, __ATOMIC_ACQUIRE);
	if (validated)
		return 0;

	__pthread_mutex_lock(&contexts->lock);

	/* Check if another thread validated the context while we waited on the mutex */
	validated = __atomic_load_n(&contexts->validated, __ATOMIC_ACQUIRE);
	if (validated) {
		__pthread_mutex_unlock(&contexts->lock);
		return 0;
	}

	rc = selinux_validate(&contexts->ctx_raw);
	if (rc == 0)
		__atomic_store_n(&contexts->validated, true, __ATOMIC_RELEASE);

	__pthread_mutex_unlock(&contexts->lock);

	if (rc < 0)
		return -1;

	return 0;
}

/* Public API helpers */
static int selabel_fini(const struct selabel_handle *rec,
			    struct selabel_lookup_rec *lr,
			    bool translating)
{
	char *ctx_trans;
	int rc;

	if (compat_validate(rec, lr, rec->spec_file, lr->lineno))
		return -1;

	if (!translating)
		return 0;

	ctx_trans = __atomic_load_n(&lr->ctx_trans, __ATOMIC_ACQUIRE);
	if (ctx_trans)
		return 0;

	__pthread_mutex_lock(&lr->lock);

	/* Check if another thread translated the context while we waited on the mutex */
	ctx_trans = __atomic_load_n(&lr->ctx_trans, __ATOMIC_ACQUIRE);
	if (ctx_trans) {
		__pthread_mutex_unlock(&lr->lock);
		return 0;
	}

	rc = selinux_raw_to_trans_context(lr->ctx_raw, &ctx_trans);
	if (rc == 0)
		__atomic_store_n(&lr->ctx_trans, ctx_trans, __ATOMIC_RELEASE);

	__pthread_mutex_unlock(&lr->lock);

	if (rc)
		return -1;

	return 0;
}

static struct selabel_lookup_rec *
selabel_lookup_common(struct selabel_handle *rec, bool translating,
		      const char *key, int type)
{
	struct selabel_lookup_rec *lr;

	if (key == NULL) {
		errno = EINVAL;
		return NULL;
	}

	lr = rec->func_lookup(rec, key, type);
	if (!lr)
		return NULL;

	if (selabel_fini(rec, lr, translating))
		return NULL;

	return lr;
}

static struct selabel_lookup_rec *
selabel_lookup_bm_common(struct selabel_handle *rec, bool translating,
		      const char *key, int type, const char **aliases)
{
	struct selabel_lookup_rec *lr;

	if (key == NULL) {
		errno = EINVAL;
		return NULL;
	}

	lr = rec->func_lookup_best_match(rec, key, aliases, type);
	if (!lr)
		return NULL;

	if (selabel_fini(rec, lr, translating))
		return NULL;

	return lr;
}

/*
 * Public API
 */

struct selabel_handle *selabel_open(unsigned int backend,
				    const struct selinux_opt *opts,
				    unsigned nopts)
{
	struct selabel_handle *rec = NULL;

	if (backend >= ARRAY_SIZE(initfuncs)) {
		errno = EINVAL;
		goto out;
	}

	if (!initfuncs[backend]) {
		errno = ENOTSUP;
		goto out;
	}

	rec = (struct selabel_handle *)calloc(1, sizeof(*rec));
	if (!rec)
		goto out;

	rec->backend = backend;
	rec->validating = selabel_is_validate_set(opts, nopts);

	rec->digest = selabel_is_digest_set(opts, nopts);

	if ((*initfuncs[backend])(rec, opts, nopts)) {
		selabel_close(rec);
		rec = NULL;
	}

out:
	return rec;
}

int selabel_lookup(struct selabel_handle *rec, char **con,
		   const char *key, int type)
{
	struct selabel_lookup_rec *lr;

	lr = selabel_lookup_common(rec, true, key, type);
	if (!lr)
		return -1;

	*con = strdup(lr->ctx_trans);
	return *con ? 0 : -1;
}

int selabel_lookup_raw(struct selabel_handle *rec, char **con,
		       const char *key, int type)
{
	struct selabel_lookup_rec *lr;

	lr = selabel_lookup_common(rec, false, key, type);
	if (!lr)
		return -1;

	*con = strdup(lr->ctx_raw);
	return *con ? 0 : -1;
}

bool selabel_partial_match(struct selabel_handle *rec, const char *key)
{
	if (!rec->func_partial_match) {
		/*
		 * If the label backend does not support partial matching,
		 * then assume a match is possible.
		 */
		return true;
	}

	return rec->func_partial_match(rec, key);
}

bool selabel_get_digests_all_partial_matches(struct selabel_handle *rec,
					     const char *key,
					     uint8_t **calculated_digest,
					     uint8_t **xattr_digest,
					     size_t *digest_len)
{
	if (!rec->func_get_digests_all_partial_matches)
		return false;

	return rec->func_get_digests_all_partial_matches(rec, key,
							 calculated_digest,
							 xattr_digest,
							 digest_len);
}

bool selabel_hash_all_partial_matches(struct selabel_handle *rec,
                                      const char *key, uint8_t *digest) {
	if (!rec->func_hash_all_partial_matches) {
		return false;
	}

	return rec->func_hash_all_partial_matches(rec, key, digest);
}

int selabel_lookup_best_match(struct selabel_handle *rec, char **con,
			      const char *key, const char **aliases, int type)
{
	struct selabel_lookup_rec *lr;

	if (!rec->func_lookup_best_match) {
		errno = ENOTSUP;
		return -1;
	}

	lr = selabel_lookup_bm_common(rec, true, key, type, aliases);
	if (!lr)
		return -1;

	*con = strdup(lr->ctx_trans);
	return *con ? 0 : -1;
}

int selabel_lookup_best_match_raw(struct selabel_handle *rec, char **con,
			      const char *key, const char **aliases, int type)
{
	struct selabel_lookup_rec *lr;

	if (!rec->func_lookup_best_match) {
		errno = ENOTSUP;
		return -1;
	}

	lr = selabel_lookup_bm_common(rec, false, key, type, aliases);
	if (!lr)
		return -1;

	*con = strdup(lr->ctx_raw);
	return *con ? 0 : -1;
}

enum selabel_cmp_result selabel_cmp(const struct selabel_handle *h1,
				    const struct selabel_handle *h2)
{
	if (!h1->func_cmp || h1->func_cmp != h2->func_cmp)
		return SELABEL_INCOMPARABLE;

	return h1->func_cmp(h1, h2);
}

int selabel_digest(struct selabel_handle *rec,
				    unsigned char **digest, size_t *digest_len,
				    char ***specfiles, size_t *num_specfiles)
{
	if (!rec->digest) {
		errno = EINVAL;
		return -1;
	}

	*digest = rec->digest->digest;
	*digest_len = DIGEST_SPECFILE_SIZE;
	*specfiles = rec->digest->specfile_list;
	*num_specfiles = rec->digest->specfile_cnt;
	return 0;
}

void selabel_close(struct selabel_handle *rec)
{
	if (rec->digest)
		selabel_digest_fini(rec->digest);
	rec->func_close(rec);
	free(rec->spec_file);
	free(rec);
}

void selabel_stats(struct selabel_handle *rec)
{
	rec->func_stats(rec);
}
