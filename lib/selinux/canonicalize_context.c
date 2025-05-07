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

int security_canonicalize_context_raw(const char * con,
				      char ** canoncon)
{
	char path[PATH_MAX];
	char *buf;
	size_t size;
	int fd, ret;

	if (!selinux_mnt) {
		errno = ENOENT;
		return -1;
	}

	snprintf(path, sizeof path, "%s/context", selinux_mnt);
	fd = open(path, O_RDWR | O_CLOEXEC);
	if (fd < 0)
		return -1;

	size = selinux_page_size;
	buf = malloc(size);
	if (!buf) {
		ret = -1;
		goto out;
	}
	if (strlcpy(buf, con, size) >= size) {
		errno = EOVERFLOW;
		ret = -1;
		goto out2;
	}

	ret = write(fd, buf, strlen(buf) + 1);
	if (ret < 0)
		goto out2;

	memset(buf, 0, size);
	ret = read(fd, buf, size - 1);
	if (ret < 0 && errno == EINVAL) {
		/* Fall back to the original context for kernels
		   that do not support the extended interface. */
		strncpy(buf, con, size);
	}

	*canoncon = strdup(buf);
	if (!(*canoncon)) {
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


int security_canonicalize_context(const char * con,
				      char ** canoncon)
{
	int ret;
	char * rcon;
	char * rcanoncon;

	if (selinux_trans_to_raw_context(con, &rcon))
		return -1;

	ret = security_canonicalize_context_raw(rcon, &rcanoncon);

	freecon(rcon);
	if (!ret) {
		ret = selinux_raw_to_trans_context(rcanoncon, canoncon);
		freecon(rcanoncon);
	}

	return ret;
}

