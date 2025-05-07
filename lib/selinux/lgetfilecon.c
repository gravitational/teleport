#include <unistd.h>
#include <fcntl.h>
#include <string.h>
#include <stdlib.h>
#include <errno.h>
#include <sys/xattr.h>
#include "selinux_internal.h"
#include "policy.h"

int lgetfilecon_raw(const char *path, char ** context)
{
	char *buf;
	ssize_t size;
	ssize_t ret;

	size = INITCONTEXTLEN + 1;
	buf = calloc(1, size);
	if (!buf)
		return -1;

	ret = lgetxattr(path, XATTR_NAME_SELINUX, buf, size - 1);
	if (ret < 0 && errno == ERANGE) {
		char *newbuf;

		size = lgetxattr(path, XATTR_NAME_SELINUX, NULL, 0);
		if (size < 0)
			goto out;

		size++;
		newbuf = realloc(buf, size);
		if (!newbuf)
			goto out;

		buf = newbuf;
		memset(buf, 0, size);
		ret = lgetxattr(path, XATTR_NAME_SELINUX, buf, size - 1);
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


int lgetfilecon(const char *path, char ** context)
{
	int ret;
	char * rcontext = NULL;

	*context = NULL;

	ret = lgetfilecon_raw(path, &rcontext);

	if (ret > 0) {
		ret = selinux_raw_to_trans_context(rcontext, context);
		freecon(rcontext);
	}

	if (ret >= 0 && *context)
		return strlen(*context) + 1;
	return ret;
}
