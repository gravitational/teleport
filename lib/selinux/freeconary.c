#include <unistd.h>
#include "selinux_internal.h"
#include <stdlib.h>
#include <errno.h>

void freeconary(char ** con)
{
	char **ptr;

	if (!con)
		return;

	for (ptr = con; *ptr; ptr++) {
		free(*ptr);
	}
	free(con);
}

