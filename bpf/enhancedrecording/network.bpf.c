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

#include "../vmlinux.h"
#include <bpf/bpf_helpers.h>       /* most used helpers: SEC, __always_inline, etc */
#include <bpf/bpf_core_read.h>     /* for BPF CO-RE helpers */
#include <bpf/bpf_tracing.h>       /* for getting kprobe arguments */

#include "../helpers.h"

char LICENSE[] SEC("license") = "Dual BSD/GPL";

// Maximum number of in-flight connect syscalls supported
#define INFLIGHT_MAX 8192

// Size, in bytes, of the ring buffer used to report
// audit events to userspace. This is the default,
// the userspace can adjust this value based on config.
#define EVENTS_BUF_SIZE (4096*8)

BPF_HASH(currsock, u32, struct sock *, INFLIGHT_MAX);

// separate data structs for ipv4 and ipv6
struct ipv4_data_t {
    u64 cgroup;
    u64 ip;
    u32 pid;
    u32 saddr;
    u32 daddr;
    u16 dport;
    char task[TASK_COMM_LEN];
};
BPF_RING_BUF(ipv4_events, EVENTS_BUF_SIZE);

struct ipv6_data_t {
    u64 cgroup;
    u64 ip;
    u32 pid;
    struct in6_addr saddr;
    struct in6_addr daddr;
    u16 dport;
    char task[TASK_COMM_LEN];
};
BPF_RING_BUF(ipv6_events, EVENTS_BUF_SIZE);

BPF_COUNTER(lost);

static int trace_connect_entry(struct sock *sk)
{
    u32 id = bpf_get_current_pid_tgid();

    // Stash the sock ptr for lookup on return.
    bpf_map_update_elem(&currsock, &id, &sk, 0);

    return 0;
};

static int trace_connect_return(int ret, short ipver)
{
    u64 pid_tgid = bpf_get_current_pid_tgid();
	u32 id = (u32)pid_tgid;

    struct sock **skpp;
    skpp = bpf_map_lookup_elem(&currsock, &id);
    if (skpp == 0) {
        return 0;   // missed entry
    }

    if (ret != 0) {
        // failed to send SYNC packet, may not have populated
        // socket __sk_common.{skc_rcv_saddr, ...}
        bpf_map_delete_elem(&currsock, &id);
        return 0;
    }

    // pull in details
    struct sock *skp = *skpp;
    u16 dport = BPF_CORE_READ(skp, __sk_common.skc_dport);

    if (ipver == 4) {
        struct ipv4_data_t data4 = {.pid = pid_tgid >> 32, .ip = ipver};
        data4.saddr = BPF_CORE_READ(skp, __sk_common.skc_rcv_saddr);
        data4.daddr = BPF_CORE_READ(skp, __sk_common.skc_daddr);
        data4.dport = __builtin_bswap16(dport);
        data4.cgroup = bpf_get_current_cgroup_id();
        bpf_get_current_comm(&data4.task, sizeof(data4.task));
        if (bpf_ringbuf_output(&ipv4_events, &data4, sizeof(data4), 0) != 0)
            INCR_COUNTER(lost);

    } else /* 6 */ {
        struct ipv6_data_t data6 = {.pid = pid_tgid >> 32, .ip = ipver};

        BPF_CORE_READ_INTO(&data6.saddr, skp, __sk_common.skc_v6_rcv_saddr);
        BPF_CORE_READ_INTO(&data6.daddr, skp, __sk_common.skc_v6_daddr);

        data6.dport = __builtin_bswap16(dport);
        data6.cgroup = bpf_get_current_cgroup_id();
        bpf_get_current_comm(&data6.task, sizeof(data6.task));
        if (bpf_ringbuf_output(&ipv6_events, &data6, sizeof(data6), 0) != 0)
            INCR_COUNTER(lost);
    }

    bpf_map_delete_elem(&currsock, &id);

    return 0;
}

SEC("kprobe/tcp_v4_connect")
int BPF_KPROBE(kprobe__tcp_v4_connect, struct sock *sk)
{
    return trace_connect_entry(sk);
}

SEC("kretprobe/tcp_v4_connect")
int kretprobe__tcp_v4_connect(struct pt_regs *ctx)
{
    return trace_connect_return(PT_REGS_RC(ctx), 4);
}

SEC("kprobe/tcp_v6_connect")
int BPF_KPROBE(kprobe__tcp_v6_connect, struct sock *sk)
{
    return trace_connect_entry(sk);
}

SEC("kretprobe/tcp_v6_connect")
int kretprobe__tcp_v6_connect(struct pt_regs *ctx)
{
    return trace_connect_return(PT_REGS_RC(ctx), 6);
}
