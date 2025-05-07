#include <unistd.h>
#include <fcntl.h>
#include <string.h>
#include "selinux_internal.h"
#include "context_internal.h"

int setexecfilecon(const char *filename, const char *fallback_type)
{
	char * mycon = NULL, *fcon = NULL, *newcon = NULL;
	context_t con = NULL;
	int rc = 0;

	if (is_selinux_enabled() < 1)
		return 0;

	rc = getcon(&mycon);
	if (rc < 0)
		goto out;

	rc = getfilecon(filename, &fcon);
	if (rc < 0)
		goto out;

	rc = security_compute_create(mycon, fcon, string_to_security_class("process"), &newcon);
	if (rc < 0)
		goto out;

	if (!strcmp(mycon, newcon)) {
		/* No default transition, use fallback_type for now. */
		rc = -1;
		con = context_new(mycon);
		if (!con)
			goto out;
		if (context_type_set(con, fallback_type))
			goto out;
		freecon(newcon);
		newcon = context_to_str(con);
		if (!newcon)
			goto out;
	}

	rc = setexeccon(newcon);
      out:

	if (rc < 0 && security_getenforce() == 0)
		rc = 0;

	context_free(con);
	freecon(newcon);
	freecon(fcon);
	freecon(mycon);
	return rc < 0 ? rc : 0;
}

#ifndef DISABLE_RPM
int rpm_execcon(unsigned int verified __attribute__ ((unused)),
		const char *filename, char *const argv[], char *const envp[])
{
	int rc;

	rc = setexecfilecon(filename, "rpm_script_t");
	if (rc < 0)
		return rc;

	return execve(filename, argv, envp);
}
#endif
