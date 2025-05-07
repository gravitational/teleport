/*
 * Netlink event notifications for SELinux.
 *
 * Author: James Morris <jmorris@redhat.com>
 */
#ifndef _LINUX_SELINUX_NETLINK_H
#define _LINUX_SELINUX_NETLINK_H

/* Message types. */
#define SELNL_MSG_BASE 0x10
enum {
	SELNL_MSG_SETENFORCE = SELNL_MSG_BASE,
	SELNL_MSG_POLICYLOAD,
	SELNL_MSG_MAX
};

/* Multicast groups */
#define SELNL_GRP_NONE		0x00000000
#define SELNL_GRP_AVC		0x00000001	/* AVC notifications */
#define SELNL_GRP_ALL		0xffffffff

/* Message structures */
struct selnl_msg_setenforce {
	int32_t val;
};

struct selnl_msg_policyload {
	uint32_t seqno;
};

#endif				/* _LINUX_SELINUX_NETLINK_H */
