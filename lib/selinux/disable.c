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

int security_disable(void)
{
	int fd, ret;
	char path[PATH_MAX];
	char buf[20];

	if (!selinux_mnt) {
		errno = ENOENT;
		return -1;
	}

	snprintf(path, sizeof path, "%s/disable", selinux_mnt);
	fd = open(path, O_WRONLY | O_CLOEXEC);
	if (fd < 0)
		return -1;

	buf[0] = '1';
	buf[1] = '\0';
	ret = write(fd, buf, strlen(buf));
	close(fd);
	if (ret < 0)
		return -1;

	return 0;
}

