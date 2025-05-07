#include <unistd.h>
#include <sys/types.h>
#include <fcntl.h>
#include <stdlib.h>
#include <errno.h>
#include <string.h>
#include "selinux_internal.h"
#include "policy.h"
#include <stdio.h>
#include <limits.h>

int security_deny_unknown(void)
{
	int fd, ret, deny_unknown = 0;
	char path[PATH_MAX];
	char buf[20];

	if (!selinux_mnt) {
		errno = ENOENT;
		return -1;
	}

	snprintf(path, sizeof(path), "%s/deny_unknown", selinux_mnt);
	fd = open(path, O_RDONLY | O_CLOEXEC);
	if (fd < 0)
		return -1;

	memset(buf, 0, sizeof(buf));
	ret = read(fd, buf, sizeof(buf) - 1);
	close(fd);
	if (ret < 0)
		return -1;

	if (sscanf(buf, "%d", &deny_unknown) != 1)
		return -1;

	return deny_unknown;
}

