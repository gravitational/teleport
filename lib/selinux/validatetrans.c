#include <unistd.h>
#include <sys/types.h>
#include <fcntl.h>
#include <stdlib.h>
#include <stdio.h>
#include <errno.h>
#include <string.h>
#include <limits.h>
#include "selinux_internal.h"
#include "policy.h"
#include "mapping.h"

int security_validatetrans_raw(const char *scon,
			       const char *tcon,
			       security_class_t tclass,
			       const char *newcon)
{
	char path[PATH_MAX];
	char *buf = NULL;
	int size, bufsz;
	int fd, ret = -1;
	errno = ENOENT;

	if (!selinux_mnt) {
		return -1;
	}

	snprintf(path, sizeof path, "%s/validatetrans", selinux_mnt);
	fd = open(path, O_WRONLY | O_CLOEXEC);
	if (fd < 0) {
		return -1;
	}

	errno = EINVAL;
	size = selinux_page_size;
	buf = malloc(size);
	if (!buf) {
		goto out;
	}

	bufsz = snprintf(buf, size, "%s %s %hu %s", scon, tcon, unmap_class(tclass), newcon);
	if (bufsz >= size || bufsz < 0) {
		// It got truncated or there was an encoding error
		goto out;
	}

	// clear errno for write()
	errno = 0;
	ret = write(fd, buf, strlen(buf));
	if (ret > 0) {
		// The kernel returns the bytes written on success, not 0 as noted in the commit message
		ret = 0;
	}
out:
	free(buf);
	close(fd);
	return ret;
}


int security_validatetrans(const char *scon,
			   const char *tcon,
			   security_class_t tclass,
			   const char *newcon)
{
	int ret = -1;
	char *rscon = NULL;
	char *rtcon = NULL;
	char *rnewcon = NULL;

	if (selinux_trans_to_raw_context(scon, &rscon)) {
		goto out;
	}

	if (selinux_trans_to_raw_context(tcon, &rtcon)) {
		goto out;
	}

	if (selinux_trans_to_raw_context(newcon, &rnewcon)) {
		goto out;
	}

	ret = security_validatetrans_raw(rscon, rtcon, tclass, rnewcon);

out:
	freecon(rnewcon);
	freecon(rtcon);
	freecon(rscon);

	return ret;
}

