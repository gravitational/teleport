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

int security_setenforce(int value)
{
	int fd, ret;
	char path[PATH_MAX];
	char buf[20];

	if (!selinux_mnt) {
		errno = ENOENT;
		return -1;
	}

	snprintf(path, sizeof path, "%s/enforce", selinux_mnt);
	fd = open(path, O_RDWR | O_CLOEXEC);
	if (fd < 0)
		return -1;

	snprintf(buf, sizeof buf, "%d", value);
	ret = write(fd, buf, strlen(buf));
	close(fd);
	if (ret < 0)
		return -1;

	return 0;
}

