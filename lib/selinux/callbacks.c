/*
 * User-supplied callbacks and default implementations.
 * Class and permission mappings.
 */

#include <stdio.h>
#include <stdlib.h>
#include <stdarg.h>
#include <errno.h>
#include <selinux/selinux.h>
#include "callbacks.h"

pthread_mutex_t log_mutex = PTHREAD_MUTEX_INITIALIZER;

/* default implementations */
static int __attribute__ ((format(printf, 2, 3)))
default_selinux_log(int type __attribute__((unused)), const char *fmt, ...)
{
	int rc;
	va_list ap;
	va_start(ap, fmt);
	rc = vfprintf(stderr, fmt, ap);
	va_end(ap);
	return rc;
}

static int
default_selinux_audit(void *ptr __attribute__((unused)),
		      security_class_t cls __attribute__((unused)),
		      char *buf __attribute__((unused)),
		      size_t len __attribute__((unused)))
{
	return 0;
}

static int
default_selinux_validate(char **ctx)
{
#ifndef BUILD_HOST
	return security_check_context(*ctx);
#else
	(void) ctx;
	return 0;
#endif
}

static int
default_selinux_setenforce(int enforcing __attribute__((unused)))
{
	return 0;
}

static int
default_selinux_policyload(int seqno __attribute__((unused)))
{
	return 0;
}

/* callback pointers */
int __attribute__ ((format(printf, 2, 3)))
(*selinux_log_direct)(int, const char *, ...) =
	default_selinux_log;

int
(*selinux_audit) (void *, security_class_t, char *, size_t) =
	default_selinux_audit;

int
(*selinux_validate)(char **ctx) =
	default_selinux_validate;

int
(*selinux_netlink_setenforce) (int enforcing) =
	default_selinux_setenforce;

int
(*selinux_netlink_policyload) (int seqno) =
	default_selinux_policyload;

/* callback setting function */
void
selinux_set_callback(int type, union selinux_callback cb)
{
	switch (type) {
	case SELINUX_CB_LOG:
		selinux_log_direct = cb.func_log;
		break;
	case SELINUX_CB_AUDIT:
		selinux_audit = cb.func_audit;
		break;
	case SELINUX_CB_VALIDATE:
		selinux_validate = cb.func_validate;
		break;
	case SELINUX_CB_SETENFORCE:
		selinux_netlink_setenforce = cb.func_setenforce;
		break;
	case SELINUX_CB_POLICYLOAD:
		selinux_netlink_policyload = cb.func_policyload;
		break;
	}
}

/* callback getting function */
union selinux_callback
selinux_get_callback(int type)
{
	union selinux_callback cb;

	switch (type) {
	case SELINUX_CB_LOG:
		cb.func_log = selinux_log_direct;
		break;
	case SELINUX_CB_AUDIT:
		cb.func_audit = selinux_audit;
		break;
	case SELINUX_CB_VALIDATE:
		cb.func_validate = selinux_validate;
		break;
	case SELINUX_CB_SETENFORCE:
		cb.func_setenforce = selinux_netlink_setenforce;
		break;
	case SELINUX_CB_POLICYLOAD:
		cb.func_policyload = selinux_netlink_policyload;
		break;
	default:
		memset(&cb, 0, sizeof(cb));
		errno = EINVAL;
		break;
	}
	return cb;
}
