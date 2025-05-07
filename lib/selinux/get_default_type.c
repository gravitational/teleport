#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include "get_default_type_internal.h"
#include <errno.h>

static int find_default_type(FILE * fp, const char *role, char **type);

int get_default_type(const char *role, char **type)
{
	FILE *fp = NULL;

	fp = fopen(selinux_default_type_path(), "re");
	if (!fp)
		return -1;

	if (find_default_type(fp, role, type) < 0) {
		fclose(fp);
		return -1;
	}

	fclose(fp);
	return 0;
}

static int find_default_type(FILE * fp, const char *role, char **type)
{
	char buf[250];
	const char *ptr = "", *end;
	char *t;
	size_t len;
	int found = 0;

	len = strlen(role);
	while (!feof_unlocked(fp)) {
		if (!fgets_unlocked(buf, sizeof buf, fp)) {
			errno = EINVAL;
			return -1;
		}
		if (buf[strlen(buf) - 1])
			buf[strlen(buf) - 1] = 0;

		ptr = buf;
		while (*ptr && isspace((unsigned char)*ptr))
			ptr++;
		if (!(*ptr))
			continue;

		if (!strncmp(role, ptr, len)) {
			end = ptr + len;
			if (*end == ':') {
				found = 1;
				ptr = ++end;
				break;
			}
		}
	}

	if (!found) {
		errno = EINVAL;
		return -1;
	}

	t = strndup(ptr, strlen(buf) - len - 1);
	if (!t)
		return -1;
	*type = t;
	return 0;
}
