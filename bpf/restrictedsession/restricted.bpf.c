/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
#include <linux/errno.h>

#include "../vmlinux.h"
#include <bpf/bpf_helpers.h>       /* most used helpers: SEC, __always_inline, etc */
#include <bpf/bpf_core_read.h>     /* for BPF CO-RE helpers */
#include <bpf/bpf_tracing.h>       /* for getting kprobe arguments */

#include "../helpers.h"

#define AF_INET     2   /* IP protocol family.  */
#define AF_INET6    10  /* IP version 6.  */

// Maximum number of restricted cgroups supported
#define MAX_RESTRICTED_CGROUPS   8192

// Maximum number of IPv4 CIDRs allowed in each of Deny and Allow lists
#define MAX_IPV4_CIDRS          256

// Maximum number of IPv6 CIDRs allowed in each of Deny and Allow lists.
// IPv4 CIDRs are also added to IPv6 lists because of 4-to-6 mapped addresses.
#define MAX_IPV6_CIDRS           (MAX_IPV4_CIDRS+128)

// Audit events ring buffer size in bytes.
// This default is overriden by the userspace portion.
#define AUDIT_EVENTS_RING_SIZE  (4*4096)

char LICENSE[] SEC("license") = "Dual BSD/GPL";

// Keeps the set of cgroups which are enforced
BPF_HASH(restricted_cgroups, u64, u8, MAX_RESTRICTED_CGROUPS);

// checks if the given cgroup is restricted
static inline bool in_restricted_cgroup(u64 cg) {
	return (bool) bpf_map_lookup_elem(&restricted_cgroups, &cg);
}

// IPv4 addr/mask to store in the trie
struct ip4_trie_key {
	u32 prefixlen;
	struct in_addr addr;
};

// IPv6 addr/mask to store in the trie
struct ip6_trie_key {
	u32 prefixlen;
	struct in6_addr addr;
};

// IPv4 addresses (with masks) to deny
BPF_LPM_TRIE(ip4_denylist, struct ip4_trie_key, u8, MAX_IPV4_CIDRS);

// IPv4 addresses (with masks) to allow (override deny)
BPF_LPM_TRIE(ip4_allowlist, struct ip4_trie_key, u8, MAX_IPV4_CIDRS);

// IPv6 addresses (with masks) to deny
BPF_LPM_TRIE(ip6_denylist, struct ip6_trie_key, u8, MAX_IPV6_CIDRS);

// IPv6 addresses (with masks) to allow (override deny)
BPF_LPM_TRIE(ip6_allowlist, struct ip6_trie_key, u8, MAX_IPV6_CIDRS);

// checks if the given address (IP) is allowed
static inline bool is_ip4_blocked(const struct sockaddr_in *addr) {
	struct ip4_trie_key key = {
		.prefixlen = 32,
		.addr = addr->sin_addr
	};

	if (!bpf_map_lookup_elem(&ip4_allowlist, &key))
		return true;

	return (bool)bpf_map_lookup_elem(&ip4_denylist, &key);
}

// checks if the given address (IP) is blocked
static inline bool is_ip6_blocked(const struct sockaddr_in6 *addr) {
	struct ip6_trie_key key;
	key.prefixlen = 128;
	key.addr = BPF_CORE_READ(addr, sin6_addr);

	if (!bpf_map_lookup_elem(&ip6_allowlist, &key))
		return true;

	return (bool)bpf_map_lookup_elem(&ip6_denylist, &key);
}

// A channel back to user-space to notify it of blocked actions
BPF_RING_BUF(audit_events, AUDIT_EVENTS_RING_SIZE);
// Counter of lost audit_events
BPF_COUNTER(lost);

enum audit_event_type {
	BLOCKED_IPV4,
	BLOCKED_IPV6
};

enum network_op {
	OP_CONNECT,
	OP_SENDMSG
};

// The header of the audit events. Followed by a type specific body.
struct audit_event_header {
	u64 cgroup;
	u32 pid;
	enum audit_event_type type;
	char task[TASK_COMM_LEN];
};

// hdr.type=BLOCKED_IPV4
struct audit_event_blocked_ipv4 {
	struct audit_event_header hdr;
	struct in_addr src;
	struct in_addr dst;
	u16 dport;
	u8 operation;
};

// hdr.type=BLOCKED_IPV6
struct audit_event_blocked_ipv6 {
	struct audit_event_header hdr;
	struct in6_addr src;
	struct in6_addr dst;
	u16 dport;
	u8 operation;
};

// Returns the source address (IPv4) to which the socket is bound
static inline struct in_addr src_addr4(const struct socket *sock) {
	struct in_addr addr;
	__builtin_memset(&addr, 0, sizeof(addr));

	addr.s_addr = BPF_CORE_READ(sock, sk, __sk_common.skc_rcv_saddr);
	return addr;
}

// Returns the source address (IPv6) to which the socket is bound
static inline struct in6_addr src_addr6(const struct socket *sock) {
	struct in6_addr addr;
	__builtin_memset(&addr, 0, sizeof(addr));

	addr = BPF_CORE_READ(sock, sk, __sk_common.skc_v6_rcv_saddr);
	return addr;
}

static inline void report_ip4_block(void *ctx, u64 cg, enum network_op op, struct socket *sock, const struct sockaddr_in *daddr) {
	struct audit_event_blocked_ipv4 ev;
	__builtin_memset(&ev, 0, sizeof(ev));

	ev.hdr.cgroup = cg;
	ev.hdr.pid = (u32) (bpf_get_current_pid_tgid() >> 32);
	ev.hdr.type = BLOCKED_IPV4;
	bpf_get_current_comm(&ev.hdr.task, sizeof(ev.hdr.task));
	ev.dport = __builtin_bswap16(daddr->sin_port);
	ev.src = src_addr4(sock);
	ev.dst = BPF_CORE_READ(daddr, sin_addr);
	ev.operation = (u8)op;

	if (bpf_ringbuf_output(&audit_events, &ev, sizeof(ev), 0) != 0)
		INCR_COUNTER(lost);
}

static inline void report_ip6_block(void *ctx, u64 cg, enum network_op op, const struct socket *sock, const struct sockaddr_in6 *daddr) {
	struct audit_event_blocked_ipv6 ev;
	__builtin_memset(&ev, 0, sizeof(ev));

	ev.hdr.cgroup = cg;
	ev.hdr.pid = (u32) (bpf_get_current_pid_tgid() >> 32);
	ev.hdr.type = BLOCKED_IPV6;
	bpf_get_current_comm(&ev.hdr.task, sizeof(ev.hdr.task));
	ev.dport = __builtin_bswap16(daddr->sin6_port);
	ev.src = src_addr6(sock);
	ev.dst = BPF_CORE_READ(daddr, sin6_addr);
	ev.operation = (u8)op;

	if (bpf_ringbuf_output(&audit_events, &ev, sizeof(ev), 0) != 0)
		INCR_COUNTER(lost);
}

SEC("lsm/socket_connect")
int BPF_PROG(socket_connect, struct socket *sock, struct sockaddr *address, int addrlen) {
	// Only care about IP4 and IP6
	if (address->sa_family != AF_INET && address->sa_family != AF_INET6)
		return 0;

	// Processes not in restricted cgroups are always allowed
	u64 cg = bpf_get_current_cgroup_id();
	if (!in_restricted_cgroup(cg))
		return 0;

	if (address->sa_family == AF_INET) {
		if (addrlen < sizeof(struct sockaddr_in))
			return -EINVAL;

		struct sockaddr_in *inet_addr = (struct sockaddr_in*)address;

		if (is_ip4_blocked(inet_addr)) {
			report_ip4_block((void*) ctx, cg, OP_CONNECT, sock, inet_addr);
			return -EPERM;
		}
	} else { // AF_INET6
		if (addrlen < sizeof(struct sockaddr_in6))
			return -EINVAL;

		struct sockaddr_in6 *inet_addr = (struct sockaddr_in6*)address;

		if (is_ip6_blocked(inet_addr)) {
			report_ip6_block((void*) ctx, cg, OP_CONNECT, sock, inet_addr);
			return -EPERM;
		}
	}

	return 0;
}

SEC("lsm/socket_sendmsg")
int BPF_PROG(socket_sendmsg, struct socket *sock, struct msghdr *msg) {
	// BPF stack is only 512 bytes but address_storage is pretty big.
	// Therefore we use a smaller number since we only care about
	// sockaddr_in and sockaddr_in6. Both are less than 64 bytes.
	// Must be zero initialized to make the verifier happy.
	char sa_buf[64] = { '\0' };

	size_t namelen = msg->msg_namelen;
	struct sockaddr *sa;
	int family = sock->ops->family;

	// Only care about IPv4 and IPv6
	if (family != AF_INET && family != AF_INET6)
		return 0;

	// If sockaddr (name) is not specified, it's an already connected socket
	// and socket_connect hook would have taken care of it.
	if (!msg->msg_name)
		return 0;

	if (namelen < sizeof(sa->sa_family))
		return -EINVAL;

	if (bpf_probe_read(sa_buf, namelen & (sizeof(sa_buf)-1), msg->msg_name) < 0)
		return -EINVAL;

	sa = (struct sockaddr *) sa_buf;

	// Socket address and socket family must match
	if (sa->sa_family != family)
		return -EINVAL;

	// Processes not in restricted cgroups are always allowed
	u64 cg = bpf_get_current_cgroup_id();
	if (!in_restricted_cgroup(cg))
		return 0;

	if (family == AF_INET) {
		struct sockaddr_in *inet_addr = (struct sockaddr_in *)sa;

		if (namelen < sizeof(struct sockaddr_in))
			return -EINVAL;

		if (is_ip4_blocked(inet_addr)) {
			report_ip4_block((void*) ctx, cg, OP_SENDMSG, sock, inet_addr);
			return -EPERM;
		}
	} else { // AF_INET6
		struct sockaddr_in6 *inet_addr = (struct sockaddr_in6 *)sa;

		if (namelen < sizeof(struct sockaddr_in6))
			return -EINVAL;

		if (is_ip6_blocked(inet_addr)) {
			report_ip6_block((void*) ctx, cg, OP_SENDMSG, sock, inet_addr);
			return -EPERM;
		}
	}

	return 0;
}
