#include <assert.h>
#include <sys/stat.h>
#include <string.h>
#include <errno.h>
#include <stdio.h>
#include "selinux_internal.h"
#include "label_internal.h"
#include "callbacks.h"
#include <limits.h>

static int (*myinvalidcon) (const char *p, unsigned l, char *c) = NULL;
static int (*mycanoncon) (const char *p, unsigned l, char **c) =  NULL;

static void
#ifdef __GNUC__
    __attribute__ ((format(printf, 1, 2)))
#endif
    default_printf(const char *fmt, ...)
{
	va_list ap;
	va_start(ap, fmt);
	vfprintf(stderr, fmt, ap);
	va_end(ap);
}

void
#ifdef __GNUC__
    __attribute__ ((format(printf, 1, 2)))
#endif
    (*myprintf) (const char *fmt,...) = &default_printf;
int myprintf_compat = 0;

void set_matchpathcon_printf(void (*f) (const char *fmt, ...))
{
	myprintf = f ? f : &default_printf;
	myprintf_compat = 1;
}

int compat_validate(const struct selabel_handle *rec,
		    struct selabel_lookup_rec *contexts,
		    const char *path, unsigned lineno)
{
	int rc;
	char **ctx = &contexts->ctx_raw;

	if (myinvalidcon)
		rc = myinvalidcon(path, lineno, *ctx);
	else if (mycanoncon)
		rc = mycanoncon(path, lineno, ctx);
	else if (rec->validating) {
		rc = selabel_validate(contexts);
		if (rc < 0) {
			if (lineno) {
				COMPAT_LOG(SELINUX_WARNING,
					    "%s: line %u has invalid context %s\n",
						path, lineno, *ctx);
			} else {
				COMPAT_LOG(SELINUX_WARNING,
					    "%s: has invalid context %s\n", path, *ctx);
			}
		}
	} else
		rc = 0;

	return rc ? -1 : 0;
}

#ifndef BUILD_HOST

static __thread struct selabel_handle *hnd;

/*
 * An array for mapping integers to contexts
 */
static __thread char **con_array;
static __thread int con_array_size;
static __thread int con_array_used;

static pthread_once_t once = PTHREAD_ONCE_INIT;
static pthread_key_t destructor_key;
static int destructor_key_initialized = 0;

static void free_array_elts(void)
{
	int i;
	for (i = 0; i < con_array_used; i++)
		free(con_array[i]);
	free(con_array);

	con_array_size = con_array_used = 0;
	con_array = NULL;
}

static int add_array_elt(char *con)
{
	char **tmp;
	if (con_array_size) {
		while (con_array_used >= con_array_size) {
			con_array_size *= 2;
			tmp = (char **)reallocarray(con_array, con_array_size,
						    sizeof(char*));
			if (!tmp) {
				free_array_elts();
				return -1;
			}
			con_array = tmp;
		}
	} else {
		con_array_size = 1000;
		con_array = (char **)malloc(sizeof(char*) * con_array_size);
		if (!con_array) {
			con_array_size = con_array_used = 0;
			return -1;
		}
	}

	con_array[con_array_used] = strdup(con);
	if (!con_array[con_array_used])
		return -1;
	return con_array_used++;
}

void set_matchpathcon_invalidcon(int (*f) (const char *p, unsigned l, char *c))
{
	myinvalidcon = f;
}

static int default_canoncon(const char *path, unsigned lineno, char **context)
{
	char *tmpcon;
	if (security_canonicalize_context_raw(*context, &tmpcon) < 0) {
		if (errno == ENOENT)
			return 0;
		if (lineno)
			myprintf("%s:  line %u has invalid context %s\n", path,
				 lineno, *context);
		else
			myprintf("%s:  invalid context %s\n", path, *context);
		return 1;
	}
	free(*context);
	*context = tmpcon;
	return 0;
}

void set_matchpathcon_canoncon(int (*f) (const char *p, unsigned l, char **c))
{
	if (f)
		mycanoncon = f;
	else
		mycanoncon = &default_canoncon;
}

static __thread struct selinux_opt options[SELABEL_NOPT];
static __thread int notrans;

void set_matchpathcon_flags(unsigned int flags)
{
	int i;
	memset(options, 0, sizeof(options));
	i = SELABEL_OPT_BASEONLY;
	options[i].type = i;
	options[i].value = (flags & MATCHPATHCON_BASEONLY) ? (char*)1 : NULL;
	i = SELABEL_OPT_VALIDATE;
	options[i].type = i;
	options[i].value = (flags & MATCHPATHCON_VALIDATE) ? (char*)1 : NULL;
	notrans = flags & MATCHPATHCON_NOTRANS;
}

/*
 * An association between an inode and a 
 * specification.  
 */
typedef struct file_spec {
	ino_t ino;		/* inode number */
	int specind;		/* index of specification in spec */
	char *file;		/* full pathname for diagnostic messages about conflicts */
	struct file_spec *next;	/* next association in hash bucket chain */
} file_spec_t;

/*
 * The hash table of associations, hashed by inode number.
 * Chaining is used for collisions, with elements ordered
 * by inode number in each bucket.  Each hash bucket has a dummy 
 * header.
 */
#define HASH_BITS 16
#define HASH_BUCKETS (1 << HASH_BITS)
#define HASH_MASK (HASH_BUCKETS-1)
static file_spec_t *fl_head;

/*
 * Try to add an association between an inode and
 * a specification.  If there is already an association
 * for the inode and it conflicts with this specification,
 * then use the specification that occurs later in the
 * specification array.
 */
int matchpathcon_filespec_add(ino_t ino, int specind, const char *file)
{
	file_spec_t *prevfl, *fl;
	int h, ret;
	struct stat sb;

	if (!fl_head) {
		fl_head = calloc(HASH_BUCKETS, sizeof(file_spec_t));
		if (!fl_head)
			goto oom;
	}

	h = (ino + (ino >> HASH_BITS)) & HASH_MASK;
	for (prevfl = &fl_head[h], fl = fl_head[h].next; fl;
	     prevfl = fl, fl = fl->next) {
		if (ino == fl->ino) {
			ret = lstat(fl->file, &sb);
			if (ret < 0 || sb.st_ino != ino) {
				fl->specind = specind;
				free(fl->file);
				fl->file = strdup(file);
				if (!fl->file)
					goto oom;
				return fl->specind;

			}

			if (!strcmp(con_array[fl->specind],
				    con_array[specind]))
				return fl->specind;

			myprintf
			    ("%s:  conflicting specifications for %s and %s, using %s.\n",
			     __FUNCTION__, file, fl->file,
			     con_array[fl->specind]);
			free(fl->file);
			fl->file = strdup(file);
			if (!fl->file)
				goto oom;
			return fl->specind;
		}

		if (ino > fl->ino)
			break;
	}

	fl = malloc(sizeof(file_spec_t));
	if (!fl)
		goto oom;
	fl->ino = ino;
	fl->specind = specind;
	fl->file = strdup(file);
	if (!fl->file)
		goto oom_freefl;
	fl->next = prevfl->next;
	prevfl->next = fl;
	return fl->specind;
      oom_freefl:
	free(fl);
      oom:
	myprintf("%s:  insufficient memory for file label entry for %s\n",
		 __FUNCTION__, file);
	return -1;
}

#if (defined(_FILE_OFFSET_BITS) && _FILE_OFFSET_BITS == 64) && defined(__INO64_T_TYPE) && !defined(__INO_T_MATCHES_INO64_T)
/* alias defined in the public header but we undefine it here */
#undef matchpathcon_filespec_add

/* ABI backwards-compatible shim for non-LFS 32-bit systems */

static_assert(sizeof(unsigned long) == sizeof(__ino_t), "inode size mismatch");
static_assert(sizeof(unsigned long) == sizeof(uint32_t), "inode size mismatch");
static_assert(sizeof(ino_t) == sizeof(ino64_t), "inode size mismatch");
static_assert(sizeof(ino64_t) == sizeof(uint64_t), "inode size mismatch");

extern int matchpathcon_filespec_add(unsigned long ino, int specind,
                                     const char *file);

int matchpathcon_filespec_add(unsigned long ino, int specind,
                              const char *file)
{
	return matchpathcon_filespec_add64(ino, specind, file);
}
#elif (defined(_FILE_OFFSET_BITS) && _FILE_OFFSET_BITS == 64) || defined(__INO_T_MATCHES_INO64_T)

static_assert(sizeof(uint64_t) == sizeof(ino_t), "inode size mismatch");

#else

static_assert(sizeof(uint32_t) == sizeof(ino_t), "inode size mismatch");

#endif

/*
 * Evaluate the association hash table distribution.
 */
void matchpathcon_filespec_eval(void)
{
	file_spec_t *fl;
	int h, used, nel, len, longest;

	if (!fl_head)
		return;

	used = 0;
	longest = 0;
	nel = 0;
	for (h = 0; h < HASH_BUCKETS; h++) {
		len = 0;
		for (fl = fl_head[h].next; fl; fl = fl->next) {
			len++;
		}
		if (len)
			used++;
		if (len > longest)
			longest = len;
		nel += len;
	}

	myprintf
	    ("%s:  hash table stats: %d elements, %d/%d buckets used, longest chain length %d\n",
	     __FUNCTION__, nel, used, HASH_BUCKETS, longest);
}

/*
 * Destroy the association hash table.
 */
void matchpathcon_filespec_destroy(void)
{
	file_spec_t *fl, *tmp;
	int h;

	free_array_elts();

	if (!fl_head)
		return;

	for (h = 0; h < HASH_BUCKETS; h++) {
		fl = fl_head[h].next;
		while (fl) {
			tmp = fl;
			fl = fl->next;
			free(tmp->file);
			free(tmp);
		}
		fl_head[h].next = NULL;
	}
	free(fl_head);
	fl_head = NULL;
}

static void matchpathcon_fini_internal(void)
{
	free_array_elts();

	if (hnd) {
		selabel_close(hnd);
		hnd = NULL;
	}
}

static void matchpathcon_thread_destructor(void __attribute__((unused)) *ptr)
{
	matchpathcon_fini_internal();
}

void __attribute__((destructor)) matchpathcon_lib_destructor(void);

void  __attribute__((destructor)) matchpathcon_lib_destructor(void)
{
	if (destructor_key_initialized)
		__selinux_key_delete(destructor_key);
}

static void matchpathcon_init_once(void)
{
	if (__selinux_key_create(&destructor_key, matchpathcon_thread_destructor) == 0)
		destructor_key_initialized = 1;
}

int matchpathcon_init_prefix(const char *path, const char *subset)
{
	if (!mycanoncon)
		mycanoncon = default_canoncon;

	__selinux_once(once, matchpathcon_init_once);
	__selinux_setspecific(destructor_key, /* some valid address to please GCC */ &selinux_page_size);

	options[SELABEL_OPT_SUBSET].type = SELABEL_OPT_SUBSET;
	options[SELABEL_OPT_SUBSET].value = subset;
	options[SELABEL_OPT_PATH].type = SELABEL_OPT_PATH;
	options[SELABEL_OPT_PATH].value = path;

	hnd = selabel_open(SELABEL_CTX_FILE, options, SELABEL_NOPT);
	return hnd ? 0 : -1;
}


int matchpathcon_init(const char *path)
{
	return matchpathcon_init_prefix(path, NULL);
}

void matchpathcon_fini(void)
{
	matchpathcon_fini_internal();
}

/*
 * We do not want to resolve a symlink to a real path if it is the final
 * component of the name.  Thus we split the pathname on the last "/" and
 * determine a real path component of the first portion.  We then have to
 * copy the last part back on to get the final real path.  Wheww.
 */
int realpath_not_final(const char *name, char *resolved_path)
{
	char *last_component;
	char *tmp_path, *p;
	size_t len = 0;
	int rc = 0;

	tmp_path = strdup(name);
	if (!tmp_path) {
		myprintf("symlink_realpath(%s) strdup() failed: %m\n",
			name);
		rc = -1;
		goto out;
	}

	last_component = strrchr(tmp_path, '/');

	if (last_component == tmp_path) {
		last_component++;
		p = strcpy(resolved_path, "");
	} else if (last_component) {
		*last_component = '\0';
		last_component++;
		p = realpath(tmp_path, resolved_path);
	} else {
		last_component = tmp_path;
		p = realpath("./", resolved_path);
	}

	if (!p) {
		myprintf("symlink_realpath(%s) realpath() failed: %m\n",
			name);
		rc = -1;
		goto out;
	}

	len = strlen(p);
	if (len + strlen(last_component) + 2 > PATH_MAX) {
		myprintf("symlink_realpath(%s) failed: Filename too long \n",
			name);
		errno = ENAMETOOLONG;
		rc = -1;
		goto out;
	}

	resolved_path += len;
	strcpy(resolved_path, "/");
	resolved_path += 1;
	strcpy(resolved_path, last_component);
out:
	free(tmp_path);
	return rc;
}

static int matchpathcon_internal(const char *path, mode_t mode, char ** con)
{
	char stackpath[PATH_MAX + 1];
	char *p = NULL;
	if (!hnd && (matchpathcon_init_prefix(NULL, NULL) < 0))
			return -1;

	if (S_ISLNK(mode)) {
		if (!realpath_not_final(path, stackpath))
			path = stackpath;
	} else {
		p = realpath(path, stackpath);
		if (p)
			path = p;
	}

	return notrans ?
		selabel_lookup_raw(hnd, con, path, mode) :
		selabel_lookup(hnd, con, path, mode);
}

int matchpathcon(const char *path, mode_t mode, char ** con) {
	return matchpathcon_internal(path, mode, con);
}

int matchpathcon_index(const char *name, mode_t mode, char ** con)
{
	int i = matchpathcon_internal(name, mode, con);

	if (i < 0)
		return -1;

	return add_array_elt(*con);
}

void matchpathcon_checkmatches(char *str __attribute__((unused)))
{
	selabel_stats(hnd);
}

/* Compare two contexts to see if their differences are "significant",
 * or whether the only difference is in the user. */
int selinux_file_context_cmp(const char * a,
			     const char * b)
{
	const char *rest_a, *rest_b;	/* Rest of the context after the user */
	if (!a && !b)
		return 0;
	if (!a)
		return -1;
	if (!b)
		return 1;
	rest_a = strchr(a, ':');
	rest_b = strchr(b, ':');
	if (!rest_a && !rest_b)
		return 0;
	if (!rest_a)
		return -1;
	if (!rest_b)
		return 1;
	return strcmp(rest_a, rest_b);
}

int selinux_file_context_verify(const char *path, mode_t mode)
{
	char * con = NULL;
	char * fcontext = NULL;
	int rc = 0;
	char stackpath[PATH_MAX + 1];
	char *p = NULL;

	if (S_ISLNK(mode)) {
		if (!realpath_not_final(path, stackpath))
			path = stackpath;
	} else {
		p = realpath(path, stackpath);
		if (p)
			path = p;
	}

	rc = lgetfilecon_raw(path, &con);
	if (rc == -1) {
		if (errno != ENOTSUP)
			return -1;
		else
			return 0;
	}
	
	if (!hnd && (matchpathcon_init_prefix(NULL, NULL) < 0)){
			freecon(con);
			return -1;
	}

	if (selabel_lookup_raw(hnd, &fcontext, path, mode) != 0) {
		if (errno != ENOENT)
			rc = -1;
		else
			rc = 0;
	} else {
		/*
		 * Need to set errno to 0 as it can be set to ENOENT if the
		 * file_contexts.subs file does not exist (see selabel_open in
		 * label.c), thus causing confusion if errno is checked on return.
		 */
		errno = 0;
		rc = (selinux_file_context_cmp(fcontext, con) == 0);
	}

	freecon(con);
	freecon(fcontext);
	return rc;
}

int selinux_lsetfilecon_default(const char *path)
{
	struct stat st;
	int rc = -1;
	char * scontext = NULL;
	if (lstat(path, &st) != 0)
		return rc;

	if (!hnd && (matchpathcon_init_prefix(NULL, NULL) < 0))
			return -1;

	/* If there's an error determining the context, or it has none, 
	   return to allow default context */
	if (selabel_lookup_raw(hnd, &scontext, path, st.st_mode)) {
		if (errno == ENOENT)
			rc = 0;
	} else {
		rc = lsetfilecon_raw(path, scontext);
		freecon(scontext);
	}
	return rc;
}

#endif
