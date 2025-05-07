#include <unistd.h>
#include <fcntl.h>
#include <string.h>
#include <stdlib.h>
#include <errno.h>
#include <stdio.h>
#include <sys/xattr.h>
#include "selinux_internal.h"
#include "policy.h"

static int fsetxattr_wrapper(int fd, const char* name, const void* value, size_t size, int flags) {
	char buf[40];
	int rc, fd_flag, saved_errno = errno;

	rc = fsetxattr(fd, name, value, size, flags);
	if (rc == 0 || errno != EBADF)
		return rc;

	/* Emulate O_PATH support */
	fd_flag = fcntl(fd, F_GETFL);
	if (fd_flag == -1 || (fd_flag & O_PATH) == 0) {
		errno = EBADF;
		return -1;
	}

	snprintf(buf, sizeof(buf), "/proc/self/fd/%d", fd);
	errno = saved_errno;
	rc = setxattr(buf, name, value, size, flags);
	if (rc < 0 && errno == ENOENT)
		errno = EBADF;
	return rc;
}

int fsetfilecon_raw(int fd, const char * context)
{
	int rc = fsetxattr_wrapper(fd, XATTR_NAME_SELINUX, context, strlen(context) + 1,
			 0);
	if (rc < 0 && errno == ENOTSUP) {
		char * ccontext = NULL;
		int err = errno;
		if ((fgetfilecon_raw(fd, &ccontext) >= 0) &&
		    (strcmp(context,ccontext) == 0)) {
			rc = 0;
		} else {
			errno = err;
		}
		freecon(ccontext);
	}
	return rc;
}


int fsetfilecon(int fd, const char *context)
{
	int ret;
	char * rcontext;

	if (selinux_trans_to_raw_context(context, &rcontext))
		return -1;

	ret = fsetfilecon_raw(fd, rcontext);

	freecon(rcontext);

	return ret;
}
