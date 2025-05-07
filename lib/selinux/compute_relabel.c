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

int security_compute_relabel_raw(const char * scon,
				 const char * tcon,
				 security_class_t tclass,
				 char ** newcon)
{
	char path[PATH_MAX];
	char *buf;
	size_t size;
	int fd, ret;

	if (!selinux_mnt) {
		errno = ENOENT;
		return -1;
	}

	snprintf(path, sizeof path, "%s/relabel", selinux_mnt);
	fd = open(path, O_RDWR | O_CLOEXEC);
	if (fd < 0)
		return -1;

	size = selinux_page_size;
	buf = malloc(size);
	if (!buf) {
		ret = -1;
		goto out;
	}

	ret = snprintf(buf, size, "%s %s %hu", scon, tcon, unmap_class(tclass));
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

	*newcon = strdup(buf);
	if (!*newcon) {
		ret = -1;
		goto out2;
	}
	ret = 0;
      out2:
	free(buf);
      out:
	close(fd);
	return ret;
}


int security_compute_relabel(const char * scon,
			     const char * tcon,
			     security_class_t tclass,
			     char ** newcon)
{
	int ret;
	char * rscon;
	char * rtcon;
	char * rnewcon;

	if (selinux_trans_to_raw_context(scon, &rscon))
		return -1;
	if (selinux_trans_to_raw_context(tcon, &rtcon)) {
		freecon(rscon);
		return -1;
	}

	ret = security_compute_relabel_raw(rscon, rtcon, tclass, &rnewcon);

	freecon(rscon);
	freecon(rtcon);
	if (!ret) {
		ret = selinux_raw_to_trans_context(rnewcon, newcon);
		freecon(rnewcon);
	}

	return ret;
}
