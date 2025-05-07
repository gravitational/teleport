/*-*- Mode: C; c-basic-offset: 8; indent-tabs-mode: nil -*-*/
#include <unistd.h>
#include <sys/types.h>
#include <stdlib.h>
#include <errno.h>
#include "selinux_internal.h"
#include <selinux/avc.h>
#include "avc_internal.h"

static pthread_once_t once = PTHREAD_ONCE_INIT;
static int selinux_enabled;

static void avc_init_once(void)
{
	selinux_enabled = is_selinux_enabled();
	if (selinux_enabled == 1) {
		if (avc_open(NULL, 0))
			return;
	}
}

int selinux_check_access(const char *scon, const char *tcon, const char *class, const char *perm, void *aux) {
	int rc;
	security_id_t scon_id;
	security_id_t tcon_id;
	security_class_t sclass;
	access_vector_t av;

	__selinux_once(once, avc_init_once);

	if (selinux_enabled != 1)
		return 0;

	rc = avc_context_to_sid(scon, &scon_id);
	if (rc < 0)
		return rc;

	rc = avc_context_to_sid(tcon, &tcon_id);
	if (rc < 0)
		return rc;

	(void) selinux_status_updated();

       sclass = string_to_security_class(class);
       if (sclass == 0) {
	       rc = errno;
	       avc_log(SELINUX_ERROR, "Unknown class %s", class);
	       if (security_deny_unknown() == 0)
		       return 0;
	       errno = rc;
	       return -1;
       }

       av = string_to_av_perm(sclass, perm);
       if (av == 0) {
	       rc = errno;
	       avc_log(SELINUX_ERROR, "Unknown permission %s for class %s", perm, class);
	       if (security_deny_unknown() == 0)
		       return 0;
	       errno = rc;
	       return -1;
       }

       return avc_has_perm (scon_id, tcon_id, sclass, av, NULL, aux);
}

static int selinux_check_passwd_access_internal(access_vector_t requested)
{
	int status = -1;
	char *user_context;
	if (is_selinux_enabled() == 0)
		return 0;
	if (getprevcon_raw(&user_context) == 0) {
		security_class_t passwd_class;
		struct av_decision avd;
		int retval;

		passwd_class = string_to_security_class("passwd");
		if (passwd_class == 0) {
			freecon(user_context);
			if (security_deny_unknown() == 0)
				return 0;
			return -1;
		}

		retval = security_compute_av_raw(user_context,
						     user_context,
						     passwd_class,
						     requested,
						     &avd);

		if ((retval == 0) && ((requested & avd.allowed) == requested)) {
			status = 0;
		}
		freecon(user_context);
	}

	if (status != 0 && security_getenforce() == 0)
		status = 0;

	return status;
}

int selinux_check_passwd_access(access_vector_t requested) {
	return selinux_check_passwd_access_internal(requested);
}

int checkPasswdAccess(access_vector_t requested)
{
	return selinux_check_passwd_access_internal(requested);
}
