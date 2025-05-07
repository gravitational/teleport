#ifndef _SELABEL_FILE_H_
#define _SELABEL_FILE_H_

#include <assert.h>
#include <ctype.h>
#include <errno.h>
#include <pthread.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

#include <sys/stat.h>
#include <sys/xattr.h>

/*
 * regex.h/c were introduced to hold all dependencies on the regular
 * expression back-end when we started supporting PCRE2. regex.h defines a
 * minimal interface required by libselinux, so that the remaining code
 * can be agnostic about the underlying implementation.
 */
#include "regex.h"

#include "callbacks.h"
#include "label_internal.h"
#include "selinux_internal.h"

#define SELINUX_MAGIC_COMPILED_FCONTEXT	0xf97cff8a

/* Version specific changes */
#define SELINUX_COMPILED_FCONTEXT_NOPCRE_VERS	1
#define SELINUX_COMPILED_FCONTEXT_PCRE_VERS	2
#define SELINUX_COMPILED_FCONTEXT_MODE		3
#define SELINUX_COMPILED_FCONTEXT_PREFIX_LEN	4
#define SELINUX_COMPILED_FCONTEXT_REGEX_ARCH	5
#define SELINUX_COMPILED_FCONTEXT_TREE_LAYOUT	6

#define SELINUX_COMPILED_FCONTEXT_MAX_VERS \
	SELINUX_COMPILED_FCONTEXT_TREE_LAYOUT

/* Required selinux_restorecon and selabel_get_digests_all_partial_matches() */
#define RESTORECON_PARTIAL_MATCH_DIGEST  "security.sehash"

#define LABEL_FILE_KIND_INVALID		255
#define LABEL_FILE_KIND_ALL		0
#define LABEL_FILE_KIND_DIR		1
#define LABEL_FILE_KIND_CHR		2
#define LABEL_FILE_KIND_BLK		3
#define LABEL_FILE_KIND_SOCK		4
#define LABEL_FILE_KIND_FIFO		5
#define LABEL_FILE_KIND_LNK		6
#define LABEL_FILE_KIND_REG		7

/* Only exported for fuzzing */
struct lookup_result {
	const char *regex_str;
	struct selabel_lookup_rec *lr;
	uint16_t prefix_len;
	uint8_t file_kind;
	bool has_meta_chars;
	struct lookup_result *next;
};
#ifdef FUZZING_BUILD_MODE_UNSAFE_FOR_PRODUCTION
extern int load_mmap(FILE *fp, const size_t len, struct selabel_handle *rec, const char *path, uint8_t inputno);
extern int process_text_file(FILE *fp, const char *prefix, struct selabel_handle *rec, const char *path, uint8_t inputno);
extern void free_lookup_result(struct lookup_result *result);
extern struct lookup_result *lookup_all(struct selabel_handle *rec, const char *key, int type, bool partial, bool find_all, struct lookup_result *buf);
extern enum selabel_cmp_result cmp(const struct selabel_handle *h1, const struct selabel_handle *h2);
#endif  /* FUZZING_BUILD_MODE_UNSAFE_FOR_PRODUCTION */

/* A path substitution entry */
struct selabel_sub {
	char *src;				/* source path prefix */
	char *dst;				/* substituted path prefix */
	uint32_t slen;				/* length of source path prefix */
	uint32_t dlen;				/* length of substituted path prefix */
};

/* A regular expression file security context specification */
struct regex_spec {
	struct selabel_lookup_rec lr;		/* contexts for lookup result */
	char *regex_str;			/* original regular expression string for diagnostics */
	struct regex_data *regex;		/* backend dependent regular expression data */
	pthread_mutex_t regex_lock;		/* lock for lazy compilation of regex */
	uint32_t lineno;			/* Line number in source file */
	uint16_t prefix_len;			/* length of fixed path prefix */
	uint8_t inputno;			/* Input number of source file */
	uint8_t file_kind;			/* file type */
	bool regex_compiled;			/* whether the regex is compiled */
	bool any_matches;			/* whether any pathname match */
	bool from_mmap;				/* whether this spec is from an mmap of the data */
};

/* A literal file security context specification */
struct literal_spec {
	struct selabel_lookup_rec lr;		/* contexts for lookup result */
	char *regex_str;			/* original regular expression string for diagnostics */
	char *literal_match;			/* simplified string from regular expression */
	uint16_t prefix_len;			/* length of fixed path prefix, i.e. length of the literal match */
	uint8_t file_kind;			/* file type */
	bool any_matches;			/* whether any pathname match */
	bool from_mmap;				/* whether this spec is from an mmap of the data */
};

/*
 * Max depth of specification nodes
 *
 * Measure before changing:
 *   - 2  leads to slower lookup
 *   - >4 require more memory (and allocations) for no performance gain
 */
#define SPEC_NODE_MAX_DEPTH 3

/* A specification node */
struct spec_node {
	/* stem of the node, or NULL for root node */
	char *stem;

	/* parent node */
	struct spec_node *parent;

	/*
	 * Array of literal specifications (ordered alphabetically)
	 */
	struct literal_spec *literal_specs;
	uint32_t literal_specs_num, literal_specs_alloc;

	/*
	 * Array of regular expression specifications (order preserved from input)
	 */
	struct regex_spec *regex_specs;
	uint32_t regex_specs_num, regex_specs_alloc;

	/*
	 * Array of child nodes (ordered alphabetically)
	 */
	struct spec_node *children;
	uint32_t children_num, children_alloc;

	/* length of the stem (reordered to minimize padding) */
	uint16_t stem_len;

	/* whether this node is from an mmap of the data */
	bool from_mmap;
};

/* Where we map the file in during selabel_open() */
struct mmap_area {
	void *addr;		/* Start addr + len used to release memory at close */
	size_t len;
	void *next_addr;	/* Incremented by next_entry() */
	size_t next_len;	/* Decremented by next_entry() */
	struct mmap_area *next;
};

/* Our stored configuration */
struct saved_data {
	/* Root specification node */
	struct spec_node *root;

	/* Number of file specifications */
	uint64_t num_specs;

	struct mmap_area *mmap_areas;

	/*
	 * Array of distribution substitutions
	 */
	struct selabel_sub *dist_subs;
	uint32_t dist_subs_num, dist_subs_alloc;

	/*
	 * Array of local substitutions
	 */
	struct selabel_sub *subs;
	uint32_t subs_num, subs_alloc;
};

void free_spec_node(struct spec_node *node);
void sort_spec_node(struct spec_node *node, struct spec_node *parent);

static inline mode_t string_to_file_kind(const char *mode)
{
	if (mode[0] != '-' || mode[1] == '\0' || mode[2] != '\0')
		return LABEL_FILE_KIND_INVALID;
	switch (mode[1]) {
	case 'b':
		return LABEL_FILE_KIND_BLK;
	case 'c':
		return LABEL_FILE_KIND_CHR;
	case 'd':
		return LABEL_FILE_KIND_DIR;
	case 'p':
		return LABEL_FILE_KIND_FIFO;
	case 'l':
		return LABEL_FILE_KIND_LNK;
	case 's':
		return LABEL_FILE_KIND_SOCK;
	case '-':
		return LABEL_FILE_KIND_REG;
	default:
		return LABEL_FILE_KIND_INVALID;
	}
}

static inline const char* file_kind_to_string(uint8_t file_kind)
{
	switch (file_kind) {
	case LABEL_FILE_KIND_BLK:
		return "block-device";
	case LABEL_FILE_KIND_CHR:
		return "character-device";
	case LABEL_FILE_KIND_DIR:
		return "directory";
	case LABEL_FILE_KIND_FIFO:
		return "fifo-file";
	case LABEL_FILE_KIND_LNK:
		return "symlink";
	case LABEL_FILE_KIND_SOCK:
		return "sock-file";
	case LABEL_FILE_KIND_REG:
		return "regular-file";
	case LABEL_FILE_KIND_ALL:
		return "wildcard";
	default:
		return "(invalid)";
	}
}

/*
 * Determine whether the regular expression specification has any meta characters
 * or any unsupported escape sequence.
 */
static bool regex_has_meta_chars(const char *regex, size_t *prefix_len, const char *path, unsigned int lineno)
{
	const char *p = regex;
	size_t plen = 0;

	for (;*p != '\0'; p++, plen++) {
		switch(*p) {
		case '.':
		case '^':
		case '$':
		case '?':
		case '*':
		case '+':
		case '|':
		case '[':
		case '(':
		case '{':
		case ']':
		case ')':
		case '}':
			*prefix_len = plen;
			return true;
		case '\\':
			p++;
			switch (*p) {
			/* curated list of supported characters */
			case '.':
			case '^':
			case '$':
			case '?':
			case '*':
			case '+':
			case '|':
			case '[':
			case '(':
			case '{':
			case ']':
			case ')':
			case '}':
			case '-':
			case '_':
			case ',':
				continue;
			default:
				COMPAT_LOG(SELINUX_INFO, "%s:  line %u has unsupported escaped character %c (%#x) for literal matching, continuing using regex\n",
					   path, lineno, isprint((unsigned char)*p) ? *p : '?', *p);
				*prefix_len = plen;
				return true;
			}
		}
	}

	*prefix_len = plen;
	return false;
}

static int regex_simplify(const char *regex, size_t len, char **out, const char *path, unsigned int lineno)
{
	char *result, *p;
	size_t i = 0;

	result = malloc(len + 1);
	if (!result)
		return -1;

	p = result;
	while (i < len) {
		switch(regex[i]) {
		case '.':
		case '^':
		case '$':
		case '?':
		case '*':
		case '+':
		case '|':
		case '[':
		case '(':
		case '{':
		case ']':
		case ')':
		case '}':
			free(result);
			return 0;
		case '\\':
			i++;
			if (i >= len) {
				COMPAT_LOG(SELINUX_WARNING, "%s:  line %u has unsupported final escape character\n",
					   path, lineno);
				free(result);
				return 0;
			}
			switch (regex[i]) {
			/* curated list of supported characters */
			case '.':
			case '^':
			case '$':
			case '?':
			case '*':
			case '+':
			case '|':
			case '[':
			case '(':
			case '{':
			case ']':
			case ')':
			case '}':
			case '-':
			case '_':
			case ',':
				*p++ = regex[i++];
				break;
			default:
				/* regex_has_meta_chars() reported already the notable occurrences */
				free(result);
				return 0;
			}
			break;
		default:
			*p++ = regex[i++];
		}
	}

	*p = '\0';
	*out = result;
	return 1;
}

static inline int compare_literal_spec(const void *p1, const void *p2)
{
	const struct literal_spec *l1 = p1;
	const struct literal_spec *l2 = p2;
	int ret;

	ret = strcmp(l1->literal_match, l2->literal_match);
	if (ret)
		return ret;

	/* Order wildcard mode (0) last */
	return (l1->file_kind < l2->file_kind) - (l1->file_kind > l2->file_kind);
}

static inline int compare_spec_node(const void *p1, const void *p2)
{
	const struct spec_node *n1 = p1;
	const struct spec_node *n2 = p2;
	int rc;

	rc = strcmp(n1->stem, n2->stem);
	/* There should not be two nodes with the same stem in the same array */
	assert(rc != 0);
	return rc;
}

static inline void sort_specs(struct saved_data *data)
{
	sort_spec_node(data->root, NULL);
}

static inline int compile_regex(struct regex_spec *spec, char *errbuf, size_t errbuf_size)
{
	const char *reg_buf;
	char *anchored_regex, *cp;
	struct regex_error_data error_data;
	size_t len;
	int rc;
	bool regex_compiled;

	if (!errbuf || errbuf_size == 0) {
	    errno = EINVAL;
	    return -1;
	}

	*errbuf = '\0';

	/* We really want pthread_once() here, but since its
	 * init_routine does not take a parameter, it's not possible
	 * to use, so we generate the same effect with atomics and a
	 * mutex */
#ifdef __ATOMIC_RELAXED
	regex_compiled =
		__atomic_load_n(&spec->regex_compiled, __ATOMIC_ACQUIRE);
#else
	/* GCC <4.7 */
	__sync_synchronize();
	regex_compiled = spec->regex_compiled;
#endif
	if (regex_compiled) {
		return 0; /* already done */
	}

	__pthread_mutex_lock(&spec->regex_lock);
	/* Check if another thread compiled the regex while we waited
	 * on the mutex */
#ifdef __ATOMIC_RELAXED
	regex_compiled =
		__atomic_load_n(&spec->regex_compiled, __ATOMIC_ACQUIRE);
#else
	/* GCC <4.7 */
	__sync_synchronize();
	regex_compiled = spec->regex_compiled;
#endif
	if (regex_compiled) {
		__pthread_mutex_unlock(&spec->regex_lock);
		return 0;
	}

	reg_buf = spec->regex_str;
	/* Anchor the regular expression. */
	len = strlen(reg_buf);
	/* Use a sufficient large upper bound for regular expression lengths
	 * to limit the compilation time on malformed inputs. */
	if (len >= 4096) {
		__pthread_mutex_unlock(&spec->regex_lock);
		snprintf(errbuf, errbuf_size, "regex of length %zu too long", len);
		errno = EINVAL;
		return -1;
	}
	cp = anchored_regex = malloc(len + 3);
	if (!anchored_regex) {
		__pthread_mutex_unlock(&spec->regex_lock);
		snprintf(errbuf, errbuf_size, "out of memory");
		return -1;
	}

	/* Create ^...$ regexp.  */
	*cp++ = '^';
	memcpy(cp, reg_buf, len);
	cp += len;
	*cp++ = '$';
	*cp = '\0';

	/* Compile the regular expression. */
	rc = regex_prepare_data(&spec->regex, anchored_regex, &error_data);
	free(anchored_regex);
	if (rc < 0) {
		regex_format_error(&error_data, errbuf, errbuf_size);
		__pthread_mutex_unlock(&spec->regex_lock);
		errno = EINVAL;
		return -1;
	}

	/* Done. */
#ifdef __ATOMIC_RELAXED
	__atomic_store_n(&spec->regex_compiled, true, __ATOMIC_RELEASE);
#else
	/* GCC <4.7 */
	spec->regex_compiled = true;
	__sync_synchronize();
#endif
	__pthread_mutex_unlock(&spec->regex_lock);
	return 0;
}

#define GROW_ARRAY(arr) ({                                                                  \
	int ret_;                                                                           \
	if ((arr ## _num) < (arr ## _alloc)) {                                              \
		ret_ = 0;                                                                   \
	} else {                                                                            \
		size_t addedsize_ = ((arr ## _alloc) >> 1) + ((arr ## _alloc >> 4)) + 4;    \
		size_t newsize_ = addedsize_ + (arr ## _alloc);                             \
		if (newsize_ < (arr ## _alloc) || newsize_ >= (typeof(arr ## _alloc))-1) {  \
			errno = EOVERFLOW;                                                  \
			ret_ = -1;                                                          \
		} else {                                                                    \
			typeof(arr) tmp_ = reallocarray(arr, newsize_, sizeof(*(arr)));     \
			if (!tmp_) {                                                        \
				ret_ = -1;                                                  \
			} else {                                                            \
				(arr) = tmp_;                                               \
				(arr ## _alloc) = newsize_;                                 \
				ret_ = 0;                                                   \
			}                                                                   \
		}                                                                           \
	}                                                                                   \
	ret_;                                                                               \
})

static int insert_spec(const struct selabel_handle *rec, struct saved_data *data,
		       const char *prefix, char *regex, uint8_t file_kind, char *context,
		       const char *path, uint8_t inputno, uint32_t lineno)
{
	size_t prefix_len;
	bool has_meta;

	if (data->num_specs == UINT64_MAX) {
		free(regex);
		free(context);
		errno = EOVERFLOW;
		return -1;
	}

	has_meta = regex_has_meta_chars(regex, &prefix_len, path, lineno);

	/* Ensured by read_spec_entry() */
	assert(prefix_len < UINT16_MAX);

	if (has_meta) {
		struct spec_node *node = data->root;
		const char *p = regex;
		uint32_t id;
		int depth = 0, rc;

		while (depth < SPEC_NODE_MAX_DEPTH) {
			const char *q;
			size_t regex_stem_len, stem_len;
			char *stem = NULL;
			bool child_found;

			q = strchr(p + 1, '/');
			if (!q)
				break;

			regex_stem_len = q - p - 1;
			/* Double slashes */
			if (regex_stem_len == 0) {
				p = q;
				continue;
			}

			rc = regex_simplify(p + 1, regex_stem_len, &stem, path, lineno);
			if (rc < 0) {
				free(regex);
				free(context);
				return -1;
			}
			if (rc == 0)
				break;

			stem_len = strlen(stem);
			if (stem_len >= UINT16_MAX) {
				free(stem);
				break;
			}

			if (depth == 0 && prefix && strcmp(prefix + 1, stem) != 0) {
				free(stem);
				free(regex);
				free(context);
				return 0;
			}

			child_found = false;
			for (uint32_t i = 0; i < node->children_num; i++) {
				if (node->children[i].stem_len == stem_len && strncmp(node->children[i].stem, stem, stem_len) == 0) {
					child_found = true;
					node = &node->children[i];
					break;
				}
			}

			if (!child_found) {
				rc = GROW_ARRAY(node->children);
				if (rc) {
					free(stem);
					free(regex);
					free(context);
					return -1;
				}

				id = node->children_num++;
				node->children[id] = (struct spec_node) {
					.stem = stem,
					.stem_len = stem_len,
				};

				node = &node->children[id];
			} else {
				free(stem);
			}

			p += regex_stem_len + 1;
			depth++;
		}

		rc = GROW_ARRAY(node->regex_specs);
		if (rc) {
			free(regex);
			free(context);
			return -1;
		}

		id = node->regex_specs_num++;

		node->regex_specs[id] = (struct regex_spec) {
			.regex_str = regex,
			.prefix_len = prefix_len,
			.regex_compiled = false,
			.regex_lock = PTHREAD_MUTEX_INITIALIZER,
			.file_kind = file_kind,
			.any_matches = false,
			.inputno = inputno,
			.lineno = lineno,
			.lr.ctx_raw = context,
			.lr.ctx_trans = NULL,
			.lr.lineno = lineno,
			.lr.validated = false,
			.lr.lock = PTHREAD_MUTEX_INITIALIZER,
		};

		data->num_specs++;

		if (rec->validating) {
			char errbuf[256];

			if (compile_regex(&node->regex_specs[id], errbuf, sizeof(errbuf))) {
				COMPAT_LOG(SELINUX_ERROR,
					   "%s:  line %u has invalid regex %s:  %s\n",
					   path, lineno, regex, errbuf);
				return -1;
			}

			if (strcmp(context, "<<none>>") != 0) {
				rc = compat_validate(rec, &node->regex_specs[id].lr, path, lineno);
				if (rc < 0)
					return rc;
			}
		}
	} else { /* !has_meta */
		struct spec_node *node = data->root;
		char *literal_regex = NULL;
		const char *p;
		uint32_t id;
		int depth = 0, rc;

		rc = regex_simplify(regex, strlen(regex), &literal_regex, path, lineno);
		if (rc != 1) {
			if (rc == 0) {
				COMPAT_LOG(SELINUX_ERROR,
					   "%s:  line %u failed to simplify regex %s\n",
					   path, lineno, regex);
				errno = EINVAL;
			}
			free(regex);
			free(context);
			return -1;
		}

		p = literal_regex;

		while (depth < SPEC_NODE_MAX_DEPTH) {
			const char *q;
			size_t length;
			char *stem;
			bool child_found;

			if (*p != '/')
				break;

			q = strchr(p + 1, '/');
			if (!q)
				break;

			length = q - p - 1;
			/* Double slashes */
			if (length == 0) {
				p = q;
				continue;
			}

			/* Ensured by read_spec_entry() */
			assert(length < UINT16_MAX);

			if (depth == 0 && prefix && strncmp(prefix + 1, p + 1, length) != 0) {
				free(literal_regex);
				free(regex);
				free(context);
				return 0;
			}

			child_found = false;
			for (uint32_t i = 0; i < node->children_num; i++) {
				if (node->children[i].stem_len == length && strncmp(node->children[i].stem, p + 1, length) == 0) {
					child_found = true;
					node = &node->children[i];
					break;
				}
			}

			if (!child_found) {
				rc = GROW_ARRAY(node->children);
				if (rc) {
					free(literal_regex);
					free(regex);
					free(context);
					return -1;
				}

				stem = strndup(p + 1, length);
				if (!stem) {
					free(literal_regex);
					free(regex);
					free(context);
					return -1;
				}

				id = node->children_num++;
				node->children[id] = (struct spec_node) {
					.stem = stem,
					.stem_len = length,
				};

				node = &node->children[id];
			}

			p = q;
			depth++;
		}

		rc = GROW_ARRAY(node->literal_specs);
		if (rc) {
			free(literal_regex);
			free(regex);
			free(context);
			return -1;
		}

		id = node->literal_specs_num++;

		assert(prefix_len == strlen(literal_regex));

		node->literal_specs[id] = (struct literal_spec) {
			.regex_str = regex,
			.prefix_len = prefix_len,
			.literal_match = literal_regex,
			.file_kind = file_kind,
			.any_matches = false,
			.lr.ctx_raw = context,
			.lr.ctx_trans = NULL,
			.lr.lineno = lineno,
			.lr.validated = false,
			.lr.lock = PTHREAD_MUTEX_INITIALIZER,
		};

		data->num_specs++;

		if (rec->validating && strcmp(context, "<<none>>") != 0) {
			rc = compat_validate(rec, &node->literal_specs[id].lr, path, lineno);
			if (rc < 0)
				return rc;
		}

	}

	return 0;
}

/* This will always check for buffer over-runs and either read the next entry
 * if buf != NULL or skip over the entry (as these areas are mapped in the
 * current buffer). */
static inline int next_entry(void *buf, struct mmap_area *fp, size_t bytes)
{
	if (bytes > fp->next_len)
		return -1;

	if (buf)
		memcpy(buf, fp->next_addr, bytes);

	fp->next_addr = (unsigned char *)fp->next_addr + bytes;
	fp->next_len -= bytes;
	return 0;
}

/* This service is used by label_file.c process_file() and
 * utils/sefcontext_compile.c */
static inline int process_line(struct selabel_handle *rec,
			       const char *path, const char *prefix,
			       char *line_buf, size_t nread,
			       uint8_t inputno, uint32_t lineno)
{
	int items;
	char *regex = NULL, *type = NULL, *context = NULL;
	struct saved_data *data = rec->data;
	const char *errbuf = NULL;
	uint8_t file_kind = LABEL_FILE_KIND_ALL;

	if (prefix) {
		if (prefix[0] != '/' ||
		    prefix[1] == '\0' ||
		    strchr(prefix + 1, '/') != NULL) {
			errno = EINVAL;
			return -1;
		}
	}

	items = read_spec_entries(line_buf, nread, &errbuf, 3, &regex, &type, &context);
	if (items < 0) {
		if (errbuf) {
			COMPAT_LOG(SELINUX_ERROR,
				   "%s:  line %u error due to: %s\n", path,
				   lineno, errbuf);
		} else {
			COMPAT_LOG(SELINUX_ERROR,
				   "%s:  line %u error due to: %m\n", path,
				   lineno);
		}
		free(regex);
		free(type);
		free(context);
		return -1;
	}

	if (items == 0)
		return items;

	if (items < 2) {
		COMPAT_LOG(SELINUX_ERROR,
			   "%s:  line %u is missing fields\n", path,
			   lineno);
		if (items == 1)
			free(regex);
		errno = EINVAL;
		return -1;
	}

	if (items == 2) {
		/* The type field is optional. */
		context = type;
		type = NULL;
	}

	if (type) {
		file_kind = string_to_file_kind(type);

		if (file_kind == LABEL_FILE_KIND_INVALID) {
			COMPAT_LOG(SELINUX_ERROR,
				   "%s:  line %u has invalid file type %s\n",
				   path, lineno, type);
			free(regex);
			free(type);
			free(context);
			errno = EINVAL;
			return -1;
		}

		free(type);
	}

	return insert_spec(rec, data, prefix, regex, file_kind, context, path, inputno, lineno);
}

#endif /* _SELABEL_FILE_H_ */
