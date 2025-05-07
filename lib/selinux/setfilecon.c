#include <unistd.h>
#include <fcntl.h>
#include <string.h>
#include <stdlib.h>
#include <errno.h>
#include <sys/xattr.h>
#include "selinux_internal.h"
#include "policy.h"

int setfilecon_raw(const char *path, const char * context)
{
	int rc = setxattr(path, XATTR_NAME_SELINUX, context, strlen(context) + 1,
			0);
	if (rc < 0 && errno == ENOTSUP) {
		char * ccontext = NULL;
		int err = errno;
		if ((getfilecon_raw(path, &ccontext) >= 0) &&
		    (strcmp(context,ccontext) == 0)) {
			rc = 0;
		} else {
			errno = err;
		}
		freecon(ccontext);
	}
	return rc;
}


int setfilecon(const char *path, const char *context)
{
	int ret;
	char * rcontext;

	if (selinux_trans_to_raw_context(context, &rcontext))
		return -1;

	ret = setfilecon_raw(path, rcontext);

	freecon(rcontext);

	return ret;
}
