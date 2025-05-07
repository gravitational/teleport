/*
 * This file describes the callbacks passed to selinux_init() and available
 * for use from the library code.  They all have default implementations.
 */
#ifndef _SELINUX_CALLBACKS_H_
#define _SELINUX_CALLBACKS_H_

#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <selinux/selinux.h>

#include "selinux_internal.h"

/* callback pointers */
extern int __attribute__ ((format(printf, 2, 3)))
(*selinux_log_direct) (int type, const char *, ...) ;

extern int
(*selinux_audit) (void *, security_class_t, char *, size_t) ;

extern int
(*selinux_validate)(char **ctx) ;

extern int
(*selinux_netlink_setenforce) (int enforcing) ;

extern int
(*selinux_netlink_policyload) (int seqno) ;

/* Thread-safe selinux_log() function */
extern pthread_mutex_t log_mutex;

#define selinux_log(type, ...) do { \
	int saved_errno__ = errno; \
	__pthread_mutex_lock(&log_mutex); \
	selinux_log_direct(type, __VA_ARGS__); \
	__pthread_mutex_unlock(&log_mutex); \
	errno = saved_errno__; \
} while(0)

#endif				/* _SELINUX_CALLBACKS_H_ */
