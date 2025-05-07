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
#include "callbacks.h"

int security_compute_user_raw(const char * scon,
			      const char *user, char *** con)
{
	char path[PATH_MAX];
	char **ary;
	char *buf, *ptr;
	size_t size;
	int fd, ret;
	unsigned int i, nel;

	if (!selinux_mnt) {
		errno = ENOENT;
		return -1;
	}

	selinux_log(SELINUX_WARNING, "Direct use of security_compute_user() is deprecated, switch to get_ordered_context_list()\n");

	snprintf(path, sizeof path, "%s/user", selinux_mnt);
	fd = open(path, O_RDWR | O_CLOEXEC);
	if (fd < 0)
		return -1;

	size = selinux_page_size;
	buf = malloc(size);
	if (!buf) {
		ret = -1;
		goto out;
	}

	ret = snprintf(buf, size, "%s %s", scon, user);
	if (ret < 0 || (size_t)ret >= size) {
		errno = EOVERFLOW;
		ret = -1;
		goto out2;
	}

	ret = write(fd, buf, strlen(buf));
	if (ret < 0)
		goto out2;

	memset(buf, 0, size);
	ret = read(fd, buf, size - 1);
	if (ret < 0)
		goto out2;

	if (sscanf(buf, "%u", &nel) != 1) {
		ret = -1;
		goto out2;
	}

	ary = malloc((nel + 1) * sizeof(char *));
	if (!ary) {
		ret = -1;
		goto out2;
	}

	ptr = buf + strlen(buf) + 1;
	for (i = 0; i < nel; i++) {
		ary[i] = strdup(ptr);
		if (!ary[i]) {
			freeconary(ary);
			ret = -1;
			goto out2;
		}
		ptr += strlen(ptr) + 1;
	}
	ary[nel] = NULL;
	*con = ary;
	ret = 0;
      out2:
	free(buf);
      out:
	close(fd);
	return ret;
}


int security_compute_user(const char * scon,
			  const char *user, char *** con)
{
	int ret;
	char * rscon;

	if (selinux_trans_to_raw_context(scon, &rscon))
		return -1;

	IGNORE_DEPRECATED_DECLARATION_BEGIN
	ret = security_compute_user_raw(rscon, user, con);
	IGNORE_DEPRECATED_DECLARATION_END

	freecon(rscon);
	if (!ret) {
		char **ptr, *tmpcon;
		for (ptr = *con; *ptr; ptr++) {
			if (selinux_raw_to_trans_context(*ptr, &tmpcon)) {
				freeconary(*con);
				*con = NULL;
				return -1;
			}
			freecon(*ptr);
			*ptr = tmpcon;
		}
	}

	return ret;
}

