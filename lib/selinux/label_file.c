/*
 * File contexts backend for labeling system
 *
 * Author : Eamon Walsh <ewalsh@tycho.nsa.gov>
 * Author : Stephen Smalley <stephen.smalley.work@gmail.com>
 * Author : Christian Göttsche <cgzones@googlemail.com>
 */

#include <assert.h>
#include <endian.h>
#include <fcntl.h>
#include <stdarg.h>
#include <string.h>
#include <stdio.h>
#include <ctype.h>
#include <errno.h>
#include <limits.h>
#include <stdint.h>
#include <unistd.h>
#include <sys/mman.h>
#include <sys/types.h>
#include <sys/stat.h>

#include "callbacks.h"
#include "label_internal.h"
#include "selinux_internal.h"
#include "label_file.h"


#ifdef FUZZING_BUILD_MODE_UNSAFE_FOR_PRODUCTION
# define FUZZ_EXTERN
#else
# define FUZZ_EXTERN static
#endif  /* FUZZING_BUILD_MODE_UNSAFE_FOR_PRODUCTION */


void free_spec_node(struct spec_node *node)
{
	for (uint32_t i = 0; i < node->literal_specs_num; i++) {
		struct literal_spec *lspec = &node->literal_specs[i];

		free(lspec->lr.ctx_raw);
		free(lspec->lr.ctx_trans);
		__pthread_mutex_destroy(&lspec->lr.lock);

		if (lspec->from_mmap)
			continue;

		free(lspec->literal_match);
		free(lspec->regex_str);
	}
	free(node->literal_specs);

	for (uint32_t i = 0; i < node->regex_specs_num; i++) {
		struct regex_spec *rspec = &node->regex_specs[i];

		free(rspec->lr.ctx_raw);
		free(rspec->lr.ctx_trans);
		__pthread_mutex_destroy(&rspec->lr.lock);
		regex_data_free(rspec->regex);
		__pthread_mutex_destroy(&rspec->regex_lock);

		if (rspec->from_mmap)
			continue;

		free(rspec->regex_str);
	}
	free(node->regex_specs);

	for (uint32_t i = 0; i < node->children_num; i++)
		free_spec_node(&node->children[i]);
	free(node->children);

	if (!node->from_mmap)
		free(node->stem);
}

void sort_spec_node(struct spec_node *node, struct spec_node *parent)
{
	/* A node should not be its own parent */
	assert(node != parent);
	/* Only root node has NULL stem */
	assert((!parent && !node->stem) || (parent && node->stem && node->stem[0] != '\0'));
	/* A non-root node should not be empty */
	assert(!parent || (node->literal_specs_num || node->regex_specs_num || node->children_num));


	node->parent = parent;

	/*
	 * Sort for comparison support and binary search lookup,
	 * except for regex specs which are matched in reverse input order.
	 */

	if (node->literal_specs_num > 1)
		qsort(node->literal_specs, node->literal_specs_num, sizeof(struct literal_spec), compare_literal_spec);

	if (node->children_num > 1)
		qsort(node->children, node->children_num, sizeof(struct spec_node), compare_spec_node);

	for (uint32_t i = 0; i < node->children_num; i++)
		sort_spec_node(&node->children[i], node);
}

/*
 * Warn about duplicate specifications.
 */
static int nodups_spec_node(const struct spec_node *node, const char *path)
{
	int rc = 0;

	if (node->literal_specs_num > 1) {
		for (uint32_t i = 0; i < node->literal_specs_num - 1; i++) {
			const struct literal_spec *node1 = &node->literal_specs[i];
			const struct literal_spec *node2 = &node->literal_specs[i+1];

			if (strcmp(node1->literal_match, node2->literal_match) != 0)
				continue;

			if (node1->file_kind != LABEL_FILE_KIND_ALL && node2->file_kind != LABEL_FILE_KIND_ALL && node1->file_kind != node2->file_kind)
				continue;

			rc = -1;
			errno = EINVAL;
			if (strcmp(node1->lr.ctx_raw, node2->lr.ctx_raw) != 0) {
				COMPAT_LOG
					(SELINUX_ERROR,
						"%s: Multiple different specifications for %s %s  (%s and %s).\n",
						path,
						file_kind_to_string(node1->file_kind),
						node1->literal_match,
						node1->lr.ctx_raw,
						node2->lr.ctx_raw);
			} else {
				COMPAT_LOG
					(SELINUX_ERROR,
						"%s: Multiple same specifications for %s %s.\n",
						path,
						file_kind_to_string(node1->file_kind),
						node1->literal_match);
			}
		}
	}

	if (node->regex_specs_num > 1) {
		for (uint32_t i = 0; i < node->regex_specs_num - 1; i++) {
			for (uint32_t j = i; j < node->regex_specs_num - 1; j++) {
				const struct regex_spec *node1 = &node->regex_specs[i];
				const struct regex_spec *node2 = &node->regex_specs[j + 1];

				if (node1->prefix_len != node2->prefix_len)
					continue;

				if (strcmp(node1->regex_str, node2->regex_str) != 0)
					continue;

				if (node1->file_kind != LABEL_FILE_KIND_ALL && node2->file_kind != LABEL_FILE_KIND_ALL && node1->file_kind != node2->file_kind)
					continue;

				rc = -1;
				errno = EINVAL;
				if (strcmp(node1->lr.ctx_raw, node2->lr.ctx_raw) != 0) {
					COMPAT_LOG
						(SELINUX_ERROR,
							"%s: Multiple different specifications for %s %s  (%s and %s).\n",
							path,
							file_kind_to_string(node1->file_kind),
							node1->regex_str,
							node1->lr.ctx_raw,
							node2->lr.ctx_raw);
				} else {
					COMPAT_LOG
						(SELINUX_ERROR,
							"%s: Multiple same specifications for %s %s.\n",
							path,
							file_kind_to_string(node1->file_kind),
							node1->regex_str);
				}
			}
		}
	}

	for (uint32_t i = 0; i < node->children_num; i++) {
		int rc2;

		rc2 = nodups_spec_node(&node->children[i], path);
		if (rc2)
			rc = rc2;
	}

	return rc;
}

FUZZ_EXTERN int process_text_file(FILE *fp, const char *prefix,
				  struct selabel_handle *rec, const char *path,
				  uint8_t inputno)
{
	int rc;
	size_t line_len;
	ssize_t nread;
	unsigned int lineno = 0;
	char *line_buf = NULL;

	while ((nread = getline(&line_buf, &line_len, fp)) > 0) {
		rc = process_line(rec, path, prefix, line_buf, nread, inputno, ++lineno);
		if (rc)
			goto out;
	}
	rc = 0;
out:
	free(line_buf);
	return rc;
}

static int merge_mmap_spec_nodes(struct spec_node *restrict dest, struct spec_node *restrict source)
{
	/* Nodes should have the same stem */
	assert((dest->stem == NULL && source->stem == NULL) ||
	       (dest->stem && source->stem && dest->stem_len && source->stem_len && strcmp(dest->stem, source->stem) == 0));
	/* Source should be loaded from mmap, so we can assume its data is sorted */
	assert(source->from_mmap);


	/*
	 * Merge literal specs
	 */
	if (source->literal_specs_num > 0) {
		if (dest->literal_specs_num > 0) {
			struct literal_spec *lspecs;
			uint32_t lspecs_num;

			if (__builtin_add_overflow(dest->literal_specs_num, source->literal_specs_num, &lspecs_num))
				return -1;

			lspecs = reallocarray(dest->literal_specs, lspecs_num, sizeof(struct literal_spec));
			if (!lspecs)
				return -1;

			memcpy(&lspecs[dest->literal_specs_num], source->literal_specs, source->literal_specs_num * sizeof(struct literal_spec));

			dest->literal_specs = lspecs;
			dest->literal_specs_num = lspecs_num;
			dest->literal_specs_alloc = lspecs_num;

			/* Cleanup moved source */
			for (uint32_t i = 0; i < source->literal_specs_num; i++) {
				source->literal_specs[i].lr.ctx_raw = NULL;
				source->literal_specs[i].lr.ctx_trans = NULL;
			}

		} else {
			assert(dest->literal_specs == NULL);
			dest->literal_specs       = source->literal_specs;
			dest->literal_specs_num   = source->literal_specs_num;
			dest->literal_specs_alloc = source->literal_specs_alloc;
			source->literal_specs       = NULL;
			source->literal_specs_num   = 0;
			source->literal_specs_alloc = 0;
		}
	}


	/*
	 * Merge regex specs
	 */
	if (source->regex_specs_num > 0) {
		if (dest->regex_specs_num > 0) {
			struct regex_spec *rspecs;
			uint32_t rspecs_num;

			if (__builtin_add_overflow(dest->regex_specs_num, source->regex_specs_num, &rspecs_num))
					return -1;

			rspecs = reallocarray(dest->regex_specs, rspecs_num, sizeof(struct regex_spec));
			if (!rspecs)
				return -1;

			memcpy(&rspecs[dest->regex_specs_num], source->regex_specs, source->regex_specs_num * sizeof(struct regex_spec));

			dest->regex_specs = rspecs;
			dest->regex_specs_num = rspecs_num;
			dest->regex_specs_alloc = rspecs_num;

			/* Cleanup moved source */
			for (uint32_t i = 0; i < source->regex_specs_num; i++) {
				source->regex_specs[i].lr.ctx_raw = NULL;
				source->regex_specs[i].lr.ctx_trans = NULL;
				source->regex_specs[i].regex = NULL;
				source->regex_specs[i].regex_compiled = false;
			}
		} else {
			assert(dest->regex_specs == NULL);
			dest->regex_specs       = source->regex_specs;
			dest->regex_specs_num   = source->regex_specs_num;
			dest->regex_specs_alloc = source->regex_specs_alloc;
			source->regex_specs       = NULL;
			source->regex_specs_num   = 0;
			source->regex_specs_alloc = 0;
		}
	}


	/*
	 * Merge child nodes
	 */
	if (source->children_num > 0) {
		if (dest->children_num > 0) {
			struct spec_node *new_children;
			uint32_t iter_dest, iter_source, new_children_alloc, new_children_num, remaining_dest, remaining_source;

			if (__builtin_add_overflow(dest->children_num, source->children_num, &new_children_alloc))
				return -1;

			/* Over-allocate in favor of re-allocating multiple times */
			new_children = calloc(new_children_alloc, sizeof(struct spec_node));
			if (!new_children)
				return -1;

			/* Since source is loaded from mmap its child nodes are sorted */
			qsort(dest->children, dest->children_num, sizeof(struct spec_node), compare_spec_node);

			for (iter_dest = 0, iter_source = 0, new_children_num = 0; iter_dest < dest->children_num && iter_source < source->children_num;) {
				struct spec_node *child_dest = &dest->children[iter_dest];
				struct spec_node *child_source = &source->children[iter_source];
				int r;

				r = strcmp(child_dest->stem, child_source->stem);
				if (r == 0) {
					int rc;

					rc = merge_mmap_spec_nodes(child_dest, child_source);
					if (rc) {
						free(new_children);
						return rc;
					}

					new_children[new_children_num++] = *child_dest;
					free_spec_node(child_source);
					iter_dest++;
					iter_source++;
				} else if (r < 0) {
					new_children[new_children_num++] = *child_dest;
					iter_dest++;
				} else {
					new_children[new_children_num++] = *child_source;
					iter_source++;
				}
			}

			remaining_dest = dest->children_num - iter_dest;
			remaining_source = source->children_num - iter_source;
			assert(!remaining_dest || !remaining_source);
			assert(new_children_num + remaining_dest + remaining_source <= new_children_alloc);

			if (remaining_dest > 0) {
				memcpy(&new_children[new_children_num], &dest->children[iter_dest], remaining_dest * sizeof(struct spec_node));
				new_children_num += remaining_dest;
			}

			if (remaining_source > 0) {
				memcpy(&new_children[new_children_num], &source->children[iter_source], remaining_source * sizeof(struct spec_node));
				new_children_num += remaining_source;
			}

			free(dest->children);
			dest->children       = new_children;
			dest->children_alloc = new_children_alloc;
			dest->children_num   = new_children_num;

			free(source->children);
			source->children       = NULL;
			source->children_alloc = 0;
			source->children_num   = 0;

		} else {
			assert(dest->children == NULL);
			dest->children       = source->children;
			dest->children_num   = source->children_num;
			dest->children_alloc = source->children_alloc;
			source->children = NULL;
			source->children_num = 0;
			source->children_alloc = 0;
		}
	}

	return 0;
}

static inline bool entry_size_check(const struct mmap_area *mmap_area, size_t nmenb, size_t size)
{
	size_t required;

	if (__builtin_mul_overflow(nmenb, size, &required))
		return true;

	return required > mmap_area->next_len;
}

struct context_array {
	char **data;
	uint32_t size;
};

static void free_context_array(struct context_array *ctx_array)
{
	if (!ctx_array->data)
		return;

	for (uint32_t i = 0; i < ctx_array->size; i++)
		free(ctx_array->data[i]);

	free(ctx_array->data);
}

static int load_mmap_ctxarray(struct mmap_area *mmap_area, const char *path, struct context_array *ctx_array, bool validating)
{
	uint32_t data_u32, count;
	uint16_t data_u16, ctx_len;
	char *ctx;
	int rc;

	/*
	 * Read number of context definitions
	 */
	rc = next_entry(&data_u32, mmap_area, sizeof(uint32_t));
	if (rc < 0)
		return -1;
	count = be32toh(data_u32);

	if (entry_size_check(mmap_area, count, 3 * sizeof(char)))
			return -1;

	(*ctx_array).data = calloc(count, sizeof(char *));
	if (!(*ctx_array).data) {
		(*ctx_array).size = 0;
		return -1;
	}
	(*ctx_array).size = count;

	for (uint32_t i = 0; i < count; i++) {
		/*
		 * Read raw context
		 * We need to allocate it on the heap since it might get free'd and replaced in a
		 * SELINUX_CB_VALIDATE callback.
		 */
		rc = next_entry(&data_u16, mmap_area, sizeof(uint16_t));
		if (rc < 0)
			return -1;
		ctx_len = be16toh(data_u16);

		if (ctx_len == 0 || ctx_len == UINT16_MAX)
			return -1;

		if (entry_size_check(mmap_area, ctx_len, sizeof(char)))
			return -1;

		ctx = malloc(ctx_len + 1);
		if (!ctx)
			return -1;

		rc = next_entry(ctx, mmap_area, ctx_len);
		if (rc < 0) {
			free(ctx);
			return -1;
		}
		ctx[ctx_len] = '\0';

		if (validating && strcmp(ctx, "<<none>>") != 0) {
			if (selinux_validate(&ctx) < 0) {
				selinux_log(SELINUX_ERROR, "%s: context %s is invalid\n",
					    path, ctx);
				free(ctx);
				return -1;
			}
		}

		(*ctx_array).data[i] = ctx;
	}

	return 0;
}

static int load_mmap_literal_spec(struct mmap_area *mmap_area, bool validating,
				  struct literal_spec *lspec, const struct context_array *ctx_array)
{
	uint32_t data_u32, ctx_id;
	uint16_t data_u16, regex_len, lmatch_len;
	uint8_t data_u8;
	int rc;

	lspec->from_mmap = true;


	/*
	 * Read raw context id
	 * We need to allocate it on the heap since it might get free'd and replaced in a
	 * SELINUX_CB_VALIDATE callback.
	 */
	rc = next_entry(&data_u32, mmap_area, sizeof(uint32_t));
	if (rc < 0)
		return -1;
	ctx_id = be32toh(data_u32);

	if (ctx_id == 0 || ctx_id == UINT32_MAX || ctx_id > ctx_array->size)
		return -1;

	lspec->lr.ctx_raw = strdup(ctx_array->data[ctx_id - 1]);
	if (!lspec->lr.ctx_raw)
		return -1;

	if (validating)
		/* validated in load_mmap_ctxarray() */
		lspec->lr.validated = true;


	/*
	 * Read original regex
	 */
	rc = next_entry(&data_u16, mmap_area, sizeof(uint16_t));
	if (rc < 0)
		return -1;
	regex_len = be16toh(data_u16);

	if (regex_len <= 1)
		return -1;

	lspec->regex_str = mmap_area->next_addr;
	rc = next_entry(NULL, mmap_area, regex_len);
	if (rc < 0)
		return -1;

	if (lspec->regex_str[0] == '\0' || lspec->regex_str[regex_len - 1] != '\0')
		return -1;


	/*
	 * Read literal match
	 */
	rc = next_entry(&data_u16, mmap_area, sizeof(uint16_t));
	if (rc < 0)
		return -1;
	lmatch_len = be16toh(data_u16);

	if (lmatch_len <= 1)
		return -1;

	lspec->literal_match = mmap_area->next_addr;
	rc = next_entry(NULL, mmap_area, lmatch_len);
	if (rc < 0)
		return -1;

	if (lspec->literal_match[0] == '\0' || lspec->literal_match[lmatch_len - 1] != '\0')
		return -1;

	lspec->prefix_len = lmatch_len - 1;

	if (lspec->prefix_len > strlen(lspec->regex_str))
		return -1;


	/*
	 * Read file kind
	 */
	rc = next_entry(&data_u8, mmap_area, sizeof(uint8_t));
	if (rc < 0)
		return -1;
	lspec->file_kind = data_u8;


	return 0;
}

static int load_mmap_regex_spec(struct mmap_area *mmap_area, bool validating, bool do_load_precompregex,
				uint8_t inputno,
				struct regex_spec *rspec, const struct context_array *ctx_array)
{
	uint32_t data_u32, ctx_id, lineno;
	uint16_t data_u16, regex_len;
	uint8_t data_u8;
	int rc;

	rspec->from_mmap = true;


	/*
	 * Read raw context id
	 * We need to allocate it on the heap since it might get free'd and replaced in a
	 * SELINUX_CB_VALIDATE callback.
	 */
	rc = next_entry(&data_u32, mmap_area, sizeof(uint32_t));
	if (rc < 0)
		return -1;
	ctx_id = be32toh(data_u32);

	if (ctx_id == 0 || ctx_id == UINT32_MAX || ctx_id > ctx_array->size)
		return -1;

	rspec->lr.ctx_raw = strdup(ctx_array->data[ctx_id - 1]);
	if (!rspec->lr.ctx_raw)
		return -1;

	if (validating)
		/* validated in load_mmap_ctxarray() */
		rspec->lr.validated = true;


	/*
	 * Read line number in source file.
	 */
	rc = next_entry(&data_u32, mmap_area, sizeof(uint32_t));
	if (rc < 0)
		return -1;
	lineno = be32toh(data_u32);

	if (lineno == 0 || lineno == UINT32_MAX)
		return -1;
	rspec->lineno = lineno;
	rspec->inputno = inputno;


	/*
	 * Read original regex
	 */
	rc = next_entry(&data_u16, mmap_area, sizeof(uint16_t));
	if (rc < 0)
		return -1;
	regex_len = be16toh(data_u16);

	if (regex_len <= 1)
		return -1;

	rspec->regex_str = mmap_area->next_addr;
	rc = next_entry(NULL, mmap_area, regex_len);
	if (rc < 0)
		return -1;

	if (rspec->regex_str[0] == '\0' || rspec->regex_str[regex_len - 1] != '\0')
		return -1;


	/*
	 * Read prefix length
	 */
	rc = next_entry(&data_u16, mmap_area, sizeof(uint16_t));
	if (rc < 0)
		return -1;
	rspec->prefix_len = be16toh(data_u16);

	if (rspec->prefix_len > strlen(rspec->regex_str))
		return -1;


	/*
	 * Read file kind
	 */
	rc = next_entry(&data_u8, mmap_area, sizeof(uint8_t));
	if (rc < 0)
		return -1;
	rspec->file_kind = data_u8;


	/*
	 * Read pcre regex related data
	 */
	rc = regex_load_mmap(mmap_area, &rspec->regex, do_load_precompregex,
			     &rspec->regex_compiled);
	if (rc < 0)
		return -1;


	__pthread_mutex_init(&rspec->regex_lock, NULL);

	return 0;
}

static int load_mmap_spec_node(struct mmap_area *mmap_area, const char *path, bool validating, bool do_load_precompregex,
			       struct spec_node *node, const unsigned depth, uint8_t inputno, const struct context_array *ctx_array)
{
	uint32_t data_u32, lspec_num, rspec_num, children_num;
	uint16_t data_u16, stem_len;
	const bool is_root = (depth == 0);
	int rc;

	/*
	 * Guard against deep recursion by malicious pre-compiled fcontext
	 * definitions. The limit of 32 is chosen intuitively and should
	 * suffice for any real world scenario. See the macro
	 * SPEC_NODE_MAX_DEPTH for the current value used for tree building.
	 */
	if (depth >= 32)
		return -1;

	node->from_mmap = true;


	/*
	 * Read stem
	 */
	rc = next_entry(&data_u16, mmap_area, sizeof(uint16_t));
	if (rc < 0)
		return -1;
	stem_len = be16toh(data_u16);

	if (stem_len == 0)
		return -1;

	if ((stem_len == 1) != is_root)
		return -1;

	node->stem_len = stem_len - 1;
	node->stem = mmap_area->next_addr;
	rc = next_entry(NULL, mmap_area, stem_len);
	if (rc < 0)
		return -1;

	if (is_root)
		node->stem = NULL;
	else if (node->stem[0] == '\0' || node->stem[stem_len - 1] != '\0')
		return -1;


	/*
	 * Read literal specs
	 */
	rc = next_entry(&data_u32, mmap_area, sizeof(uint32_t));
	if (rc < 0)
		return -1;
	lspec_num = be32toh(data_u32);

	if (lspec_num == UINT32_MAX)
		return -1;

	if (lspec_num > 0) {
		if (entry_size_check(mmap_area,  lspec_num, 3 * sizeof(uint16_t) + sizeof(uint32_t) + 6 * sizeof(char)))
			return -1;

		node->literal_specs = calloc(lspec_num, sizeof(struct literal_spec));
		if (!node->literal_specs)
			return -1;

		node->literal_specs_num   = lspec_num;
		node->literal_specs_alloc = lspec_num;

		for (uint32_t i = 0; i < lspec_num; i++) {
			rc = load_mmap_literal_spec(mmap_area, validating, &node->literal_specs[i], ctx_array);
			if (rc)
				return -1;
		}
	}


	/*
	 * Read regex specs
	 */
	rc = next_entry(&data_u32, mmap_area, sizeof(uint32_t));
	if (rc < 0)
		return -1;
	rspec_num = be32toh(data_u32);

	if (rspec_num == UINT32_MAX)
		return -1;

	if (rspec_num > 0) {
		if (entry_size_check(mmap_area, rspec_num, sizeof(uint32_t) + 3 * sizeof(uint16_t) + 4 * sizeof(char)))
			return -1;

		node->regex_specs = calloc(rspec_num, sizeof(struct regex_spec));
		if (!node->regex_specs)
			return -1;

		node->regex_specs_num   = rspec_num;
		node->regex_specs_alloc = rspec_num;

		for (uint32_t i = 0; i < rspec_num; i++) {
			rc = load_mmap_regex_spec(mmap_area, validating, do_load_precompregex, inputno, &node->regex_specs[i], ctx_array);
			if (rc)
				return -1;
		}
	}


	/*
	 * Read child nodes
	 */
	rc = next_entry(&data_u32, mmap_area, sizeof(uint32_t));
	if (rc < 0)
		return -1;
	children_num = be32toh(data_u32);

	if (children_num == UINT32_MAX)
		return -1;

	if (children_num > 0) {
		const char *prev_stem = NULL;

		if (entry_size_check(mmap_area, children_num, 3 * sizeof(uint32_t) + sizeof(uint16_t)))
			return -1;

		node->children = calloc(children_num, sizeof(struct spec_node));
		if (!node->children)
			return -1;

		node->children_num   = children_num;
		node->children_alloc = children_num;

		for (uint32_t i = 0; i < children_num; i++) {
			rc = load_mmap_spec_node(mmap_area, path, validating, do_load_precompregex, &node->children[i], depth + 1, inputno, ctx_array);
			if (rc)
				return -1;

			/* Ensure child nodes are sorted and distinct */
			if (prev_stem && strcmp(prev_stem, node->children[i].stem) >= 0)
				return -1;

			prev_stem = node->children[i].stem;
		}
	}


	if (!is_root && lspec_num == 0 && rspec_num == 0 && children_num == 0)
		return -1;

	return 0;
}

FUZZ_EXTERN int load_mmap(FILE *fp, const size_t len, struct selabel_handle *rec,
			  const char *path, uint8_t inputno)
{
	struct saved_data *data = rec->data;
	struct spec_node *root = NULL;
	struct context_array ctx_array = {};
	int rc;
	char *addr = NULL, *str_buf = NULL;
	struct mmap_area *mmap_area = NULL;
	uint64_t data_u64, num_specs;
	uint32_t data_u32, pcre_ver_len, pcre_arch_len;
	const char *reg_arch, *reg_version;
	bool reg_version_matches = false, reg_arch_matches = false;

	mmap_area = malloc(sizeof(*mmap_area));
	if (!mmap_area)
		goto err;

	addr = mmap(NULL, len, PROT_READ, MAP_PRIVATE, fileno(fp), 0);
	if (addr == MAP_FAILED)
		goto err;

	rc = madvise(addr, len, MADV_WILLNEED);
	if (rc == -1)
		COMPAT_LOG(SELINUX_INFO, "%s:  Failed to advise memory mapping:  %m\n",
			   path);

	/* save where we mmap'd the file to cleanup on close() */
	*mmap_area = (struct mmap_area) {
		.addr      = addr,
		.next_addr = addr,
		.next      = NULL,
		.next_len  = len,
		.len       = len,
	};

	/* check if this looks like an fcontext file */
	rc = next_entry(&data_u32, mmap_area, sizeof(uint32_t));
	if (rc < 0 || be32toh(data_u32) != SELINUX_MAGIC_COMPILED_FCONTEXT)
		goto err;

	/* check if this version is higher than we understand */
	rc = next_entry(&data_u32, mmap_area, sizeof(uint32_t));
	if (rc < 0 || be32toh(data_u32) != SELINUX_COMPILED_FCONTEXT_TREE_LAYOUT) {
		COMPAT_LOG(SELINUX_WARNING,
				"%s:  Unsupported compiled fcontext version %d, supported is version %d\n",
				path, be32toh(data_u32), SELINUX_COMPILED_FCONTEXT_TREE_LAYOUT);
		goto err;
	}

	reg_version = regex_version();
	if (!reg_version)
		goto err;

	reg_arch = regex_arch_string();
	if (!reg_arch)
		goto err;

	rc = next_entry(&data_u32, mmap_area, sizeof(uint32_t));
	if (rc < 0)
		goto err;
	pcre_ver_len = be32toh(data_u32);

	/* Check version lengths */
	if (strlen(reg_version) != pcre_ver_len) {
		/*
		 * Skip the entry and conclude that we have
		 * a mismatch, which is not fatal.
		 */
		next_entry(NULL, mmap_area, pcre_ver_len);
		goto end_version_check;
	}

	if (entry_size_check(mmap_area, pcre_ver_len, sizeof(char)))
		goto err;

	str_buf = malloc(pcre_ver_len + 1);
	if (!str_buf)
		goto err;

	rc = next_entry(str_buf, mmap_area, pcre_ver_len);
	if (rc < 0)
		goto err;

	str_buf[pcre_ver_len] = '\0';

	/* Check for regex version mismatch */
	if (strcmp(str_buf, reg_version) != 0)
		COMPAT_LOG(SELINUX_WARNING,
			"%s:  Regex version mismatch, expected: %s actual: %s\n",
			path, reg_version, str_buf);
	else
		reg_version_matches = true;

	free(str_buf);
	str_buf = NULL;

end_version_check:

	rc = next_entry(&data_u32, mmap_area, sizeof(uint32_t));
	if (rc < 0)
		goto err;
	pcre_arch_len = be32toh(data_u32);

	/* Check arch string lengths */
	if (strlen(reg_arch) != pcre_arch_len) {
		/*
		 * Skip the entry and conclude that we have
		 * a mismatch, which is not fatal.
		 */
		next_entry(NULL, mmap_area, pcre_arch_len);
		goto end_arch_check;
	}

	if (entry_size_check(mmap_area, pcre_arch_len, sizeof(char)))
		goto err;

	str_buf = malloc(pcre_arch_len + 1);
	if (!str_buf)
		goto err;

	rc = next_entry(str_buf, mmap_area, pcre_arch_len);
	if (rc < 0)
		goto err;

	str_buf[pcre_arch_len] = '\0';

	/* Check if arch string mismatch */
	if (strcmp(str_buf, reg_arch) != 0)
		COMPAT_LOG(SELINUX_WARNING,
			"%s:  Regex architecture mismatch, expected: %s actual: %s\n",
			path, reg_arch, str_buf);
	else
		reg_arch_matches = true;

	free(str_buf);
	str_buf = NULL;

end_arch_check:

	/* Read number of total specifications */
	rc = next_entry(&data_u64, mmap_area, sizeof(uint64_t));
	if (rc < 0)
		goto err;
	num_specs = be64toh(data_u64);

	rc = load_mmap_ctxarray(mmap_area, path, &ctx_array, rec->validating);
	if (rc)
		goto err;

	root = calloc(1, sizeof(*root));
	if (!root)
		goto err;

	rc = load_mmap_spec_node(mmap_area, path, rec->validating,
				 reg_version_matches && reg_arch_matches,
				 root, 0,
				 inputno,
				 &ctx_array);
	if (rc)
		goto err;

	/*
	 * On intermediate failure some data might already have been merged, so always keep the mmap'ed memory.
	 */
	mmap_area->next = data->mmap_areas;
	data->mmap_areas = mmap_area;
	mmap_area = NULL;

	if (data->num_specs == 0) {
		free_spec_node(data->root);
		free(data->root);
		data->root = root;
		root = NULL;
	} else {
		rc = merge_mmap_spec_nodes(data->root, root);
		if (rc)
			goto err;

		free_spec_node(root);
		free(root);
		root = NULL;
	}

	/* Success */
	data->num_specs += num_specs;

	free_context_array(&ctx_array);

	return 0;

err:
	free_context_array(&ctx_array);
	if (root) {
		free_spec_node(root);
		free(root);
	}
	free(str_buf);
	free(mmap_area);
	if (addr && addr != MAP_FAILED)
		munmap(addr, len);
	if (errno == 0)
		errno = EINVAL;
	return -1;
}

struct file_details {
	const char *suffix;
	struct stat sb;
};

static char *rolling_append(char *current, const char *suffix, size_t max)
{
	size_t size;
	size_t suffix_size;
	size_t current_size;

	if (!suffix)
		return current;

	current_size = strlen(current);
	suffix_size = strlen(suffix);

	size = current_size + suffix_size;
	if (size < current_size || size < suffix_size)
		return NULL;

	/* ensure space for the '.' and the '\0' characters. */
	if (size >= (SIZE_MAX - 2))
		return NULL;

	size += 2;

	if (size > max)
		return NULL;

	/* Append any given suffix */
	char *to = current + current_size;
	*to++ = '.';
	strcpy(to, suffix);

	return current;
}

static int fcontext_is_binary(FILE *fp)
{
	uint32_t magic;
	int rc;

	size_t len = fread(&magic, sizeof(magic), 1, fp);

	rc = fseek(fp, 0L, SEEK_SET);
	if (rc == -1)
		return -1;

	if (!len)
		return 0;

	if (be32toh(magic) == SELINUX_MAGIC_COMPILED_FCONTEXT)
		return 1;

	/*
	 * Treat old format magic in little endian as fcontext file as well,
	 * to avoid it getting parsed as text file.
	 */
	if (le32toh(magic) == SELINUX_MAGIC_COMPILED_FCONTEXT)
		return 2;

	return 0;
}

#define ARRAY_SIZE(x) (sizeof(x) / sizeof((x)[0]))

static FILE *open_file(const char *path, const char *suffix,
		       char *save_path, size_t len, struct stat *sb, bool open_oldest)
{
	unsigned int i;
	int rc;
	char stack_path[len];
	struct file_details *found = NULL;

	/*
	 * Rolling append of suffix. Try to open with path.suffix then the
	 * next as path.suffix.suffix and so forth.
	 */
	struct file_details fdetails[2] = {
			{ .suffix = suffix },
			{ .suffix = "bin" }
	};

	rc = snprintf(stack_path, sizeof(stack_path), "%s", path);
	if (rc < 0 || (size_t)rc >= sizeof(stack_path)) {
		errno = ENAMETOOLONG;
		return NULL;
	}

	for (i = 0; i < ARRAY_SIZE(fdetails); i++) {

		/* This handles the case if suffix is null */
		path = rolling_append(stack_path, fdetails[i].suffix,
				      sizeof(stack_path));
		if (!path) {
			errno = ENOMEM;
			return NULL;
		}

		rc = stat(path, &fdetails[i].sb);
		if (rc)
			continue;

		/* first file thing found, just take it */
		if (!found) {
			strcpy(save_path, path);
			found = &fdetails[i];
			continue;
		}

		/*
		 * Keep picking the newest file found. Where "newest"
		 * includes equality. This provides a precedence on
		 * secondary suffixes even when the timestamp is the
		 * same. Ie choose file_contexts.bin over file_contexts
		 * even if the time stamp is the same. Invert this logic
		 * on open_oldest set to true. The idea is that if the
		 * newest file failed to process, we can attempt to
		 * process the oldest. The logic here is subtle and depends
		 * on the array ordering in fdetails for the case when time
		 * stamps are the same.
		 */
		if (open_oldest ^
			(fdetails[i].sb.st_mtime >= found->sb.st_mtime)) {
			found = &fdetails[i];
			strcpy(save_path, path);
		}
	}

	if (!found) {
		errno = ENOENT;
		return NULL;
	}

	memcpy(sb, &found->sb, sizeof(*sb));
	return fopen(save_path, "re");
}

static int process_file(const char *path, const char *suffix,
			struct selabel_handle *rec,
			const char *prefix,
			struct selabel_digest *digest,
			uint8_t inputno)
{
	int rc;
	unsigned int i;
	struct stat sb;
	FILE *fp = NULL;
	char found_path[PATH_MAX];

	/*
	 * On the first pass open the newest modified file. If it fails to
	 * process, then the second pass shall open the oldest file. If both
	 * passes fail, then it's a fatal error.
	 */
	for (i = 0; i < 2; i++) {
		fp = open_file(path, suffix, found_path, sizeof(found_path),
			&sb, i > 0);
		if (fp == NULL)
			return -1;

		rc = fcontext_is_binary(fp);
		if (rc < 0) {
			fclose_errno_safe(fp);
			return -1;
		}

		if (rc == 2) {
			COMPAT_LOG(SELINUX_INFO, "%s:  Old compiled fcontext format, skipping\n", found_path);
			errno = EINVAL;
		} else if (rc == 1) {
			rc = load_mmap(fp, sb.st_size, rec, found_path, inputno);
		} else {
			rc = process_text_file(fp, prefix, rec, found_path, inputno);
		}

		if (!rc)
			rc = digest_add_specfile(digest, fp, NULL, sb.st_size,
				found_path);

		fclose_errno_safe(fp);

		if (!rc)
			return 0;
	}
	return -1;
}

static void selabel_subs_fini(struct selabel_sub *subs, uint32_t num)
{
	for (uint32_t i = 0; i < num; i++) {
		free(subs[i].src);
		free(subs[i].dst);
	}

	free(subs);
}

static char *selabel_apply_subs(const struct selabel_sub *subs, uint32_t num, const char *src, size_t slen)
{
	char *dst, *tmp;
	uint32_t len;

	for (uint32_t i = 0; i < num; i++) {
		const struct selabel_sub *ptr = &subs[i];

		if (strncmp(src, ptr->src, ptr->slen) == 0 ) {
			if (src[ptr->slen] == '/' ||
			    src[ptr->slen] == '\0') {
				if ((src[ptr->slen] == '/') &&
				    (strcmp(ptr->dst, "/") == 0))
					len = ptr->slen + 1;
				else
					len = ptr->slen;

				dst = malloc(ptr->dlen + slen - len + 1);
				if (!dst)
					return NULL;

				tmp = mempcpy(dst, ptr->dst, ptr->dlen);
				tmp = mempcpy(tmp, &src[len], slen - len);
				*tmp = '\0';
				return dst;
			}
		}
	}

	return NULL;
}

#if !defined(BUILD_HOST) && !defined(ANDROID)
static int selabel_subs_init(const char *path, struct selabel_digest *digest,
			     struct selabel_sub **out_subs,
			     uint32_t *out_num, uint32_t *out_alloc)
{
	char buf[1024];
	FILE *cfg;
	struct stat sb;
	struct selabel_sub *tmp = NULL;
	uint32_t tmp_num = 0, tmp_alloc = 0;
	char *src_cpy = NULL, *dst_cpy = NULL;
	int rc;

	*out_subs = NULL;
	*out_num = 0;
	*out_alloc = 0;

	cfg = fopen(path, "re");
	if (!cfg) {
		/* If the file does not exist, it is not fatal */
		return (errno == ENOENT) ? 0 : -1;
	}

	while (fgets_unlocked(buf, sizeof(buf) - 1, cfg)) {
		char *ptr;
		char *src = buf;
		char *dst;
		size_t slen, dlen;

		while (*src && isspace((unsigned char)*src))
			src++;
		if (src[0] == '#') continue;
		ptr = src;
		while (*ptr && ! isspace((unsigned char)*ptr))
			ptr++;
		*ptr++ = '\0';
		if (! *src) continue;

		dst = ptr;
		while (*dst && isspace((unsigned char)*dst))
			dst++;
		ptr = dst;
		while (*ptr && ! isspace((unsigned char)*ptr))
			ptr++;
		*ptr = '\0';
		if (! *dst)
			continue;

		slen = strlen(src);
		if (slen >= UINT32_MAX) {
			errno = EINVAL;
			goto err;
		}

		dlen = strlen(dst);
		if (dlen >= UINT32_MAX) {
			errno = EINVAL;
			goto err;
		}

		src_cpy = strdup(src);
		if (!src_cpy)
			goto err;

		dst_cpy = strdup(dst);
		if (!dst_cpy)
			goto err;

		rc = GROW_ARRAY(tmp);
		if (rc)
			goto err;

		tmp[tmp_num++] = (struct selabel_sub) {
			.src = src_cpy,
			.slen = slen,
			.dst = dst_cpy,
			.dlen = dlen,
		};
		src_cpy = NULL;
		dst_cpy = NULL;
	}

	rc = fstat(fileno(cfg), &sb);
	if (rc < 0)
		goto err;

	if (digest_add_specfile(digest, cfg, NULL, sb.st_size, path) < 0)
		goto err;

	*out_subs = tmp;
	*out_num = tmp_num;
	*out_alloc = tmp_alloc;

	fclose(cfg);

	return 0;

err:
	free(dst_cpy);
	free(src_cpy);
	for (uint32_t i = 0; i < tmp_num; i++) {
		free(tmp[i].src);
		free(tmp[i].dst);
	}
	free(tmp);
	fclose_errno_safe(cfg);
	return -1;
}
#endif

static char *selabel_sub_key(const struct saved_data *data, const char *key, size_t key_len)
{
	char *ptr, *dptr;

	ptr = selabel_apply_subs(data->subs, data->subs_num, key, key_len);
	if (ptr) {
		dptr = selabel_apply_subs(data->dist_subs, data->dist_subs_num, ptr, strlen(ptr));
		if (dptr) {
			free(ptr);
			ptr = dptr;
		}
	} else {
		ptr = selabel_apply_subs(data->dist_subs, data->dist_subs_num, key, key_len);
	}

	return ptr;
}

static void closef(struct selabel_handle *rec);

static int init(struct selabel_handle *rec, const struct selinux_opt *opts,
		unsigned n)
{
	struct saved_data *data = rec->data;
	const char *path = NULL;
	const char *prefix = NULL;
	int status = -1, baseonly = 0;

	/* Process arguments */
	while (n) {
		n--;
		switch(opts[n].type) {
		case SELABEL_OPT_PATH:
			path = opts[n].value;
			break;
		case SELABEL_OPT_SUBSET:
			prefix = opts[n].value;
			break;
		case SELABEL_OPT_BASEONLY:
			baseonly = !!opts[n].value;
			break;
		case SELABEL_OPT_UNUSED:
		case SELABEL_OPT_VALIDATE:
		case SELABEL_OPT_DIGEST:
			break;
		default:
			errno = EINVAL;
			return -1;
		}
	}

#if !defined(BUILD_HOST) && !defined(ANDROID)
	char subs_file[PATH_MAX + 1];
	/* Process local and distribution substitution files */
	if (!path) {
		status = selabel_subs_init(
			selinux_file_context_subs_dist_path(),
			rec->digest,
			&data->dist_subs, &data->dist_subs_num, &data->dist_subs_alloc);
		if (status)
			goto finish;
		status = selabel_subs_init(selinux_file_context_subs_path(),
			rec->digest,
			&data->subs, &data->subs_num, &data->subs_alloc);
		if (status)
			goto finish;
		path = selinux_file_context_path();
	} else {
		snprintf(subs_file, sizeof(subs_file), "%s.subs_dist", path);
		status = selabel_subs_init(subs_file, rec->digest,
					   &data->dist_subs, &data->dist_subs_num, &data->dist_subs_alloc);
		if (status)
			goto finish;
		snprintf(subs_file, sizeof(subs_file), "%s.subs", path);
		status = selabel_subs_init(subs_file, rec->digest,
					   &data->subs, &data->subs_num, &data->subs_alloc);
		if (status)
			goto finish;
	}

#endif

	if (!path) {
		errno = EINVAL;
		goto finish;
	}

	rec->spec_file = strdup(path);
	if (!rec->spec_file)
		goto finish;

	/*
	 * The do detailed validation of the input and fill the spec array
	 */
	status = process_file(path, NULL, rec, prefix, rec->digest, 0);
	if (status)
		goto finish;

	if (rec->validating) {
		sort_specs(data);

		status = nodups_spec_node(data->root, path);
		if (status)
			goto finish;
	}

	if (!baseonly) {
		status = process_file(path, "homedirs", rec, prefix,
							    rec->digest, 1);
		if (status && errno != ENOENT)
			goto finish;

		status = process_file(path, "local", rec, prefix,
							    rec->digest, 2);
		if (status && errno != ENOENT)
			goto finish;
	}

	if (!rec->validating || !baseonly)
		sort_specs(data);

	digest_gen_hash(rec->digest);

	status = 0;

finish:
	if (status)
		closef(rec);

	return status;
}

/*
 * Backend interface routines
 */
static void closef(struct selabel_handle *rec)
{
	struct saved_data *data = (struct saved_data *)rec->data;
	struct mmap_area *area, *last_area;

	if (!data)
		return;

	selabel_subs_fini(data->subs, data->subs_num);
	selabel_subs_fini(data->dist_subs, data->dist_subs_num);

	free_spec_node(data->root);
	free(data->root);

	area = data->mmap_areas;
	while (area) {
		munmap(area->addr, area->len);
		last_area = area;
		area = area->next;
		free(last_area);
	}
	free(data);
	rec->data = NULL;
}

static uint32_t search_literal_spec(const struct literal_spec *array, uint32_t size, const char *key, size_t key_len, bool partial)
{
	uint32_t lower, upper;

	if (size == 0)
		return (uint32_t)-1;

	lower = 0;
	upper = size - 1;

	while (lower <= upper) {
		uint32_t m = lower + (upper - lower) / 2;
		int r;

		if (partial)
			r = strncmp(array[m].literal_match, key, key_len);
		else
			r = strcmp(array[m].literal_match, key);

		if (r == 0) {
			/* Return the first result, regardless of file kind */
			while (m > 0) {
				if (partial)
					r = strncmp(array[m - 1].literal_match, key, key_len);
				else
					r = strcmp(array[m - 1].literal_match, key);

				if (r == 0)
					m--;
				else
					break;
			}
			return m;
		}

		if (r < 0)
			lower = m + 1;
		else {
			if (m == 0)
				break;

			upper = m - 1;
		}
	}

	return (uint32_t)-1;
}

FUZZ_EXTERN void free_lookup_result(struct lookup_result *result)
{
	struct lookup_result *tmp;

	while (result) {
		tmp = result->next;
		free(result);
		result = tmp;
	}
}

/**
 * lookup_check_node() - Try to find a file context definition in the given node or parents.
 * @node:      The deepest specification node to match against. Parent nodes are successively
 *             searched on no match or when finding all matches.
 * @key:       The absolute file path to look up.
 * @file_kind: The kind of the file to look up (translated from file type into LABEL_FILE_KIND_*).
 * @partial:   Whether to partially match the given file path or completely.
 * @find_all:  Whether to find all file context definitions or just the most specific.
 * @buf:       A pre-allocated buffer for a potential result to avoid allocating it on the heap or
 *             NULL. Mututal exclusive with @find_all.
 *
 * Return: A pointer to a file context definition if a match was found. If @find_all was specified
 *         its a linked list of all results. If @buf was specified it is returned on a match found.
 *         NULL is returned in case of no match found.
 */
static struct lookup_result *lookup_check_node(struct spec_node *node, const char *key, uint8_t file_kind,
					       bool partial, bool find_all, struct lookup_result *buf)
{
	struct lookup_result *result = NULL;
	struct lookup_result **next = &result;
	struct lookup_result *child_regex_match = NULL;
	uint8_t child_regex_match_inputno = 0;  /* initialize to please GCC */
	uint32_t child_regex_match_lineno = 1;  /* initialize to please GCC */
	size_t key_len = strlen(key);

	assert(!(find_all && buf != NULL));

	for (struct spec_node *n = node; n; n = n->parent) {

		if (n == node) {
			uint32_t literal_idx = search_literal_spec(n->literal_specs, n->literal_specs_num, key, key_len, partial);
			if (literal_idx != (uint32_t)-1) {
				do {
					struct literal_spec *lspec = &n->literal_specs[literal_idx];

					if (file_kind == LABEL_FILE_KIND_ALL || lspec->file_kind == LABEL_FILE_KIND_ALL || lspec->file_kind == file_kind) {
						struct lookup_result *r;

#ifdef __ATOMIC_RELAXED
						__atomic_store_n(&lspec->any_matches, true, __ATOMIC_RELAXED);
#else
#error "Please use a compiler that supports __atomic builtins"
#endif

						if (strcmp(lspec->lr.ctx_raw, "<<none>>") == 0) {
							errno = ENOENT;
							goto fail;
						}

						if (likely(buf)) {
							r = buf;
						} else {
							r = malloc(sizeof(*r));
							if (!r)
								goto fail;
						}

						*r = (struct lookup_result) {
							.regex_str = lspec->regex_str,
							.prefix_len = lspec->prefix_len,
							.file_kind = lspec->file_kind,
							.lr = &lspec->lr,
							.has_meta_chars = false,
							.next = NULL,
						};

						if (likely(!find_all))
							return r;

						*next = r;
						next = &r->next;
					}

					literal_idx++;
				} while (literal_idx < n->literal_specs_num &&
					(partial ? (strncmp(n->literal_specs[literal_idx].literal_match, key, key_len) == 0)
						: (strcmp(n->literal_specs[literal_idx].literal_match, key) == 0)));
			}
		}

		for (uint32_t i = n->regex_specs_num; i > 0; i--) {
			/* search in reverse order */
			struct regex_spec *rspec = &n->regex_specs[i - 1];
			char errbuf[256];
			int rc;

			if (child_regex_match &&
			    (rspec->inputno < child_regex_match_inputno ||
			     (rspec->inputno == child_regex_match_inputno && rspec->lineno < child_regex_match_lineno)))
				break;

			if (file_kind != LABEL_FILE_KIND_ALL && rspec->file_kind != LABEL_FILE_KIND_ALL && file_kind != rspec->file_kind)
				continue;

			if (compile_regex(rspec, errbuf, sizeof(errbuf)) < 0) {
				COMPAT_LOG(SELINUX_ERROR, "Failed to compile regular expression '%s':  %s\n",
					   rspec->regex_str, errbuf);
				goto fail;
			}

			rc = regex_match(rspec->regex, key, partial);
			if (rc == REGEX_MATCH || (partial && rc == REGEX_MATCH_PARTIAL)) {
				struct lookup_result *r;

				if (rc == REGEX_MATCH) {
#ifdef __ATOMIC_RELAXED
					__atomic_store_n(&rspec->any_matches, true, __ATOMIC_RELAXED);
#else
#error "Please use a compiler that supports __atomic builtins"
#endif
				}

				if (strcmp(rspec->lr.ctx_raw, "<<none>>") == 0) {
					errno = ENOENT;
					goto fail;
				}

				if (child_regex_match) {
					r = child_regex_match;
				} else if (buf) {
					r = buf;
				} else {
					r = malloc(sizeof(*r));
					if (!r)
						goto fail;
				}

				*r = (struct lookup_result) {
					.regex_str = rspec->regex_str,
					.prefix_len = rspec->prefix_len,
					.file_kind = rspec->file_kind,
					.lr = &rspec->lr,
					.has_meta_chars = true,
					.next = NULL,
				};

				if (likely(!find_all)) {
					child_regex_match = r;
					child_regex_match_inputno = rspec->inputno;
					child_regex_match_lineno = rspec->lineno;
					goto parent_node;
				}

				*next = r;
				next = &r->next;

				continue;
			}

			if (rc == REGEX_NO_MATCH)
				continue;

			/* else it's an error */
			errno = ENOENT;
			goto fail;
		}

	    parent_node:
		continue;
	}

	if (child_regex_match)
		return child_regex_match;

	if (!result)
		errno = ENOENT;
	return result;

    fail:
	if (!find_all && child_regex_match && child_regex_match != buf)
		free(child_regex_match);

	free_lookup_result(result);

	return NULL;
}

static struct spec_node* search_child_node(struct spec_node *array, uint32_t size, const char *key, size_t key_len)
{
	uint32_t lower, upper;

	if (size == 0)
		return NULL;

	lower = 0;
	upper = size - 1;

	while (lower <= upper) {
		uint32_t m = lower + (upper - lower) / 2;
		int r;

		r = strncmp(array[m].stem, key, key_len);

		if (r == 0 && array[m].stem[key_len] == '\0')
			return &array[m];

		if (r < 0)
			lower = m + 1;
		else {
			if (m == 0)
				break;

			upper = m - 1;
		}
	}

	return NULL;
}

static struct spec_node* lookup_find_deepest_node(struct spec_node *node, const char *key)
{
	/* Find the node matching the deepest stem */

	struct spec_node *n = node;
	const char *p = key;

	while (true) {
		struct spec_node *child;
		size_t length;
		const char *q;

		if (*p != '/')
			break;

		q = strchr(p + 1, '/');
		if (q == NULL)
			break;

		length = q - p - 1;
		if (length == 0)
			break;

		child = search_child_node(n->children, n->children_num, p + 1, length);
		if (!child)
			break;

		n = child;
		p = q;
	}

	return n;
}

static uint8_t mode_to_file_kind(int type) {
	type &= S_IFMT;

	switch (type) {
	case S_IFBLK:
		return LABEL_FILE_KIND_BLK;
	case S_IFCHR:
		return LABEL_FILE_KIND_CHR;
	case S_IFDIR:
		return LABEL_FILE_KIND_DIR;
	case S_IFIFO:
		return LABEL_FILE_KIND_FIFO;
	case S_IFLNK:
		return LABEL_FILE_KIND_LNK;
	case S_IFSOCK:
		return LABEL_FILE_KIND_SOCK;
	case S_IFREG:
		return LABEL_FILE_KIND_REG;
	case 0:
	default:
		return LABEL_FILE_KIND_ALL;
	}
}

// Finds all the matches of |key| in the given context. Returns the result in
// the allocated array and updates the match count. If match_count is NULL,
// stops early once the 1st match is found.
FUZZ_EXTERN struct lookup_result *lookup_all(struct selabel_handle *rec,
				 const char *key,
				 int type,
				 bool partial,
				 bool find_all,
				 struct lookup_result *buf)
{
	struct saved_data *data = (struct saved_data *)rec->data;
	struct lookup_result *result = NULL;
	struct spec_node *node;
	size_t len;
	uint8_t file_kind = mode_to_file_kind(type);
	char *clean_key = NULL;
	const char *prev_slash, *next_slash;
	unsigned int sofar = 0;
	char *sub = NULL;

	if (unlikely(!key)) {
		errno = EINVAL;
		goto finish;
	}

	if (unlikely(!data->num_specs)) {
		errno = ENOENT;
		goto finish;
	}

	/* Remove duplicate slashes */
	if (unlikely(next_slash = strstr(key, "//"))) {
		clean_key = (char *) malloc(strlen(key) + 1);
		if (!clean_key)
			goto finish;
		prev_slash = key;
		while (next_slash) {
			memcpy(clean_key + sofar, prev_slash, next_slash - prev_slash);
			sofar += next_slash - prev_slash;
			prev_slash = next_slash + 1;
			next_slash = strstr(prev_slash, "//");
		}
		strcpy(clean_key + sofar, prev_slash);
		key = clean_key;
	}

	/* remove trailing slash */
	len = strlen(key);
	if (unlikely(len == 0)) {
		errno = EINVAL;
		goto finish;
	}

	if (unlikely(len > 1 && key[len - 1] == '/')) {
		/* reuse clean_key from above if available */
		if (!clean_key) {
			clean_key = (char *) malloc(len);
			if (!clean_key)
				goto finish;

			memcpy(clean_key, key, len - 1);
		}

		clean_key[len - 1] = '\0';
		key = clean_key;
		len--;
	}

	sub = selabel_sub_key(data, key, len);
	if (sub)
		key = sub;

	node = lookup_find_deepest_node(data->root, key);

	result = lookup_check_node(node, key, file_kind, partial, find_all, buf);

finish:
	free(clean_key);
	free(sub);
	return result;
}

static struct lookup_result *lookup_common(struct selabel_handle *rec,
					   const char *key,
					   int type,
					   bool partial,
					   struct lookup_result *buf) {
	return lookup_all(rec, key, type, partial, false, buf);
}

/*
 * Returns true if the digest of all partial matched contexts is the same as
 * the one saved by setxattr, otherwise returns false. The length of the SHA1
 * digest will always be returned. The caller must free any returned digests.
 */
static bool get_digests_all_partial_matches(struct selabel_handle *rec,
					    const char *pathname,
					    uint8_t **calculated_digest,
					    uint8_t **xattr_digest,
					    size_t *digest_len)
{
	uint8_t read_digest[SHA1_HASH_SIZE];
	ssize_t read_size = getxattr(pathname, RESTORECON_PARTIAL_MATCH_DIGEST,
				     read_digest, SHA1_HASH_SIZE
#ifdef __APPLE__
				     , 0, 0
#endif /* __APPLE __ */
				    );
	uint8_t hash_digest[SHA1_HASH_SIZE];
	bool status = selabel_hash_all_partial_matches(rec, pathname,
						       hash_digest);

	*xattr_digest = NULL;
	*calculated_digest = NULL;
	*digest_len = SHA1_HASH_SIZE;

	if (read_size == SHA1_HASH_SIZE) {
		*xattr_digest = calloc(1, SHA1_HASH_SIZE + 1);
		if (!*xattr_digest)
			goto oom;

		memcpy(*xattr_digest, read_digest, SHA1_HASH_SIZE);
	}

	if (status) {
		*calculated_digest = calloc(1, SHA1_HASH_SIZE + 1);
		if (!*calculated_digest)
			goto oom;

		memcpy(*calculated_digest, hash_digest, SHA1_HASH_SIZE);
	}

	if (status && read_size == SHA1_HASH_SIZE &&
		memcmp(read_digest, hash_digest, SHA1_HASH_SIZE) == 0)
		return true;

	return false;

oom:
	selinux_log(SELINUX_ERROR, "SELinux: %s: Out of memory\n", __func__);
	return false;
}

static bool hash_all_partial_matches(struct selabel_handle *rec, const char *key, uint8_t *digest)
{
	assert(digest);

	struct lookup_result *matches = lookup_all(rec, key, 0, true, true, NULL);
	if (!matches) {
		return false;
	}

	Sha1Context context;
	Sha1Initialise(&context);

	for (const struct lookup_result *m = matches; m; m = m->next) {
		const char* regex_str = m->regex_str;
		uint8_t file_kind = m->file_kind;
		const char* ctx_raw = m->lr->ctx_raw;

		Sha1Update(&context, regex_str, strlen(regex_str) + 1);
		Sha1Update(&context, &file_kind, sizeof(file_kind));
		Sha1Update(&context, ctx_raw, strlen(ctx_raw) + 1);
	}

	SHA1_HASH sha1_hash;
	Sha1Finalise(&context, &sha1_hash);
	memcpy(digest, sha1_hash.bytes, SHA1_HASH_SIZE);

	free_lookup_result(matches);
	return true;
}

static struct selabel_lookup_rec *lookup(struct selabel_handle *rec,
					 const char *key, int type)
{
	struct lookup_result buf, *result;

	result = lookup_common(rec, key, type, false, &buf);
	if (!result)
		return NULL;

	return result->lr;
}

static bool partial_match(struct selabel_handle *rec, const char *key)
{
	struct lookup_result buf;

	return !!lookup_common(rec, key, 0, true, &buf);
}

static struct selabel_lookup_rec *lookup_best_match(struct selabel_handle *rec,
						    const char *key,
						    const char **aliases,
						    int type)
{
	size_t n, i, best = (size_t)-1;
	struct lookup_result **results;
	uint16_t prefix_len = 0;
	struct selabel_lookup_rec *lr = NULL;

	if (!aliases || !aliases[0])
		return lookup(rec, key, type);

	for (n = 0; aliases[n]; n++)
		;

	results = calloc(n+1, sizeof(*results));
	if (!results)
		return NULL;
	results[0] = lookup_common(rec, key, type, false, NULL);
	if (results[0]) {
		if (!results[0]->has_meta_chars) {
			/* exact match on key */
			lr = results[0]->lr;
			goto out;
		}
		best = 0;
		prefix_len = results[0]->prefix_len;
	}
	for (i = 1; i <= n; i++) {
		results[i] = lookup_common(rec, aliases[i-1], type, false, NULL);
		if (results[i]) {
			if (!results[i]->has_meta_chars) {
				/* exact match on alias */
				lr = results[i]->lr;
				goto out;
			}
			if (results[i]->prefix_len > prefix_len) {
				best = i;
				prefix_len = results[i]->prefix_len;
			}
		}
	}

	if (best != (size_t)-1) {
		/* longest fixed prefix match on key or alias */
		lr = results[best]->lr;
	} else {
		errno = ENOENT;
	}

out:
	for (i = 0; i <= n; i++)
		free_lookup_result(results[i]);
	free(results);
	return lr;
}

static void spec_node_stats(const struct spec_node *node)
{
	bool any_matches;

	for (uint32_t i = 0; i < node->literal_specs_num; i++) {
		const struct literal_spec *lspec = &node->literal_specs[i];

#ifdef __ATOMIC_RELAXED
		any_matches = __atomic_load_n(&node->literal_specs[i].any_matches, __ATOMIC_RELAXED);
#else
#error "Please use a compiler that supports __atomic builtins"
#endif

		if (!any_matches) {
			COMPAT_LOG(SELINUX_WARNING,
				"Warning!  No matches for (%s, %s, %s)\n",
				lspec->regex_str,
				file_kind_to_string(lspec->file_kind),
				lspec->lr.ctx_raw);
		}
	}

	for (uint32_t i = 0; i < node->regex_specs_num; i++) {
		const struct regex_spec *rspec = &node->regex_specs[i];

#ifdef __ATOMIC_RELAXED
		any_matches = __atomic_load_n(&rspec->any_matches, __ATOMIC_RELAXED);
#else
#error "Please use a compiler that supports __atomic builtins"
#endif

		if (!any_matches) {
			COMPAT_LOG(SELINUX_WARNING,
				"Warning!  No matches for (%s, %s, %s)\n",
				rspec->regex_str,
				file_kind_to_string(rspec->file_kind),
				rspec->lr.ctx_raw);
		}
	}

	for (uint32_t i = 0; i < node->children_num; i++)
		spec_node_stats(&node->children[i]);
}

static void stats(struct selabel_handle *rec)
{
	const struct saved_data *data = (const struct saved_data *)rec->data;

	spec_node_stats(data->root);
}

static inline const char* fmt_stem(const char *stem)
{
	return stem ?: "(root)";
}

static enum selabel_cmp_result lspec_incomp(const char *stem, const struct literal_spec *lspec1, const struct literal_spec *lspec2, const char *reason, uint32_t iter1, uint32_t iter2)
{
	selinux_log(SELINUX_INFO,
		    "selabel_cmp: mismatched %s in stem %s on literal entry %u: (%s, %s, %s) vs entry %u: (%s, %s, %s)\n",
		    reason,
		    fmt_stem(stem),
		    iter1, lspec1->regex_str, file_kind_to_string(lspec1->file_kind), lspec1->lr.ctx_raw,
		    iter2, lspec2->regex_str, file_kind_to_string(lspec2->file_kind), lspec2->lr.ctx_raw);
	return SELABEL_INCOMPARABLE;
}

static enum selabel_cmp_result rspec_incomp(const char *stem, const struct regex_spec *rspec1, const struct regex_spec *rspec2, const char *reason, uint32_t iter1, uint32_t iter2)
{
	selinux_log(SELINUX_INFO,
		    "selabel_cmp: mismatched %s in stem %s on regex entry %u: (%s, %s, %s) vs entry %u: (%s, %s, %s)\n",
		    reason,
		    fmt_stem(stem),
		    iter1, rspec1->regex_str, file_kind_to_string(rspec1->file_kind), rspec1->lr.ctx_raw,
		    iter2, rspec2->regex_str, file_kind_to_string(rspec2->file_kind), rspec2->lr.ctx_raw);
	return SELABEL_INCOMPARABLE;
}

static enum selabel_cmp_result spec_node_cmp(const struct spec_node *node1, const struct spec_node *node2)
{
	enum selabel_cmp_result result = SELABEL_EQUAL;

	if ((node1->stem && node2->stem && strcmp(node1->stem, node2->stem) != 0) ||
	    (node1->stem && node1->stem[0] != '\0' && !node2->stem) ||
		(!node1->stem && node2->stem && node2->stem[0] != '\0')) {
		selinux_log(SELINUX_INFO, "selabel_cmp: incompareable nodes: %s vs %s\n",
					fmt_stem(node1->stem), fmt_stem(node2->stem));
		return SELABEL_INCOMPARABLE;
	}

	/* Literal specs comparison */
	{
		uint32_t iter1 = 0, iter2 = 0;
		while (iter1 < node1->literal_specs_num && iter2 < node2->literal_specs_num) {
			const struct literal_spec *lspec1 = &node1->literal_specs[iter1];
			const struct literal_spec *lspec2 = &node2->literal_specs[iter2];
			int cmp;

			cmp = strcmp(lspec1->literal_match, lspec2->literal_match);
			if (cmp < 0) {
				if (result == SELABEL_EQUAL || result == SELABEL_SUPERSET) {
					result = SELABEL_SUPERSET;
					iter1++;
					continue;
				}

				return lspec_incomp(node1->stem, lspec1, lspec2, "literal_str", iter1, iter2);
			}

			if (cmp > 0) {
				if (result == SELABEL_EQUAL || result == SELABEL_SUBSET) {
					result = SELABEL_SUBSET;
					iter2++;
					continue;
				}

				return lspec_incomp(node1->stem, lspec1, lspec2, "literal_str", iter1, iter2);
			}

			/* If literal match is equal compare file kind */

			if (lspec1->file_kind > lspec2->file_kind) {
				if (result == SELABEL_EQUAL || result == SELABEL_SUPERSET) {
					result = SELABEL_SUPERSET;
					iter1++;
					continue;
				}

				return lspec_incomp(node1->stem, lspec1, lspec2, "file_kind", iter1, iter2);
			}

			if (lspec1->file_kind < lspec2->file_kind) {
				if (result == SELABEL_EQUAL || result == SELABEL_SUBSET) {
					result = SELABEL_SUBSET;
					iter2++;
					continue;
				}

				return lspec_incomp(node1->stem, lspec1, lspec2, "file_kind", iter1, iter2);
			}

			iter1++;
			iter2++;
		}
		if (iter1 != node1->literal_specs_num) {
			if (result == SELABEL_EQUAL || result == SELABEL_SUPERSET) {
				result = SELABEL_SUPERSET;
			} else {
				selinux_log(SELINUX_INFO, "selabel_cmp: mismatch literal left remnant in stem %s\n", fmt_stem(node1->stem));
				return SELABEL_INCOMPARABLE;
			}
		}
		if (iter2 != node2->literal_specs_num) {
			if (result == SELABEL_EQUAL || result == SELABEL_SUBSET) {
				result = SELABEL_SUBSET;
			} else {
				selinux_log(SELINUX_INFO, "selabel_cmp: mismatch literal right remnant in stem %s\n", fmt_stem(node1->stem));
				return SELABEL_INCOMPARABLE;
			}
		}
	}

	/* Regex specs comparison */
	{
		uint32_t iter1 = 0, iter2 = 0;
		while (iter1 < node1->regex_specs_num && iter2 < node2->regex_specs_num) {
			const struct regex_spec *rspec1 = &node1->regex_specs[iter1];
			const struct regex_spec *rspec2 = &node2->regex_specs[iter2];
			bool found_successor;

			if (rspec1->file_kind == rspec2->file_kind && strcmp(rspec1->regex_str, rspec2->regex_str) == 0) {
				iter1++;
				iter2++;
				continue;
			}

			if (result == SELABEL_SUPERSET) {
				iter1++;
				continue;
			}

			if (result == SELABEL_SUBSET) {
				iter2++;
				continue;
			}

			assert(result == SELABEL_EQUAL);

			found_successor = false;

			for (uint32_t i = iter2; i < node2->regex_specs_num; i++) {
				const struct regex_spec *successor = &node2->regex_specs[i];

				if (rspec1->file_kind == successor->file_kind && strcmp(rspec1->regex_str, successor->regex_str) == 0) {
					result = SELABEL_SUBSET;
					iter1++;
					iter2 = i + 1;
					found_successor = true;
					break;
				}
			}

			if (found_successor)
				continue;

			for (uint32_t i = iter1; i < node1->regex_specs_num; i++) {
				const struct regex_spec *successor = &node1->regex_specs[i];

				if (successor->file_kind == rspec2->file_kind && strcmp(successor->regex_str, rspec2->regex_str) == 0) {
					result = SELABEL_SUPERSET;
					iter1 = i + 1;
					iter2++;
					found_successor = true;
					break;
				}
			}

			if (found_successor)
				continue;

			return rspec_incomp(node1->stem, rspec1, rspec2, "regex", iter1, iter2);
		}
		if (iter1 != node1->regex_specs_num) {
			if (result == SELABEL_EQUAL || result == SELABEL_SUPERSET) {
				result = SELABEL_SUPERSET;
			} else {
				const struct regex_spec *rspec1 = &node1->regex_specs[iter1];

				selinux_log(SELINUX_INFO, "selabel_cmp: mismatch regex left remnant in stem %s entry %u: (%s, %s, %s)\n",
					    fmt_stem(node1->stem),
					    iter1, rspec1->regex_str, file_kind_to_string(rspec1->file_kind), rspec1->lr.ctx_raw);
				return SELABEL_INCOMPARABLE;
			}
		}
		if (iter2 != node2->regex_specs_num) {
			if (result == SELABEL_EQUAL || result == SELABEL_SUBSET) {
				result = SELABEL_SUBSET;
			} else {
				const struct regex_spec *rspec2 = &node2->regex_specs[iter2];

				selinux_log(SELINUX_INFO, "selabel_cmp: mismatch regex right remnant in stem %s entry %u: (%s, %s, %s)\n",
					    fmt_stem(node1->stem),
					    iter2, rspec2->regex_str, file_kind_to_string(rspec2->file_kind), rspec2->lr.ctx_raw);
				return SELABEL_INCOMPARABLE;
			}
		}
	}

	/* Child nodes comparison */
	{
		uint32_t iter1 = 0, iter2 = 0;
		while (iter1 < node1->children_num && iter2 < node2->children_num) {
			const struct spec_node *child1 = &node1->children[iter1];
			const struct spec_node *child2 = &node2->children[iter2];
			enum selabel_cmp_result child_result;
			int cmp;

			cmp = strcmp(child1->stem, child2->stem);
			if (cmp < 0) {
				if (result == SELABEL_EQUAL || result == SELABEL_SUPERSET) {
					result = SELABEL_SUPERSET;
					iter1++;
					continue;
				}

				selinux_log(SELINUX_INFO, "selabel_cmp: mismatch left remnant child node %s stem %s\n", child1->stem, fmt_stem(node1->stem));
				return SELABEL_INCOMPARABLE;
			}

			if (cmp > 0) {
				if (result == SELABEL_EQUAL || result == SELABEL_SUBSET) {
						result = SELABEL_SUBSET;
						iter2++;
						continue;
				}

				selinux_log(SELINUX_INFO, "selabel_cmp: mismatch right remnant child node %s stem %s\n", child1->stem, fmt_stem(node1->stem));
				return SELABEL_INCOMPARABLE;
			}

			iter1++;
			iter2++;

			/* If stem is equal do a deep comparison */

			child_result = spec_node_cmp(child1, child2);
			switch (child_result) {
			case SELABEL_SUBSET:
				if (result == SELABEL_EQUAL || result == SELABEL_SUBSET) {
					result = SELABEL_SUBSET;
					continue;
				}
				selinux_log(SELINUX_INFO, "selabel_cmp: mismatch child node %s stem %s\n", child1->stem, fmt_stem(node1->stem));
				return SELABEL_INCOMPARABLE;
			case SELABEL_EQUAL:
				continue;
			case SELABEL_SUPERSET:
				if (result == SELABEL_EQUAL || result == SELABEL_SUPERSET) {
					result = SELABEL_SUPERSET;
					continue;
				}
				selinux_log(SELINUX_INFO, "selabel_cmp: mismatch child node %s stem %s\n", child1->stem, fmt_stem(node1->stem));
				return SELABEL_INCOMPARABLE;
			case SELABEL_INCOMPARABLE:
			default:
				selinux_log(SELINUX_INFO, "selabel_cmp: mismatch child node %s stem %s\n", child1->stem, fmt_stem(node1->stem));
				return SELABEL_INCOMPARABLE;
			}
		}
		if (iter1 != node1->children_num) {
			if (result == SELABEL_EQUAL || result == SELABEL_SUPERSET) {
				result = SELABEL_SUPERSET;
			} else {
				selinux_log(SELINUX_INFO, "selabel_cmp: mismatch child left remnant in stem %s\n", fmt_stem(node1->stem));
				return SELABEL_INCOMPARABLE;
			}
		}
		if (iter2 != node2->children_num) {
			if (result == SELABEL_EQUAL || result == SELABEL_SUBSET) {
				result = SELABEL_SUBSET;
			} else {
				selinux_log(SELINUX_INFO, "selabel_cmp: mismatch child right remnant in stem %s\n", fmt_stem(node1->stem));
				return SELABEL_INCOMPARABLE;
			}
		}
	}

	return result;
}

FUZZ_EXTERN enum selabel_cmp_result cmp(const struct selabel_handle *h1, const struct selabel_handle *h2)
{
	const struct saved_data *data1, *data2;

	/* Ensured by selabel_cmp() */
	assert(h1->backend == SELABEL_CTX_FILE && h2->backend == SELABEL_CTX_FILE);

	data1 = h1->data;
	data2 = h2->data;

	if (data1->num_specs == 0)
		return data2->num_specs == 0 ? SELABEL_EQUAL : SELABEL_SUBSET;
	if (data2->num_specs == 0)
		return SELABEL_SUPERSET;

	return spec_node_cmp(data1->root, data2->root);
}

int selabel_file_init(struct selabel_handle *rec,
		      const struct selinux_opt *opts,
		      unsigned nopts)
{
	struct saved_data *data;
	struct spec_node *root;

	data = calloc(1, sizeof(*data));
	if (!data)
		return -1;

	root = calloc(1, sizeof(*root));
	if (!root) {
		free(data);
		return -1;
	}

	data->root = root;

	rec->data = data;
	rec->func_close = &closef;
	rec->func_stats = &stats;
	rec->func_lookup = &lookup;
	rec->func_partial_match = &partial_match;
	rec->func_get_digests_all_partial_matches = &get_digests_all_partial_matches;
	rec->func_hash_all_partial_matches = &hash_all_partial_matches;
	rec->func_lookup_best_match = &lookup_best_match;
	rec->func_cmp = &cmp;

	return init(rec, opts, nopts);
}
