#include <unistd.h>
#include <fcntl.h>
#include <string.h>
#include <stdlib.h>
#include <errno.h>
#include <sys/socket.h>
#include "selinux_internal.h"
#include "policy.h"

#ifndef SO_PEERSEC
#define SO_PEERSEC 31
#endif

int getpeercon_raw(int fd, char ** context)
{
	char *buf;
	socklen_t size;
	ssize_t ret;

	size = INITCONTEXTLEN + 1;
	buf = calloc(1, size);
	if (!buf)
		return -1;

	ret = getsockopt(fd, SOL_SOCKET, SO_PEERSEC, buf, &size);
	if (ret < 0 && errno == ERANGE) {
		char *newbuf;

		newbuf = realloc(buf, size);
		if (!newbuf)
			goto out;

		buf = newbuf;
		memset(buf, 0, size);
		ret = getsockopt(fd, SOL_SOCKET, SO_PEERSEC, buf, &size);
	}
      out:
	if (ret < 0)
		free(buf);
	else
		*context = buf;
	return ret;
}


int getpeercon(int fd, char ** context)
{
	int ret;
	char * rcontext;

	ret = getpeercon_raw(fd, &rcontext);

	if (!ret) {
		ret = selinux_raw_to_trans_context(rcontext, context);
		freecon(rcontext);
	}

	return ret;
}
