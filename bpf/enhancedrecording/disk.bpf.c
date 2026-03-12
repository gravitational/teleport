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
#define EVENTS_BUF_SIZE (4096 * 2048)

char LICENSE[] SEC("license") = "Dual BSD/GPL";

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
    u8 file_path[PATH_MAX];
    // Flags are the flags passed to open.
    int flags;
};

// Force emitting struct data_t into the ELF. bpf2go needs this
// to generate the Go bindings.
const struct data_t *unused __attribute__((unused));

// Hashmap that keeps all audit session IDs that should be monitored 
// by Teleport.
BPF_HASH(monitored_sessionids, u32, u8, MAX_MONITORED_SESSIONS);

// open_events ring buffer
BPF_RING_BUF(open_events, EVENTS_BUF_SIZE);

BPF_COUNTER(lost);

static int handle_open(struct file *f) {
    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    u32 session_id = BPF_CORE_READ(task, sessionid);
    u8 *is_monitored = bpf_map_lookup_elem(&monitored_sessionids, &session_id);
    if (is_monitored == NULL) {
        return 0;
    }

    struct data_t *data = bpf_ringbuf_reserve(&open_events, sizeof(*data), 0);
    if (!data) {
        INCR_COUNTER(lost);
        bpf_printk("open_events ring buffer full");
        return 0;
    }

    bpf_d_path(&f->f_path, (char *)data->file_path, sizeof(data->file_path));
    print_disk_event(task, (char *)data->file_path);

    bpf_get_current_comm(&data->command, sizeof(data->command));

    data->pid = bpf_get_current_pid_tgid() >> 32;
    data->flags = BPF_CORE_READ(f, f_flags);
    data->cgroup = bpf_get_current_cgroup_id();
    data->audit_session_id = BPF_CORE_READ(task, sessionid);

    bpf_ringbuf_submit(data, 0);

    return 0;
}

SEC("fentry/security_file_open")
int BPF_PROG(security_file_open, struct file *f) {
    return handle_open(f);
}
