/*
 * sestatus.c
 *
 * APIs to reference SELinux kernel status page (/selinux/status)
 *
 * Author: KaiGai Kohei <kaigai@ak.jp.nec.com>
 *
 */
#include <fcntl.h>
#include <limits.h>
#include <sched.h>
#include <sys/mman.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>
#include "avc_internal.h"
#include "policy.h"

/*
 * copied from the selinux/include/security.h
 */
struct selinux_status_t
{
	uint32_t	version;	/* version number of this structure */
	uint32_t	sequence;	/* sequence number of seqlock logic */
	uint32_t	enforcing;	/* current setting of enforcing mode */
	uint32_t	policyload;	/* times of policy reloaded */
	uint32_t	deny_unknown;	/* current setting of deny_unknown */
	/* version > 0 support above status */
} __attribute((packed));

/*
 * `selinux_status'
 *
 * NULL : not initialized yet
 * MAP_FAILED : opened, but fallback-mode
 * Valid Pointer : opened and mapped correctly
 */
static struct selinux_status_t *selinux_status = NULL;
static uint32_t			last_seqno;
static uint32_t			last_policyload;

static uint32_t			fallback_sequence;
static int			fallback_enforcing;
static int			fallback_policyload;

static void			*fallback_netlink_thread = NULL;

/*
 * read_sequence
 *
 * A utility routine to reference kernel status page according to
 * seqlock logic. Since selinux_status->sequence is an odd value during
 * the kernel status page being updated, we try to synchronize completion
 * of this updating, but we assume it is rare.
 * The sequence is almost even number.
 *
 * __sync_synchronize is a portable memory barrier for various kind
 * of architecture that is supported by GCC.
 */
static inline uint32_t read_sequence(struct selinux_status_t *status)
{
	uint32_t	seqno = 0;

	do {
		/*
		 * No need for sched_yield() in the first trial of
		 * this loop.
		 */
		if (seqno & 0x0001)
			sched_yield();

		seqno = status->sequence;

		__sync_synchronize();

	} while (seqno & 0x0001);

	return seqno;
}

/*
 * selinux_status_updated
 *
 * It returns whether something has been happened since the last call.
 * Because `selinux_status->sequence' shall be always incremented on
 * both of setenforce/policyreload events, so differences from the last
 * value informs us something has been happened.
 */
int selinux_status_updated(void)
{
	uint32_t	curr_seqno;
	uint32_t	tmp_seqno;
	uint32_t	enforcing;
	uint32_t	policyload;

	if (selinux_status == NULL) {
		errno = EINVAL;
		return -1;
	}

	if (selinux_status == MAP_FAILED) {
		if (avc_netlink_check_nb() < 0)
			return -1;

		curr_seqno = fallback_sequence;
	} else {
		curr_seqno = read_sequence(selinux_status);
	}

	/*
	 * `curr_seqno' is always even-number, so it does not match with
	 * `last_seqno' being initialized to odd-number in the first call.
	 * We never return 'something was updated' in the first call,
	 * because this function focuses on status-updating since the last
	 * invocation.
	 */
	if (last_seqno & 0x0001)
		last_seqno = curr_seqno;

	if (last_seqno == curr_seqno)
		return 0;

	/* sequence must not be changed during references */
	do {
		enforcing = selinux_status->enforcing;
		policyload = selinux_status->policyload;
		tmp_seqno = curr_seqno;
		curr_seqno = read_sequence(selinux_status);
	} while (tmp_seqno != curr_seqno);

	if (avc_enforcing != (int) enforcing) {
		if (avc_process_setenforce(enforcing) < 0)
			return -1;
	}
	if (last_policyload != policyload) {
		if (avc_process_policyload(policyload) < 0)
			return -1;
		last_policyload = policyload;
	}
	last_seqno = curr_seqno;

	return 1;
}

/*
 * selinux_status_getenforce
 *
 * It returns the current performing mode of SELinux.
 * 1 means currently we run in enforcing mode, or 0 means permissive mode.
 */
int selinux_status_getenforce(void)
{
	uint32_t	seqno;
	uint32_t	enforcing;

	if (selinux_status == NULL) {
		errno = EINVAL;
		return -1;
	}

	if (selinux_status == MAP_FAILED) {
		if (avc_netlink_check_nb() < 0)
			return -1;

		return fallback_enforcing;
	}

	/* sequence must not be changed during references */
	do {
		seqno = read_sequence(selinux_status);

		enforcing = selinux_status->enforcing;

	} while (seqno != read_sequence(selinux_status));

	return enforcing ? 1 : 0;
}

/*
 * selinux_status_policyload
 *
 * It returns times of policy reloaded on the running system.
 * Note that it is not a reliable value on fallback-mode until it receives
 * the first event message via netlink socket, so, a correct usage of this
 * value is to compare it with the previous value to detect policy reloaded
 * event.
 */
int selinux_status_policyload(void)
{
	uint32_t	seqno;
	uint32_t	policyload;

	if (selinux_status == NULL) {
		errno = EINVAL;
		return -1;
	}

	if (selinux_status == MAP_FAILED) {
		if (avc_netlink_check_nb() < 0)
			return -1;

		return fallback_policyload;
	}

	/* sequence must not be changed during references */
	do {
		seqno = read_sequence(selinux_status);

		policyload = selinux_status->policyload;

	} while (seqno != read_sequence(selinux_status));

	return policyload;
}

/*
 * selinux_status_deny_unknown
 *
 * It returns a guideline to handle undefined object classes or permissions.
 * 0 means SELinux treats policy queries on undefined stuff being allowed,
 * however, 1 means such queries are denied.
 */
int selinux_status_deny_unknown(void)
{
	uint32_t	seqno;
	uint32_t	deny_unknown;

	if (selinux_status == NULL) {
		errno = EINVAL;
		return -1;
	}

	if (selinux_status == MAP_FAILED)
		return security_deny_unknown();

	/* sequence must not be changed during references */
	do {
		seqno = read_sequence(selinux_status);

		deny_unknown = selinux_status->deny_unknown;

	} while (seqno != read_sequence(selinux_status));

	return deny_unknown ? 1 : 0;
}

/*
 * callback routines for fallback case using netlink socket
 */
static int fallback_cb_setenforce(int enforcing)
{
	fallback_sequence += 2;
	fallback_enforcing = enforcing;

	return 0;
}

static int fallback_cb_policyload(int policyload)
{
	fallback_sequence += 2;
	fallback_policyload = policyload;

	return 0;
}

/*
 * selinux_status_open
 *
 * It tries to open and mmap kernel status page (/selinux/status).
 * Since Linux 2.6.37 or later supports this feature, we may run
 * fallback routine using a netlink socket on older kernels, if
 * the supplied `fallback' is not zero.
 * It returns 0 on success, -1 on error or 1 when we are ready to
 * use these interfaces, but netlink socket was opened as fallback
 * instead of the kernel status page.
 */
int selinux_status_open(int fallback)
{
	int		fd;
	char		path[PATH_MAX];
	long		pagesize;
	uint32_t	seqno;

	if (selinux_status != NULL) {
		return (selinux_status == MAP_FAILED) ? 1 : 0;
	}

	if (!selinux_mnt) {
		errno = ENOENT;
		return -1;
	}

	pagesize = sysconf(_SC_PAGESIZE);
	if (pagesize < 0)
		return -1;

	snprintf(path, sizeof(path), "%s/status", selinux_mnt);
	fd = open(path, O_RDONLY | O_CLOEXEC);
	if (fd < 0)
		goto error;

	selinux_status = mmap(NULL, pagesize, PROT_READ, MAP_SHARED, fd, 0);
	close(fd);
	if (selinux_status == MAP_FAILED) {
		goto error;
	}
	last_seqno = (uint32_t)(-1);

	/* sequence must not be changed during references */
	do {
		seqno = read_sequence(selinux_status);

		last_policyload = selinux_status->policyload;

	} while (seqno != read_sequence(selinux_status));

	/* No need to use avc threads if the kernel status page is available */
	avc_using_threads = 0;

	return 0;

error:
	/*
	 * If caller wants fallback routine, we try to provide
	 * an equivalent functionality using existing netlink
	 * socket, although it needs system call invocation to
	 * receive event notification.
	 */
	if (fallback && avc_netlink_open(0) == 0) {
		union selinux_callback	cb;

		/* register my callbacks */
		cb.func_setenforce = fallback_cb_setenforce;
		selinux_set_callback(SELINUX_CB_SETENFORCE, cb);
		cb.func_policyload = fallback_cb_policyload;
		selinux_set_callback(SELINUX_CB_POLICYLOAD, cb);

		/* mark as fallback mode */
		selinux_status = MAP_FAILED;
		last_seqno = (uint32_t)(-1);

		if (avc_using_threads)
		{
			fallback_netlink_thread = avc_create_thread(&avc_netlink_loop);
		}

		fallback_sequence = 0;
		fallback_enforcing = security_getenforce();
		fallback_policyload = 0;

		return 1;
	}
	selinux_status = NULL;

	return -1;
}

/*
 * selinux_status_close
 *
 * It unmap and close the kernel status page, or close netlink socket
 * if fallback mode.
 */
void selinux_status_close(void)
{
	long pagesize;

	/* not opened */
	if (selinux_status == NULL)
		return;

	/* fallback-mode */
	if (selinux_status == MAP_FAILED)
	{
		if (avc_using_threads)
			avc_stop_thread(fallback_netlink_thread);

		avc_netlink_release_fd();
		avc_netlink_close();
		selinux_status = NULL;
		return;
	}

	pagesize = sysconf(_SC_PAGESIZE);
	/* not much we can do other than leak memory */
	if (pagesize > 0)
		munmap(selinux_status, pagesize);
	selinux_status = NULL;

	last_seqno = (uint32_t)(-1);
}
