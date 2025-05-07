#include <unistd.h>
#include "selinux_internal.h"
#include <stdlib.h>
#include <errno.h>

void freecon(char * con)
{
	free(con);
}

