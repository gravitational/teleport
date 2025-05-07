#include <unistd.h>
#include <fcntl.h>
#include <sys/stat.h>
#include <string.h>
#include "selinux_internal.h"
#include <stdio.h>
#include <stdlib.h>
#include <ctype.h>
#include <errno.h>
#include <limits.h>
#include <regex.h>
#include <stdarg.h>

int matchmediacon(const char *media, char ** con)
{
	const char *path = selinux_media_context_path();
	FILE *infile;
	char *ptr, *ptr2 = NULL;
	int found = 0;
	char current_line[PATH_MAX];
	if ((infile = fopen(path, "re")) == NULL)
		return -1;
	while (!feof_unlocked(infile)) {
		if (!fgets_unlocked(current_line, sizeof(current_line), infile)) {
			fclose(infile);
			return -1;
		}
		if (current_line[strlen(current_line) - 1])
			current_line[strlen(current_line) - 1] = 0;
		/* Skip leading whitespace before the partial context. */
		ptr = current_line;
		while (*ptr && isspace((unsigned char)*ptr))
			ptr++;

		if (!(*ptr))
			continue;

		/* Find the end of the media context. */
		ptr2 = ptr;
		while (*ptr2 && !isspace((unsigned char)*ptr2))
			ptr2++;
		if (!(*ptr2))
			continue;

		*ptr2++ = 0;
		if (strcmp(media, ptr) == 0) {
			found = 1;
			break;
		}
	}
	fclose(infile);
	if (!found)
		return -1;

	/* Skip whitespace. */
	while (*ptr2 && isspace((unsigned char)*ptr2))
		ptr2++;
	if (!(*ptr2)) {
		return -1;
	}

	if (selinux_raw_to_trans_context(ptr2, con)) {
		*con = NULL;
		return -1;
	}

	return 0;
}
