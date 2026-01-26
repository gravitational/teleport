#include "../vmlinux.h"
#include <linux/limits.h>
#include <bpf/bpf_helpers.h>       /* most used helpers: SEC, __always_inline, etc */
#include <bpf/bpf_core_read.h>     /* for BPF CO-RE helpers */
#include <bpf/bpf_tracing.h>       /* for getting kprobe arguments */

#include "./common.h"

// Maximum number of in-flight open syscalls supported
#define INFLIGHT_MAX 8192

// Size, in bytes, of the ring buffer used to report
// audit events to userspace. This is the default,
// the userspace can adjust this value based on config.
#define EVENTS_BUF_SIZE (4096*128)


char LICENSE[] SEC("license") = "Dual BSD/GPL";

struct val_t {
    u64 pid;
    const char *fname;
    int flags;
};

struct data_t {
    // CgroupID is the internal cgroupv2 ID of the event.
    u64 cgroup;
    // AuditSessionID is the audit session ID that is used to correlate
    // events with specific sessions.
    u32 audit_session_id;
    // PID is the ID of the process.
    u64 pid;
    // Return_code is the return code of open.
    int return_code;
    // Command is name of the executable opening the file.
    u8 command[TASK_COMM_LEN];
    // File_path is the full path to the file being opened.
    u8 file_path[NAME_MAX];
    // Flags are the flags passed to open.
    int flags;
};

// Force emitting struct data_t into the ELF. bpf2go needs this
// to generate the Go bindings.
const struct data_t *unused __attribute__((unused));

// Hashmap that keeps all audit session IDs that should be monitored 
// by Teleport.
BPF_HASH(monitored_sessionids, u32, u8, MAX_MONITORED_SESSIONS);

BPF_HASH(infotmp, u64, struct val_t, INFLIGHT_MAX);

// open_events ring buffer
BPF_RING_BUF(open_events, EVENTS_BUF_SIZE);

BPF_COUNTER(lost);

static int enter_open(const char *filename, int flags) {
    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    u32 session_id = BPF_CORE_READ(task, sessionid);
    u8 *is_monitored = bpf_map_lookup_elem(&monitored_sessionids, &session_id);
    if (is_monitored == NULL) {
        return 0;
    }

    print_disk_event(task, filename);

    struct val_t val = {};
    u64 id = bpf_get_current_pid_tgid();

    val.pid = id >> 32;
    val.fname = filename;
    val.flags = flags;
    bpf_map_update_elem(&infotmp, &id, &val, 0);

    return 0;
}

static int exit_open(int ret) {
    struct val_t *valp;
    u64 id = bpf_get_current_pid_tgid();

    valp = bpf_map_lookup_elem(&infotmp, &id);
    if (valp == NULL) {
        // Missed entry.
        return 0;
    }

    struct data_t data = {};
    if (bpf_get_current_comm(&data.command, sizeof(data.command)) != 0) {
        data.command[0] = '\0';
    }

    bpf_probe_read_user(&data.file_path, sizeof(data.file_path), (void *)valp->fname);

    data.pid = valp->pid;
    data.flags = valp->flags;
    data.return_code = ret;
    data.cgroup = bpf_get_current_cgroup_id();

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    data.audit_session_id = BPF_CORE_READ(task, sessionid);

    if (bpf_ringbuf_output(&open_events, &data, sizeof(data), 0) != 0)
        INCR_COUNTER(lost);

    bpf_map_delete_elem(&infotmp, &id);

    return 0;
}


SEC("tp/syscalls/sys_enter_creat")
int tracepoint__syscalls__sys_enter_creat(struct syscall_trace_enter *tp)
{
    const char *filename = (const char*) tp->args[0];

    return enter_open(filename, 0);
}

SEC("tp/syscalls/sys_exit_creat")
int tracepoint__syscalls__sys_exit_creat(struct syscall_trace_exit *tp)
{
    return exit_open(tp->ret);
}

SEC("tp/syscalls/sys_enter_open")
int tracepoint__syscalls__sys_enter_open(struct syscall_trace_enter *tp)
{
    const char *filename = (const char*) tp->args[0];
    int flags = tp->args[1];

    return enter_open(filename, flags);
};

SEC("tp/syscalls/sys_exit_open")
int tracepoint__syscalls__sys_exit_open(struct syscall_trace_exit *tp)
{
    return exit_open(tp->ret);
}

SEC("tp/syscalls/sys_enter_openat")
int tracepoint__syscalls__sys_enter_openat(struct syscall_trace_enter *tp)
{
    const char *filename = (const char*) tp->args[1];
    int flags = tp->args[2];

    return enter_open(filename, flags);
};

SEC("tp/syscalls/sys_exit_openat")
int tracepoint__syscalls__sys_exit_openat(struct syscall_trace_exit *tp)
{
    return exit_open(tp->ret);
}

SEC("tp/syscalls/sys_enter_openat2")
int tracepoint__syscalls__sys_enter_openat2(struct syscall_trace_enter *tp)
{
    const char *filename = (const char*) tp->args[1];
    struct open_how *how = (struct open_how *) tp->args[2];

    return enter_open(filename, BPF_CORE_READ(how, flags));
};

SEC("tp/syscalls/sys_exit_openat2")
int tracepoint__syscalls__sys_exit_openat2(struct syscall_trace_exit *tp)
{
    return exit_open(tp->ret);
}
