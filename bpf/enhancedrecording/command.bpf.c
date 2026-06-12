#include "../vmlinux.h"
#include <bpf/bpf_helpers.h>       /* most used helpers: SEC, __always_inline, etc */
#include <bpf/bpf_core_read.h>     /* for BPF CO-RE helpers */
#include <bpf/bpf_tracing.h>       /* for getting kprobe arguments */

#include "./common.h"
#include "../helpers.h"

// ARGSIZE specifies the max argument size read. This value must match the
// `ArgvMax` Golang constant.
#define ARGSIZE 1024
#define MAXARGS 20

// Size, in bytes, of the ring buffer used to report
// audit events to userspace. This is the default,
// the userspace can adjust this value based on config.
#define EVENTS_BUF_SIZE (4096*8)

// hashmap keeps all cgroups id that should be monitored by Teleport.
BPF_HASH(monitored_cgroups, u64, int64_t, MAX_MONITORED_SESSIONS);

char LICENSE[] SEC("license") = "Dual BSD/GPL";

enum event_type {
    EVENT_ARG,
    EVENT_RET,
};

// common_data_t is a struct used to store fields that are common across
// multiple events. Those fields are the same as `data_t`.
struct common_data_t {
    u64 pid;
    u64 ppid;
    char comm[TASK_COMM_LEN];
    u64 cgroup;
};

struct data_t {
    // pid as in the userspace term (i.e. task->tgid in kernel).
    u64 pid;
    // ppid is the userspace term (i.e task->real_parent->tgid in kernel).
    u64 ppid;
    char comm[TASK_COMM_LEN];
    enum event_type type;
    char argv[ARGSIZE];
    int retval;
    u64 cgroup;
};

BPF_RING_BUF(execve_events, EVENTS_BUF_SIZE);

BPF_COUNTER(lost);

static int __submit_arg(void *ptr, struct common_data_t *common)
{
    struct data_t *data = bpf_ringbuf_reserve(&execve_events, sizeof(*data), 0);
    if (!data) {
        return -1;
    }

    if (bpf_probe_read_user(data->argv, sizeof(data->argv), ptr) < 0) {
        bpf_ringbuf_discard(data, 0);
        return -1;
    }

    data->type = EVENT_ARG;
    data->pid = common->pid;
    data->cgroup = common->cgroup;
    for (int i = 0; i < TASK_COMM_LEN; i++)
        data->comm[i] = common->comm[i];

    bpf_ringbuf_submit(data, 0);
    return 1;
}

static int submit_arg(void *ptr, struct common_data_t *common)
{
    const char *argp = 0;
    bpf_probe_read_user(&argp, sizeof(argp), ptr);
    if (argp) {
        return __submit_arg((void *)(argp), common);
    }
    return 0;
}

static int enter_execve(const char *filename,
                const char *const *argv,
                const char *const *envp)
{
    struct common_data_t common = {};
    struct task_struct *task;
    u64 cgroup = bpf_get_current_cgroup_id();
    u64 *is_monitored;

    // Check if the cgroup should be monitored.
    is_monitored = bpf_map_lookup_elem(&monitored_cgroups, &cgroup);
    if (is_monitored == NULL) {
        // Missed entry.
        return 0;
    }

    common.pid = bpf_get_current_pid_tgid() >> 32;
    common.cgroup = cgroup;

    task = (struct task_struct *)bpf_get_current_task();
    common.ppid = BPF_CORE_READ(task, real_parent, tgid);
    bpf_get_current_comm(&common.comm, sizeof(common.comm));

    if(__submit_arg((void *)filename, &common) < 0) {
        INCR_COUNTER(lost);
        goto out;
    }

    for (int i = 1; i < MAXARGS; i++) {
        int res = submit_arg((void *)&argv[i], &common);
        if (res < 0) {
            INCR_COUNTER(lost);
            goto out;
        }

        // If no arguments were sent, we reached the end of the arguments list.
        if (res == 0)
             goto out;
    }

    // handle truncated argument list
    char ellipsis[] = "...";
    if (__submit_arg((void *)ellipsis, &common) < 0)
        INCR_COUNTER(lost);
out:
    return 0;
}

static int exit_execve(int ret)
{
    struct task_struct *task;
    u64 cgroup = bpf_get_current_cgroup_id();
    u64 *is_monitored;

    // Check if the cgroup should be monitored.
    is_monitored = bpf_map_lookup_elem(&monitored_cgroups, &cgroup);
    if (is_monitored == NULL) {
        // cgroup has not been marked for monitoring, ignore.
        return 0;
    }

    struct data_t *data = bpf_ringbuf_reserve(&execve_events, sizeof(*data), 0);
    if (!data) {
        INCR_COUNTER(lost);
        return 0;
    }

    data->pid = bpf_get_current_pid_tgid() >> 32;
    data->cgroup = cgroup;

    task = (struct task_struct *)bpf_get_current_task();
    data->ppid = BPF_CORE_READ(task, real_parent, tgid);

    bpf_get_current_comm(&data->comm, sizeof(data->comm));
    data->type = EVENT_RET;
    data->retval = ret;

    bpf_ringbuf_submit(data, 0);
    return 0;
}

SEC("tp/syscalls/sys_execve")
int tracepoint__syscalls__sys_enter_execve(struct syscall_trace_enter *tp)
{
    const char *filename = (const char *)tp->args[0];
    const char *const *argv = (const char *const *)tp->args[1];
    const char *const *envp = (const char *const *)tp->args[2];

    return enter_execve(filename, argv, envp);
}

SEC("tp/syscalls/sys_exit_execve")
int tracepoint__syscalls__sys_exit_execve(struct syscall_trace_exit *tp)
{
    return exit_execve(tp->ret);
}

SEC("tp/syscalls/sys_execveat")
int tracepoint__syscalls__sys_enter_execveat(struct syscall_trace_enter *tp)
{
    const char *filename = (const char *)tp->args[1];
    const char *const *argv = (const char *const *)tp->args[2];
    const char *const *envp = (const char *const *)tp->args[3];

    return enter_execve(filename, argv, envp);
}

SEC("tp/syscalls/sys_exit_execveat")
int tracepoint__syscalls__sys_exit_execveat(struct syscall_trace_exit *tp)
{
    return exit_execve(tp->ret);
}
