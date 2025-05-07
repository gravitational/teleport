#include <unistd.h>
#include <fcntl.h>
#include <string.h>
#include <stdlib.h>
#include <errno.h>
#include <stdio.h>
#include <sys/xattr.h>
#include "selinux_internal.h"
#include "policy.h"

static ssize_t fgetxattr_wrapper(int fd, const char *name, void *value, size_t size) {
	char buf[40];
	int fd_flag, saved_errno = errno;
	ssize_t ret;

	ret = fgetxattr(fd, name, value, size);
	if (ret != -1 || errno != EBADF)
		return ret;

	/* Emulate O_PATH support */
	fd_flag = fcntl(fd, F_GETFL);
	if (fd_flag == -1 || (fd_flag & O_PATH) == 0) {
		errno = EBADF;
		return -1;
	}

	snprintf(buf, sizeof(buf), "/proc/self/fd/%d", fd);
	errno = saved_errno;
	ret = getxattr(buf, name, value, size);
	if (ret < 0 && errno == ENOENT)
		errno = EBADF;
	return ret;
}

int fgetfilecon_raw(int fd, char ** context)
{
	char *buf;
	ssize_t size;
	ssize_t ret;

	size = INITCONTEXTLEN + 1;
	buf = calloc(1, size);
	if (!buf)
		return -1;

	ret = fgetxattr_wrapper(fd, XATTR_NAME_SELINUX, buf, size - 1);
	if (ret < 0 && errno == ERANGE) {
		char *newbuf;

		size = fgetxattr_wrapper(fd, XATTR_NAME_SELINUX, NULL, 0);
		if (size < 0)
			goto out;

		size++;
		newbuf = realloc(buf, size);
		if (!newbuf)
			goto out;

		buf = newbuf;
		memset(buf, 0, size);
		ret = fgetxattr_wrapper(fd, XATTR_NAME_SELINUX, buf, size - 1);
	}
      out:
	if (ret == 0) {
		/* Re-map empty attribute values to errors. */
		errno = ENOTSUP;
		ret = -1;
	}
	if (ret < 0)
		free(buf);
	else
		*context = buf;
	return ret;
}


int fgetfilecon(int fd, char ** context)
{
	char * rcontext = NULL;
	int ret;

	*context = NULL;

	ret = fgetfilecon_raw(fd, &rcontext);

	if (ret > 0) {
		ret = selinux_raw_to_trans_context(rcontext, context);
		freecon(rcontext);
	}

	if (ret >= 0 && *context)
		return strlen(*context) + 1;

	return ret;
}
