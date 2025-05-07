/*
 * Author: Karl MacMillan <kmacmillan@tresys.com>
 *
 * Modified:  
 *   Dan Walsh <dwalsh@redhat.com> - Added security_load_booleans().
 */

#ifndef DISABLE_BOOL

#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <stdlib.h>
#include <dirent.h>
#include <string.h>
#include <stdio.h>
#include <stdio_ext.h>
#include <unistd.h>
#include <fnmatch.h>
#include <limits.h>
#include <ctype.h>
#include <errno.h>

#include "selinux_internal.h"
#include "policy.h"

#define SELINUX_BOOL_DIR "/booleans/"

static int filename_select(const struct dirent *d)
{
	if (d->d_name[0] == '.'
	    && (d->d_name[1] == '\0'
		|| (d->d_name[1] == '.' && d->d_name[2] == '\0')))
		return 0;
	return 1;
}

int security_get_boolean_names(char ***names, int *len)
{
	char path[PATH_MAX];
	int i, rc;
	struct dirent **namelist;
	char **n;

	if (!len || names == NULL) {
		errno = EINVAL;
		return -1;
	}
	if (!selinux_mnt) {
		errno = ENOENT;
		return -1;
	}

	snprintf(path, sizeof path, "%s%s", selinux_mnt, SELINUX_BOOL_DIR);
	*len = scandir(path, &namelist, &filename_select, alphasort);
	if (*len < 0) {
		return -1;
	}
	if (*len == 0) {
		free(namelist);
		errno = ENOENT;
		return -1;
	}

	n = (char **)malloc(sizeof(char *) * *len);
	if (!n) {
		rc = -1;
		goto bad;
	}

	for (i = 0; i < *len; i++) {
		n[i] = strdup(namelist[i]->d_name);
		if (!n[i]) {
			rc = -1;
			goto bad_freen;
		}
	}
	rc = 0;
	*names = n;
      out:
	for (i = 0; i < *len; i++) {
		free(namelist[i]);
	}
	free(namelist);
	return rc;
      bad_freen:
	if (i > 0) {
		while (i >= 1)
			free(n[--i]);
	}
	free(n);
      bad:
	goto out;
}

char *selinux_boolean_sub(const char *name)
{
	char *sub = NULL;
	char *line_buf = NULL;
	size_t line_len;
	FILE *cfg;

	if (!name)
		return NULL;

	cfg = fopen(selinux_booleans_subs_path(), "re");
	if (!cfg)
		goto out;

	while (getline(&line_buf, &line_len, cfg) != -1) {
		char *ptr;
		char *src = line_buf;
		char *dst;
		while (*src && isspace((unsigned char)*src))
			src++;
		if (!*src)
			continue;
		if (src[0] == '#')
			continue;

		ptr = src;
		while (*ptr && !isspace((unsigned char)*ptr))
			ptr++;
		*ptr++ = '\0';
		if (strcmp(src, name) != 0)
			continue;

		dst = ptr;
		while (*dst && isspace((unsigned char)*dst))
			dst++;
		if (!*dst)
			continue;
		ptr = dst;
		while (*ptr && !isspace((unsigned char)*ptr))
			ptr++;
		*ptr = '\0';

		if (!strchr(dst, '/'))
			sub = strdup(dst);

		break;
	}
	free(line_buf);
	fclose(cfg);
out:
	if (!sub)
		sub = strdup(name);
	return sub;
}

static int bool_open(const char *name, int flag) {
	char *fname = NULL;
	char *alt_name = NULL;
	size_t len;
	int fd = -1;
	int ret;
	char *ptr;

	if (!name || strchr(name, '/')) {
		errno = EINVAL;
		return -1;
	}

	/* note the 'sizeof' gets us enough room for the '\0' */
	len = strlen(name) + strlen(selinux_mnt) + sizeof(SELINUX_BOOL_DIR);
	fname = malloc(sizeof(char) * len);
	if (!fname)
		return -1;

	ret = snprintf(fname, len, "%s%s%s", selinux_mnt, SELINUX_BOOL_DIR, name);
	if (ret < 0 || (size_t)ret >= len)
		goto out;

	fd = open(fname, flag);
	if (fd >= 0 || errno != ENOENT)
		goto out;

	alt_name = selinux_boolean_sub(name);
	if (!alt_name)
		goto out;

	/* note the 'sizeof' gets us enough room for the '\0' */
	len = strlen(alt_name) + strlen(selinux_mnt) + sizeof(SELINUX_BOOL_DIR);
	ptr = realloc(fname, len);
	if (!ptr)
		goto out;
	fname = ptr;

	ret = snprintf(fname, len, "%s%s%s", selinux_mnt, SELINUX_BOOL_DIR, alt_name);
	if (ret < 0 || (size_t)ret >= len)
		goto out;

	fd = open(fname, flag);
out:
	free(fname);
	free(alt_name);

	return fd;
}

#define STRBUF_SIZE 3
static int get_bool_value(const char *name, char **buf)
{
	int fd, len;
	int errno_tmp;

	if (!selinux_mnt) {
		errno = ENOENT;
		return -1;
	}

	*buf = malloc(sizeof(char) * (STRBUF_SIZE + 1));
	if (!*buf)
		return -1;

	(*buf)[STRBUF_SIZE] = 0;

	fd = bool_open(name, O_RDONLY | O_CLOEXEC);
	if (fd < 0)
		goto out_err;

	len = read(fd, *buf, STRBUF_SIZE);
	errno_tmp = errno;
	close(fd);
	errno = errno_tmp;
	if (len != STRBUF_SIZE)
		goto out_err;

	return 0;
out_err:
	free(*buf);
	return -1;
}

int security_get_boolean_pending(const char *name)
{
	char *buf;
	int val;

	if (get_bool_value(name, &buf))
		return -1;

	if (atoi(&buf[1]))
		val = 1;
	else
		val = 0;
	free(buf);
	return val;
}

int security_get_boolean_active(const char *name)
{
	char *buf;
	int val;

	if (get_bool_value(name, &buf))
		return -1;

	buf[1] = '\0';
	if (atoi(buf))
		val = 1;
	else
		val = 0;
	free(buf);
	return val;
}

int security_set_boolean(const char *name, int value)
{
	int fd, ret;
	char buf[2];

	if (!selinux_mnt) {
		errno = ENOENT;
		return -1;
	}
	if (value < 0 || value > 1) {
		errno = EINVAL;
		return -1;
	}

	fd = bool_open(name, O_WRONLY | O_CLOEXEC);
	if (fd < 0)
		return -1;

	if (value)
		buf[0] = '1';
	else
		buf[0] = '0';
	buf[1] = '\0';

	ret = write(fd, buf, 2);
	close(fd);

	if (ret > 0)
		return 0;
	else
		return -1;
}

int security_commit_booleans(void)
{
	int fd, ret;
	char buf[2];
	char path[PATH_MAX];

	if (!selinux_mnt) {
		errno = ENOENT;
		return -1;
	}

	snprintf(path, sizeof path, "%s/commit_pending_bools", selinux_mnt);
	fd = open(path, O_WRONLY | O_CLOEXEC);
	if (fd < 0)
		return -1;

	buf[0] = '1';
	buf[1] = '\0';

	ret = write(fd, buf, 2);
	close(fd);

	if (ret > 0)
		return 0;
	else
		return -1;
}

static void rollback(SELboolean * boollist, int end)
{
	int i;

	for (i = 0; i < end; i++)
		security_set_boolean(boollist[i].name,
				     security_get_boolean_active(boollist[i].
								 name));
}

int security_set_boolean_list(size_t boolcnt, SELboolean * boollist,
			      int permanent)
{

	size_t i;
	for (i = 0; i < boolcnt; i++) {
		boollist[i].value = !!boollist[i].value;
		if (security_set_boolean(boollist[i].name, boollist[i].value)) {
			rollback(boollist, i);
			return -1;
		}
	}

	/* OK, let's do the commit */
	if (security_commit_booleans()) {
		return -1;
	}

	/* Return error as flag no longer used */
	if (permanent)
		return -1;

	return 0;
}

/* This function is deprecated */
int security_load_booleans(char *path __attribute__((unused)))
{
	return -1;
}
#else

#include <stdlib.h>
#include "selinux_internal.h"

int security_set_boolean_list(size_t boolcnt __attribute__((unused)),
	SELboolean * boollist __attribute__((unused)),
	int permanent __attribute__((unused)))
{
	return -1;
}

int security_load_booleans(char *path __attribute__((unused)))
{
	return -1;
}

int security_get_boolean_names(char ***names __attribute__((unused)),
	int *len __attribute__((unused)))
{
	return -1;
}

int security_get_boolean_pending(const char *name __attribute__((unused)))
{
	return -1;
}

int security_get_boolean_active(const char *name __attribute__((unused)))
{
	return -1;
}

int security_set_boolean(const char *name __attribute__((unused)),
	int value __attribute__((unused)))
{
	return -1;
}

int security_commit_booleans(void)
{
	return -1;
}

char *selinux_boolean_sub(const char *name __attribute__((unused)))
{
	return NULL;
}
#endif

