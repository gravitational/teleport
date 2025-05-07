#include <unistd.h>
#include <sys/types.h>
#include <fcntl.h>
#include <stdlib.h>
#include <stdio.h>
#include <errno.h>
#include <string.h>
#include <limits.h>
#include <ctype.h>
#include "selinux_internal.h"
#include "policy.h"
#include "mapping.h"

static int object_name_encode(const char *objname, char *buffer, size_t buflen)
{
	unsigned char code;
	size_t	offset = 0;

	if (buflen - offset < 1)
		return -1;
	buffer[offset++] = ' ';

	do {
		code = *objname++;

		if (isalnum(code) || code == '\0' || code == '-' ||
		    code == '.' || code == '_' || code == '~') {
			if (buflen - offset < 1)
				return -1;
			buffer[offset++] = code;
		} else if (code == ' ') {
			if (buflen - offset < 1)
				return -1;
			buffer[offset++] = '+';
		} else {
			static const char *const table = "0123456789ABCDEF";
			int	l = (code & 0x0f);
			int	h = (code & 0xf0) >> 4;

			if (buflen - offset < 3)
				return -1;
			buffer[offset++] = '%';
			buffer[offset++] = table[h];
			buffer[offset++] = table[l];
		}
	} while (code != '\0');

	return 0;
}

int security_compute_create_name_raw(const char * scon,
				     const char * tcon,
				     security_class_t tclass,
				     const char *objname,
				     char ** newcon)
{
	char path[PATH_MAX];
	char *buf;
	size_t size;
	int fd, ret, len;

	if (!selinux_mnt) {
		errno = ENOENT;
		return -1;
	}

	snprintf(path, sizeof path, "%s/create", selinux_mnt);
	fd = open(path, O_RDWR | O_CLOEXEC);
	if (fd < 0)
		return -1;

	size = selinux_page_size;
	buf = malloc(size);
	if (!buf) {
		ret = -1;
		goto out;
	}

	len = snprintf(buf, size, "%s %s %hu",
		       scon, tcon, unmap_class(tclass));
	if (len < 0 || (size_t)len >= size) {
		errno = EOVERFLOW;
		ret = -1;
		goto out2;
	}

	if (objname &&
	    object_name_encode(objname, buf + len, size - len) < 0) {
		errno = ENAMETOOLONG;
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
	if (!(*newcon)) {
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

int security_compute_create_raw(const char * scon,
				const char * tcon,
				security_class_t tclass,
				char ** newcon)
{
	return security_compute_create_name_raw(scon, tcon, tclass,
						NULL, newcon);
}

int security_compute_create_name(const char * scon,
				 const char * tcon,
				 security_class_t tclass,
				 const char *objname,
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

	ret = security_compute_create_name_raw(rscon, rtcon, tclass,
					       objname, &rnewcon);
	freecon(rscon);
	freecon(rtcon);
	if (!ret) {
		ret = selinux_raw_to_trans_context(rnewcon, newcon);
		freecon(rnewcon);
	}

	return ret;
}

int security_compute_create(const char * scon,
				const char * tcon,
			    security_class_t tclass,
				char ** newcon)
{
	return security_compute_create_name(scon, tcon, tclass, NULL, newcon);
}
