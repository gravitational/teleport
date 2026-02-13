#include "../vmlinux.h"
#include <bpf/bpf_helpers.h>       /* most used helpers: SEC, __always_inline, etc */
#include <bpf/bpf_core_read.h>     /* for BPF CO-RE helpers */
#include <bpf/bpf_tracing.h>       /* for getting kprobe arguments */

#include "./common.h"

#define IPV4 4
#define IPV6 6

char LICENSE[] SEC("license") = "Dual BSD/GPL";

// Maximum number of in-flight connect syscalls supported
#define INFLIGHT_MAX 8192

// Size, in bytes, of the ring buffer used to report
// audit events to userspace. This is the default,
// the userspace can adjust this value based on config.
#define EVENTS_BUF_SIZE (4096*8)

// Hashmap that keeps all audit session IDs that should be monitored 
// by Teleport.
BPF_HASH(monitored_sessionids, u32, u8, MAX_MONITORED_SESSIONS);

BPF_HASH(currsock, u32, struct sock *, INFLIGHT_MAX);

// separate data structs for ipv4 and ipv6
struct ipv4_data_t {
    // CgroupID is the internal cgroupv2 ID of the event.
    u64 cgroup;
    // AuditSessionID is the audit session ID that is used to correlate
    // events with specific sessions.
    u32 audit_session_id;
    // Version is the version of TCP (4 or 6).
    u64 ip;
    // PID is the process ID.
    u32 pid;
    // SrcAddr is the source IP address.
    u32 saddr;
    // DstAddr is the destination IP address.
    u32 daddr;
    // DstPort is the port the connection is being made to.
    u16 dport;
    // Command is name of the executable making the connection.
    u8 command[TASK_COMM_LEN];
};
BPF_RING_BUF(ipv4_events, EVENTS_BUF_SIZE);

// Force emitting struct ipv4_data_t into the ELF. bpf2go needs this
// to generate the Go bindings.
const struct ipv4_data_t *unused_ipv4_data_t __attribute__((unused));

struct ipv6_data_t {
    // CgroupID is the internal cgroupv2 ID of the event.
    u64 cgroup;
    // AuditSessionID is the audit session ID that is used to correlate
    // events with specific sessions.
    u32 audit_session_id;
    // Version is the version of TCP (4 or 6).
    u64 ip;
    // PID is the process ID.
    u32 pid;
    // SrcAddr is the source IP address.
    struct in6_addr saddr;
    // DstAddr is the destination IP address.
    struct in6_addr daddr;
    // DstPort is the port the connection is being made to.
    u16 dport;
    // Command is name of the executable making the connection.
    u8 command[TASK_COMM_LEN];
};
BPF_RING_BUF(ipv6_events, EVENTS_BUF_SIZE);

// Force emitting struct ipv6_data_t into the ELF. bpf2go needs this
// to generate the Go bindings.
const struct ipv6_data_t *unused_ipv6_data_t __attribute__((unused));

BPF_COUNTER(lost);

static int trace_connect_entry(struct sock *sk)
{
    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    u32 session_id = BPF_CORE_READ(task, sessionid);
    u8 *is_monitored = bpf_map_lookup_elem(&monitored_sessionids, &session_id);
    if (is_monitored == NULL) {
        return 0;
    }

    u32 id = bpf_get_current_pid_tgid();

    // Stash the sock ptr for lookup on return.
    bpf_map_update_elem(&currsock, &id, &sk, 0);

    return 0;
};

static int trace_connect_return(int ret, short ipver)
{
    struct sock **skpp;
    u64 pid_tgid = bpf_get_current_pid_tgid();
    u32 id = (u32)pid_tgid;
    u32 pid = pid_tgid >> 32;

    skpp = bpf_map_lookup_elem(&currsock, &id);
    if (skpp == NULL) {
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

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    u32 session_id = BPF_CORE_READ(task, sessionid);

    if (ipver == IPV4) {
        struct ipv4_data_t data4 = {.pid = pid, .ip = ipver};
        data4.saddr = BPF_CORE_READ(skp, __sk_common.skc_rcv_saddr);
        data4.daddr = BPF_CORE_READ(skp, __sk_common.skc_daddr);
        data4.dport = __builtin_bswap16(dport);
        data4.cgroup = bpf_get_current_cgroup_id();
        data4.audit_session_id = session_id;
        bpf_get_current_comm(&data4.command, sizeof(data4.command));
        if (bpf_ringbuf_output(&ipv4_events, &data4, sizeof(data4), 0) != 0)
            INCR_COUNTER(lost);

    } else /* IPV6 */ {
        struct ipv6_data_t data6 = {.pid = pid, .ip = ipver};

        BPF_CORE_READ_INTO(&data6.saddr, skp, __sk_common.skc_v6_rcv_saddr);
        BPF_CORE_READ_INTO(&data6.daddr, skp, __sk_common.skc_v6_daddr);

        data6.dport = __builtin_bswap16(dport);
        data6.cgroup = bpf_get_current_cgroup_id();
        data6.audit_session_id = session_id;
        bpf_get_current_comm(&data6.command, sizeof(data6.command));
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
    return trace_connect_return(PT_REGS_RC(ctx), IPV4);
}

SEC("kprobe/tcp_v6_connect")
int BPF_KPROBE(kprobe__tcp_v6_connect, struct sock *sk)
{
    return trace_connect_entry(sk);
}

SEC("kretprobe/tcp_v6_connect")
int kretprobe__tcp_v6_connect(struct pt_regs *ctx)
{
    return trace_connect_return(PT_REGS_RC(ctx), IPV6);
}
