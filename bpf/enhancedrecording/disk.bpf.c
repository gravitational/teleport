#include "../vmlinux.h"
#include <linux/limits.h>
#include <bpf/bpf_helpers.h>       /* most used helpers: SEC, __always_inline, etc */
#include <bpf/bpf_core_read.h>     /* for BPF CO-RE helpers */
#include <bpf/bpf_tracing.h>       /* for getting kprobe arguments */

#include "./common.h"

// Size, in bytes, of the ring buffer used to report
// audit events to userspace. This is the default,
// the userspace can adjust this value based on config.
//
// Each event is 4120 bytes, so this default buffer size
// will fit just over 2000 events at once which should be
// a safe default.
#define EVENTS_BUF_SIZE (4096 * 2048)

// ERR_PTR range: pointers in [-MAX_ERRNO, -1] indicate errors.
// This matches the kernel's MAX_ERRNO in include/linux/err.h.
#define MAX_ERRNO 4095

char LICENSE[] SEC("license") = "Dual BSD/GPL";

// inflight_disk_t stores the resolved path obtained from bpf_d_path
// in fentry/security_file_open. This is stashed in task-local
// storage so that fexit/do_filp_open can retrieve it.
// security_file_open only fires for opens that get far enough
// to have a dentry, so this won't be populated for early
// failures like ENOENT — that's fine, do_filp_open falls back
// to the unresolved path from struct filename in that case.
struct inflight_disk_t {
    bool valid;
    u8 file_path[PATH_MAX];
};

struct {
    __uint(type, BPF_MAP_TYPE_TASK_STORAGE);
    __uint(map_flags, BPF_F_NO_PREALLOC);
    __type(key, int);
    __type(value, struct inflight_disk_t);
} inflight_open SEC(".maps");

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

// security_file_open allows us to get the resolved absolute path
// of the file being opened. We stash the resolved path so 
// fexit/do_filp_open can emit an event with a return code and our
// resolved path.
SEC("fentry/security_file_open")
int BPF_PROG(security_file_open, struct file *f)
{
    struct task_struct *task = bpf_get_current_task_btf();
    u32 session_id = task->sessionid;
    u8 *is_monitored = bpf_map_lookup_elem(&monitored_sessionids, &session_id);
    if (is_monitored == NULL) {
        return 0;
    }

    struct inflight_disk_t *info = bpf_task_storage_get(&inflight_open, task, NULL, BPF_LOCAL_STORAGE_GET_F_CREATE);
    if (!info) {
        return 0;
    }

    if (bpf_d_path(&f->f_path, (char *)info->file_path, sizeof(info->file_path)) > 0) {
        info->valid = true;
    }

    return 0;
}

// do_filp_open is called for all file open syscalls, but we aren't
// able to get a resolved path here. If opening failed early,
// security_file_open will not be called so we fall back to using the
// unresolved path in the emitted event.
SEC("fexit/do_filp_open")
int BPF_PROG(do_filp_open_exit, int dfd, struct filename *pathname, const struct open_flags *op, struct file *ret)
{
    struct task_struct *task = bpf_get_current_task_btf();
    u32 session_id = task->sessionid;
    u8 *is_monitored = bpf_map_lookup_elem(&monitored_sessionids, &session_id);
    if (is_monitored == NULL) {
        return 0;
    }

    struct inflight_disk_t *info = bpf_task_storage_get(&inflight_open, task, NULL, 0);

    struct data_t *data = bpf_ringbuf_reserve(&open_events, sizeof(*data), 0);
    if (!data) {
        INCR_COUNTER(lost);
        bpf_printk("open_events ring buffer full");
        goto out;
    }

    // Use the resolved path from security_file_open if possible.
    if (info && info->valid) {
        bpf_probe_read_kernel_str(data->file_path, sizeof(data->file_path), info->file_path);
    } else {
        const char *name = BPF_CORE_READ(pathname, name);
        bpf_probe_read_kernel_str(data->file_path, sizeof(data->file_path), name);
    }

    data->flags = BPF_CORE_READ(op, open_flag);

    // If the return is a pointer to a file, the open succeeded and the
    // return code is 0. Otherwise the open failed and the return code
    // is the negative errno.
    bool is_err = (unsigned long)ret >= (unsigned long)-MAX_ERRNO;
    data->return_code = is_err ? (long)ret : 0;

    print_disk_event(task, (char *)data->file_path, data->return_code);

    bpf_get_current_comm(&data->command, sizeof(data->command));
    data->pid = bpf_get_current_pid_tgid() >> 32;
    data->cgroup = bpf_get_current_cgroup_id();
    data->audit_session_id = session_id;

    bpf_ringbuf_submit(data, 0);

out:
    // Mark consumed so a subsequent open on the same thread doesn't 
    // pick up stale data.
    if (info) {
        info->valid = false;
    }

    return 0;
}
