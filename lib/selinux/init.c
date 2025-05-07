#include <unistd.h>
#include <fcntl.h>
#include <string.h>
#include <stdlib.h>
#include <errno.h>
#include <ctype.h>
#include <stdio.h>
#include <stdio_ext.h>
#include <dlfcn.h>
#include <sys/statvfs.h>
#include <sys/vfs.h>
#include <stdint.h>
#include <limits.h>

#include "policy.h"
#include "selinux_internal.h"
#include "setrans_internal.h"

char *selinux_mnt = NULL;
int selinux_page_size = 0;

int has_selinux_config = 0;

/* Verify the mount point for selinux file system has a selinuxfs.
   If the file system:
   * Exist,
   * Is mounted with an selinux file system,
   * The file system is read/write
   * then set this as the default file system.
*/
static int verify_selinuxmnt(const char *mnt)
{
	struct statfs sfbuf;
	int rc;

	do {
		rc = statfs(mnt, &sfbuf);
	} while (rc < 0 && errno == EINTR);
	if (rc == 0) {
		if ((uint32_t)sfbuf.f_type == (uint32_t)SELINUX_MAGIC) {
			struct statvfs vfsbuf;
			rc = statvfs(mnt, &vfsbuf);
			if (rc == 0) {
				if (!(vfsbuf.f_flag & ST_RDONLY)) {
					set_selinuxmnt(mnt);
				}
				return 0;
			}
		}
	}

	return -1;
}

int selinuxfs_exists(void)
{
	int exists = 0;
	FILE *fp = NULL;
	char *buf = NULL;
	size_t len;
	ssize_t num;

	fp = fopen("/proc/filesystems", "re");
	if (!fp)
		return 1; /* Fail as if it exists */
	__fsetlocking(fp, FSETLOCKING_BYCALLER);

	num = getline(&buf, &len, fp);
	while (num != -1) {
		if (strstr(buf, SELINUXFS)) {
			exists = 1;
			break;
		}
		num = getline(&buf, &len, fp);
	}

	free(buf);
	fclose(fp);
	return exists;
}

static void init_selinuxmnt(void)
{
	char *buf = NULL, *p;
	FILE *fp = NULL;
	size_t len;
	ssize_t num;

	if (selinux_mnt)
		return;

	if (verify_selinuxmnt(SELINUXMNT) == 0) return;

	if (verify_selinuxmnt(OLDSELINUXMNT) == 0) return;

	/* Drop back to detecting it the long way. */
	if (!selinuxfs_exists())
		goto out;

	/* At this point, the usual spot doesn't have an selinuxfs so
	 * we look around for it */
	fp = fopen("/proc/mounts", "re");
	if (!fp)
		goto out;

	__fsetlocking(fp, FSETLOCKING_BYCALLER);
	while ((num = getline(&buf, &len, fp)) != -1) {
		char *tmp;
		p = strchr(buf, ' ');
		if (!p)
			goto out;
		p++;
		tmp = strchr(p, ' ');
		if (!tmp)
			goto out;
		if (!strncmp(tmp + 1, SELINUXFS" ", strlen(SELINUXFS)+1)) {
			*tmp = '\0';
			break;
		}
	}

	/* If we found something, dup it */
	if (num > 0)
		verify_selinuxmnt(p);

      out:
	free(buf);
	if (fp)
		fclose(fp);
	return;
}

void fini_selinuxmnt(void)
{
	free(selinux_mnt);
	selinux_mnt = NULL;
}


void set_selinuxmnt(const char *mnt)
{
	selinux_mnt = strdup(mnt);
}


static void init_lib(void) __attribute__ ((constructor));
static void init_lib(void)
{
	selinux_page_size = sysconf(_SC_PAGE_SIZE);
	init_selinuxmnt();
#ifndef ANDROID
	has_selinux_config = (access(SELINUXCONFIG, F_OK) == 0);
#endif
}

static void fini_lib(void) __attribute__ ((destructor));
static void fini_lib(void)
{
	fini_selinuxmnt();
}
