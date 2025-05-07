/*
 * This file contains helper functions for labeling support.
 *
 * Author : Richard Haines <richard_c_haines@btinternet.com>
 */

#include <stdint.h>
#include <stdlib.h>
#include <stdarg.h>
#include <ctype.h>
#include <string.h>
#include <stdio.h>
#include <errno.h>
#include "label_internal.h"

/*
 * Read an entry from a spec file (e.g. file_contexts)
 * entry - Buffer to allocate for the entry.
 * ptr - current location of the line to be processed.
 * returns  - 0 on success and *entry is set to be a null
 *            terminated value. On Error it returns -1 and
 *            errno will be set.
 *
 */
static inline int read_spec_entry(char **entry, const char **ptr, size_t *len, const char **errbuf)
{
	const char *tmp_buf;

	*entry = NULL;

	while (isspace((unsigned char)**ptr) && **ptr != '\0')
		(*ptr)++;

	tmp_buf = *ptr;
	*len = 0;

	while (!isspace((unsigned char)**ptr) && **ptr != '\0') {
		if (!isascii((unsigned char)**ptr)) {
			errno = EINVAL;
			*errbuf = "Non-ASCII characters found";
			return -1;
		}
		(*ptr)++;
		(*len)++;
	}

	if (*len) {
		if (*len >= UINT16_MAX) {
			errno = EINVAL;
			*errbuf = "Spec entry too long";
			return -1;
		}

		*entry = strndup(tmp_buf, *len);
		if (!*entry)
			return -1;
	}

	return 0;
}

/*
 * line_buf - Buffer containing the spec entries .
 * errbuf   - Double pointer used for passing back specific error messages.
 * num_args - The number of spec parameter entries to process.
 * ...      - A 'char **spec_entry' for each parameter.
 * returns  - The number of items processed. On error, it returns -1 with errno
 *            set and may set errbuf to a specific error message.
 *
 * This function calls read_spec_entry() to do the actual string processing.
 * As such, can return anything from that function as well.
 */
int  read_spec_entries(char *line_buf, size_t nread, const char **errbuf, int num_args, ...)
{
	char **spec_entry;
	const char *buf_p;
	size_t entry_len = 0;
	int rc, items;
	va_list ap;

	*errbuf = NULL;

	if (line_buf[nread - 1] == '\n')
		line_buf[nread - 1] = '\0';
	else
		/* Handle case if line not \n terminated by bumping
		 * the len for the check below (as the line is NUL
		 * terminated by getline(3)) */
		nread++;

	buf_p = line_buf;
	while (isspace((unsigned char)*buf_p))
		buf_p++;

	/* Skip comment lines and empty lines. */
	if (*buf_p == '#' || *buf_p == '\0')
		return 0;

	/* Process the spec file entries */
	va_start(ap, num_args);

	items = 0;
	while (items < num_args) {
		spec_entry = va_arg(ap, char **);

		if (buf_p[0] == '\0' || nread - 1 == (size_t)(buf_p - line_buf)) {
			va_end(ap);
			return items;
		}

		rc = read_spec_entry(spec_entry, &buf_p, &entry_len, errbuf);
		if (rc < 0) {
			va_end(ap);
			return rc;
		}
		if (entry_len)
			items++;
	}
	va_end(ap);
	return items;
}

/* Once all the specfiles are in the hash_buf, generate the hash. */
void  digest_gen_hash(struct selabel_digest *digest)
{
	Sha1Context context;
	size_t remaining_size;
	const unsigned char *ptr;

	/* If SELABEL_OPT_DIGEST not set then just return */
	if (!digest)
		return;

	Sha1Initialise(&context);

	/* Process in blocks of UINT32_MAX bytes */
	remaining_size = digest->hashbuf_size;
	ptr = digest->hashbuf;
	while (remaining_size > UINT32_MAX) {
		Sha1Update(&context, ptr, UINT32_MAX);
		remaining_size -= UINT32_MAX;
		ptr += UINT32_MAX;
	}
	Sha1Update(&context, ptr, remaining_size);

	Sha1Finalise(&context, (SHA1_HASH *)digest->digest);
	free(digest->hashbuf);
	digest->hashbuf = NULL;
}

/**
 * digest_add_specfile - Add a specfile to the hashbuf and if gen_hash true
 *			 then generate the hash.
 * @digest: pointer to the selabel_digest struct
 * @fp: file pointer for fread(3) or NULL if not.
 * @from_addr: pointer at start of buffer for memcpy or NULL if not (used for
 *	       mmap(3) files).
 * @buf_len: length of buffer to copy.
 * @path: pointer to the specfile.
 *
 * Return %0 on success, -%1 with @errno set on failure.
 */
int  digest_add_specfile(struct selabel_digest *digest, FILE *fp,
				    const char *from_addr, size_t buf_len,
				    const char *path)
{
	unsigned char *tmp_buf;

	/* If SELABEL_OPT_DIGEST not set then just return */
	if (!digest)
		return 0;

	if (digest->hashbuf_size + buf_len < digest->hashbuf_size) {
		errno = EOVERFLOW;
		return -1;
	}
	digest->hashbuf_size += buf_len;

	tmp_buf = realloc(digest->hashbuf, digest->hashbuf_size);
	if (!tmp_buf)
		return -1;

	digest->hashbuf = tmp_buf;

	if (fp) {
		if (fseek(fp, 0L, SEEK_SET) == -1)
			return -1;

		if (fread(digest->hashbuf + (digest->hashbuf_size - buf_len),
					    1, buf_len, fp) != buf_len)
			return -1;

	} else if (from_addr) {
		tmp_buf = memcpy(digest->hashbuf +
				    (digest->hashbuf_size - buf_len),
				    from_addr, buf_len);
		if (!tmp_buf)
			return -1;
	}
	/* Now add path to list */
	digest->specfile_list[digest->specfile_cnt] = strdup(path);
	if (!digest->specfile_list[digest->specfile_cnt])
		return -1;

	digest->specfile_cnt++;
	if (digest->specfile_cnt > DIGEST_FILES_MAX) {
		errno = EOVERFLOW;
		return -1;
	}

	return 0;
}
