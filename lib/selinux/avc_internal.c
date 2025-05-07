/*
 * Callbacks for user-supplied memory allocation, supplemental
 * auditing, and locking routines.
 *
 * Author : Eamon Walsh <ewalsh@epoch.ncsc.mil>
 *
 * Netlink code derived in part from sample code by
 * James Morris <jmorris@redhat.com>.
 */

#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <unistd.h>
#include <fcntl.h>
#include <string.h>
#include <poll.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <linux/types.h>
#include <linux/netlink.h>
#include "callbacks.h"
#include "selinux_netlink.h"
#include "avc_internal.h"
#include "selinux_internal.h"

#ifndef NETLINK_SELINUX
#define NETLINK_SELINUX 7
#endif

/* callback pointers */
void *(*avc_func_malloc) (size_t) = NULL;
void (*avc_func_free) (void *) = NULL;

void (*avc_func_log) (const char *, ...) = NULL;
void (*avc_func_audit) (void *, security_class_t, char *, size_t) = NULL;

int avc_using_threads = 0;
int avc_app_main_loop = 0;
void *(*avc_func_create_thread) (void (*)(void)) = NULL;
void (*avc_func_stop_thread) (void *) = NULL;

void *(*avc_func_alloc_lock) (void) = NULL;
void (*avc_func_get_lock) (void *) = NULL;
void (*avc_func_release_lock) (void *) = NULL;
void (*avc_func_free_lock) (void *) = NULL;

/* message prefix string and avc enforcing mode */
char avc_prefix[AVC_PREFIX_SIZE] = "uavc";
int avc_running = 0;
int avc_enforcing = 1;
int avc_setenforce = 0;

/* process setenforce events for netlink and sestatus */
int avc_process_setenforce(int enforcing)
{
	int rc = 0;

	avc_log(SELINUX_SETENFORCE,
		"%s:  op=setenforce lsm=selinux enforcing=%d res=1",
		avc_prefix, enforcing);
	if (avc_setenforce)
		goto out;
	avc_enforcing = enforcing;
	if (avc_enforcing && (rc = avc_ss_reset(0)) < 0) {
		avc_log(SELINUX_ERROR,
			"%s:  cache reset returned %d (errno %d)\n",
			avc_prefix, rc, errno);
		return rc;
	}

out:
	return selinux_netlink_setenforce(enforcing);
}

/* process policyload events for netlink and sestatus */
int avc_process_policyload(uint32_t seqno)
{
	int rc = 0;

	avc_log(SELINUX_POLICYLOAD,
		"%s:  op=load_policy lsm=selinux seqno=%u res=1",
		avc_prefix, seqno);
	rc = avc_ss_reset(seqno);
	if (rc < 0) {
		avc_log(SELINUX_ERROR,
			"%s:  cache reset returned %d (errno %d)\n",
			avc_prefix, rc, errno);
		return rc;
	}

	selinux_flush_class_cache();

	return selinux_netlink_policyload(seqno);
}

/* netlink socket code */
static int fd = -1;

int avc_netlink_open(int blocking)
{
	int len, rc = 0;
	struct sockaddr_nl addr;

	fd = socket(PF_NETLINK, SOCK_RAW | SOCK_CLOEXEC, NETLINK_SELINUX);
	if (fd < 0) {
		rc = fd;
		goto out;
	}
	
	if (!blocking && fcntl(fd, F_SETFL, O_NONBLOCK)) {
		close(fd);
		fd = -1;
		rc = -1;
		goto out;
	}

	len = sizeof(addr);

	memset(&addr, 0, len);
	addr.nl_family = AF_NETLINK;
	addr.nl_groups = SELNL_GRP_AVC;

	if (bind(fd, (struct sockaddr *)&addr, len) < 0) {
		close(fd);
		fd = -1;
		rc = -1;
		goto out;
	}
      out:
	return rc;
}

void avc_netlink_close(void)
{
	if (fd >= 0)
		close(fd);
	fd = -1;
}

static int avc_netlink_receive(void *buf, unsigned buflen, int blocking)
{
	int rc;
	struct pollfd pfd = { fd, POLLIN | POLLPRI, 0 };
	struct sockaddr_nl nladdr;
	socklen_t nladdrlen = sizeof nladdr;
	struct nlmsghdr *nlh = (struct nlmsghdr *)buf;

	do {
		rc = poll(&pfd, 1, (blocking ? -1 : 0));
	} while (rc < 0 && errno == EINTR);

	if (rc == 0 && !blocking) {
		errno = EWOULDBLOCK;
		return -1;
	}
	else if (rc < 1) {
		avc_log(SELINUX_ERROR, "%s:  netlink poll: error %d\n",
			avc_prefix, errno);
		return rc;
	}

	rc = recvfrom(fd, buf, buflen, 0, (struct sockaddr *)&nladdr,
		      &nladdrlen);
	if (rc < 0)
		return rc;

	if (nladdrlen != sizeof nladdr) {
		avc_log(SELINUX_WARNING,
			"%s:  warning: netlink address truncated, len %u?\n",
			avc_prefix, nladdrlen);
		return -1;
	}

	if (nladdr.nl_pid) {
		avc_log(SELINUX_WARNING,
			"%s:  warning: received spoofed netlink packet from: %u\n",
			avc_prefix, nladdr.nl_pid);
		return -1;
	}

	if (rc == 0) {
		avc_log(SELINUX_WARNING,
			"%s:  warning: received EOF on netlink socket\n",
			avc_prefix);
		errno = EBADFD;
		return -1;
	}

	if (nlh->nlmsg_flags & MSG_TRUNC || nlh->nlmsg_len > (unsigned)rc) {
		avc_log(SELINUX_WARNING,
			"%s:  warning: incomplete netlink message\n",
			avc_prefix);
		return -1;
	}

	return 0;
}

static int avc_netlink_process(void *buf)
{
	int rc;
	struct nlmsghdr *nlh = (struct nlmsghdr *)buf;

	switch (nlh->nlmsg_type) {
	case NLMSG_ERROR:{
		struct nlmsgerr *err = NLMSG_DATA(nlh);

		/* Netlink ack */
		if (err->error == 0)
			break;

		errno = -err->error;
		avc_log(SELINUX_ERROR,
			"%s:  netlink error: %d\n", avc_prefix, errno);
		return -1;
	}

	case SELNL_MSG_SETENFORCE:{
		struct selnl_msg_setenforce *msg = NLMSG_DATA(nlh);
		rc = avc_process_setenforce(!!msg->val);
		if (rc < 0)
			return rc;
		break;
	}

	case SELNL_MSG_POLICYLOAD:{
		struct selnl_msg_policyload *msg = NLMSG_DATA(nlh);
		rc = avc_process_policyload(msg->seqno);
		if (rc < 0)
			return rc;
		break;
	}

	default:
		avc_log(SELINUX_WARNING,
			"%s:  warning: unknown netlink message %d\n",
			avc_prefix, nlh->nlmsg_type);
	}
	return 0;
}

int avc_netlink_check_nb(void)
{
	int rc;
	char buf[1024] __attribute__ ((aligned));

	while (1) {
		errno = 0;
		rc = avc_netlink_receive(buf, sizeof(buf), 0);
		if (rc < 0) {
			if (errno == EWOULDBLOCK)
				return 0;
			if (errno == 0 || errno == EINTR)
				continue;
			else {
				avc_log(SELINUX_ERROR,
					"%s:  netlink recvfrom: error %d\n",
					avc_prefix, errno);
				return rc;
			}
		}

		(void)avc_netlink_process(buf);
	}
	return 0;
}

/* run routine for the netlink listening thread */
void avc_netlink_loop(void)
{
	int rc;
	char buf[1024] __attribute__ ((aligned));

	while (1) {
		errno = 0;
		rc = avc_netlink_receive(buf, sizeof(buf), 1);
		if (rc < 0) {
			if (errno == 0 || errno == EINTR)
				continue;
			else {
				avc_log(SELINUX_ERROR,
					"%s:  netlink recvfrom: error %d\n",
					avc_prefix, errno);
				break;
			}
		}

		rc = avc_netlink_process(buf);
		if (rc < 0)
			break;
	}

	close(fd);
	fd = -1;
	avc_log(SELINUX_ERROR,
		"%s:  netlink thread: errors encountered, terminating\n",
		avc_prefix);
}

int avc_netlink_acquire_fd(void)
{
	if (fd < 0) {
		int rc = 0;
		rc = avc_netlink_open(0);
		if (rc < 0) {
			avc_log(SELINUX_ERROR,
				"%s: could not open netlink socket: %d (%m)\n",
				avc_prefix, errno);
			return rc;
		}
	}

    avc_app_main_loop = 1;

    return fd;
}

void avc_netlink_release_fd(void)
{
    avc_app_main_loop = 0;
}
