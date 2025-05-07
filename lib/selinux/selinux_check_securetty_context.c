#include <unistd.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <ctype.h>
#include "selinux_internal.h"
#include "context_internal.h"

int selinux_check_securetty_context(const char * tty_context)
{
	char *line = NULL;
	char *start, *end = NULL;
	size_t line_len = 0;
	ssize_t len;
	int found = -1;
	FILE *fp;
	fp = fopen(selinux_securetty_types_path(), "re");
	if (fp) {
		context_t con = context_new(tty_context);
		if (con) {
			const char *type = context_type_get(con);
			while ((len = getline(&line, &line_len, fp)) != -1) {

				if (line[len - 1] == '\n')
					line[len - 1] = 0;

				/* Skip leading whitespace. */
				start = line;
				while (*start && isspace((unsigned char)*start))
					start++;
				if (!(*start))
					continue;

				end = start;
				while (*end && !isspace((unsigned char)*end))
					end++;
				if (*end)
					*end++ = 0;
				if (!strcmp(type, start)) {
					found = 0;
					break;
				}
			}
			free(line);
			context_free(con);
		}
		fclose(fp);
	}

	return found;
}

