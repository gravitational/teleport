#include <unistd.h>
#include <sys/types.h>
#include <fcntl.h>
#include <stdlib.h>
#include <stdio.h>
#include <errno.h>
#include <string.h>
#include "selinux_internal.h"
#include "policy.h"
#include <limits.h>

#define SELINUX_INITCON_DIR "/initial_contexts/"

int security_get_initial_context_raw(const char * name, char ** con)
{
	char path[PATH_MAX];
	char *buf;
	size_t size;
	int fd, ret;

	if (!selinux_mnt) {
		errno = ENOENT;
		return -1;
	}

	if (strchr(name, '/')) {
		errno = EINVAL;
		return -1;
	}

	ret = snprintf(path, sizeof path, "%s%s%s", selinux_mnt, SELINUX_INITCON_DIR, name);
	if (ret < 0 || (size_t)ret >= sizeof path) {
		errno = EOVERFLOW;
		return -1;
	}

	fd = open(path, O_RDONLY | O_CLOEXEC);
	if (fd < 0)
		return -1;

	size = selinux_page_size;
	buf = calloc(1, size);
	if (!buf) {
		ret = -1;
		goto out;
	}
	ret = read(fd, buf, size - 1);
	if (ret < 0)
		goto out2;

	*con = strdup(buf);
	if (!(*con)) {
		ret = -1;
		goto out2;
	}
	ret = 0;
      out2:
	free(buf);
      out:
	close(fd);
	return ret;
}


int security_get_initial_context(const char * name, char ** con)
{
	int ret;
	char * rcon;

	ret = security_get_initial_context_raw(name, &rcon);
	if (!ret) {
		ret = selinux_raw_to_trans_context(rcon, con);
		freecon(rcon);
	}

	return ret;
}

