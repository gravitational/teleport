#include "../vmlinux.h"
#include <bpf/bpf_helpers.h>       /* most used helpers: SEC, __always_inline, etc */
#include <bpf/bpf_core_read.h>     /* for BPF CO-RE helpers */
#include <bpf/bpf_tracing.h>       /* for getting kprobe arguments */

#include "./common.h"

// Size, in bytes, of the ring buffer used to report
// audit events to userspace. This is the default,
// the userspace can adjust this value based on config.
#define EVENTS_BUF_SIZE (4096 * 128)

char LICENSE[] SEC("license") = "Dual BSD/GPL";

struct data_t {
    // pid as in the userspace term (i.e. task->tgid in kernel).
    u64 pid;
    // ppid is the userspace term (i.e task->real_parent->tgid in kernel).
    u64 ppid;
    // Command is the executable.
    u8 command[TASK_COMM_LEN];
    // Filename is the path of the executable.
    u8 filename[FILENAMESIZE];
    // Args is the list of arguments to the program.
    u8 args[ARGBUFSIZE];
    // ArgsLen is the length of the args.
    u64 args_len;
    // ArgsTruncated is true if the args were truncated.
    bool args_truncated;
    // CgroupID is the internal cgroupv2 ID of the event.
    u64 cgroup;
    // AuditSessionID is the audit session ID that is used to correlate
    // events with specific sessions.
    u32 audit_session_id;
};

// Force emitting struct data_t into the ELF. bpf2go needs this
// to generate the Go bindings.
const struct data_t *unused __attribute__((unused));

// Hashmap that keeps all audit session IDs that should be monitored 
// by Teleport.
BPF_HASH(monitored_sessionids, u32, u8, MAX_MONITORED_SESSIONS);

BPF_RING_BUF(execve_events, EVENTS_BUF_SIZE);

BPF_COUNTER(lost);

static int enter_execve(struct trace_event_raw_sched_process_exec *tp)
{
    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    u32 session_id = BPF_CORE_READ(task, sessionid);
    u8 *is_monitored = bpf_map_lookup_elem(&monitored_sessionids, &session_id);
    if (is_monitored == NULL) {
        return 0;
    }

    struct data_t *data = bpf_ringbuf_reserve(&execve_events, sizeof(*data), 0);
    if (!data) {
        INCR_COUNTER(lost);
        bpf_printk("execve_events ring buffer full");
        return 0;
    }

    u64 filename_loc = BPF_CORE_READ(tp, __data_loc_filename) & 0xFFFF;
    bpf_probe_read_str(&data->filename, sizeof(data->filename), (void *)tp + filename_loc);

    void *arg_start = (void *)BPF_CORE_READ(task, mm, arg_start);
    void *arg_end = (void *)BPF_CORE_READ(task, mm, arg_end);
    data->args_len = arg_end - arg_start;
    
    data->args_truncated = false;
    if (data->args_len > ARGBUFSIZE) {
        data->args_len = ARGBUFSIZE;
        data->args_truncated = true;
    }
    int read_ret = bpf_probe_read_user(&data->args, data->args_len, arg_start);
    if (read_ret < 0) {
        data->args_len = 0;
    }

    print_command_event(task, data->filename, data->args);

    data->pid = bpf_get_current_pid_tgid() >> 32;
    data->cgroup = bpf_get_current_cgroup_id();
    data->audit_session_id = session_id;

    data->ppid = BPF_CORE_READ(task, real_parent, tgid);
    bpf_get_current_comm(&data->command, sizeof(data->command));

    bpf_ringbuf_submit(data, 0);

    return 0;
}

SEC("tracepoint/sched/sched_process_exec")
int tracepoint__sched__sched_process_exec(struct trace_event_raw_sched_process_exec *tp)
{
    return enter_execve(tp);
}
