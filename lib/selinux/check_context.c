#include <unistd.h>
#include <sys/types.h>
#include <fcntl.h>
#include <stdlib.h>
#include <errno.h>
#include <string.h>
#include <stdio.h>
#include "selinux_internal.h"
#include "policy.h"
#include <limits.h>

int security_check_context_raw(const char * con)
{
	char path[PATH_MAX];
	int fd, ret;

	if (!selinux_mnt) {
		errno = ENOENT;
		return -1;
	}

	snprintf(path, sizeof path, "%s/context", selinux_mnt);
	fd = open(path, O_RDWR | O_CLOEXEC);
	if (fd < 0)
		return -1;

	ret = write(fd, con, strlen(con) + 1);
	close(fd);
	if (ret < 0)
		return -1;
	return 0;
}


int security_check_context(const char * con)
{
	int ret;
	char * rcon;

	if (selinux_trans_to_raw_context(con, &rcon))
		return -1;

	ret = security_check_context_raw(rcon);

	freecon(rcon);

	return ret;
}

