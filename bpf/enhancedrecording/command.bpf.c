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

#define ARGSIZE  128
#define MAXARGS 20

// Size, in bytes, of the ring buffer used to report
// audit events to userspace. This is the default,
// the userspace can adjust this value based on config.
#define EVENTS_BUF_SIZE (4096*8)

char LICENSE[] SEC("license") = "Dual BSD/GPL";

// Maximum number of commands that can be running concurrently
#define MAX_EXEC_PROCS (4096)

BPF_HASH(execed_pids, u32, u8, MAX_EXEC_PROCS);

enum event_type {
    EVENT_ARG,
    EVENT_RET,
    EVENT_EXIT,
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

static inline void submit_event(struct data_t *data)
{
    if (bpf_ringbuf_output(&execve_events, data, sizeof(struct data_t), 0) != 0)
        INCR_COUNTER(lost);
}

static int __submit_arg(void *ptr, struct data_t *data)
{
    bpf_probe_read_user(data->argv, sizeof(data->argv), ptr);
    submit_event(data);
    return 1;
}

static int submit_arg(void *ptr, struct data_t *data)
{
    const char *argp = 0;
    bpf_probe_read_user(&argp, sizeof(argp), ptr);
    if (argp) {
        return __submit_arg((void *)(argp), data);
    }
    return 0;
}

static void fill_event_data_common(struct data_t *data, u32 tgid)
{
    struct task_struct *task;

    data->pid = tgid;
    data->cgroup = bpf_get_current_cgroup_id();

    task = (struct task_struct *)bpf_get_current_task();
    data->ppid = BPF_CORE_READ(task, real_parent, tgid);

    bpf_get_current_comm(&data->comm, sizeof(data->comm));
}

static inline void mark_executed(u32 tgid)
{
    u8 value = 0;
    bpf_map_update_elem(&execed_pids, &tgid, &value, 0);
}

static inline void clear_executed(u32 tgid)
{
    bpf_map_delete_elem(&execed_pids, &tgid);
}

static inline bool is_marked_executed(u32 tgid)
{
    return bpf_map_lookup_elem(&execed_pids, &tgid) != NULL;
}

static int enter_execve(const char *filename,
                const char *const *argv,
                const char *const *envp)
{
    // create data here and pass to submit_arg to save stack space (#555)
    struct data_t data = {};
    struct task_struct *task;

    data.pid = bpf_get_current_pid_tgid() >> 32;
    data.cgroup = bpf_get_current_cgroup_id();

    task = (struct task_struct *)bpf_get_current_task();
    data.ppid = BPF_CORE_READ(task, real_parent, tgid);

    bpf_get_current_comm(&data.comm, sizeof(data.comm));
    data.type = EVENT_ARG;

    __submit_arg((void *)filename, &data);

    // skip first arg, as we submitted filename
    for (int i = 1; i < MAXARGS; i++) {
        if (submit_arg((void *)&argv[i], &data) == 0)
             goto out;
    }

    // handle truncated argument list
    char ellipsis[] = "...";
    __submit_arg((void *)ellipsis, &data);

out:
    mark_executed(data.pid);
    return 0;
}

static int exit_execve(int ret)
{
    struct data_t data = {};
    u64 pid_tgid = bpf_get_current_pid_tgid();
    u32 tgid = (u32) pid_tgid;

    fill_event_data_common(&data, tgid);

    data.type = EVENT_RET;
    data.retval = ret;

    submit_event(&data);

    return 0;
}

// Convert kernel code encoding to exit status used by us.
static inline int decode_exit_code(long code)
{
    if (code & 0xff)
        // killed by a signal. 0x80 bit contains whether it core dumped
        // or not. Don't care about that.
        return -(code & 0x7f);
    else
        return code >> 8; // exited normally
}

SEC("tp/syscalls/sys_execve")
int tracepoint__syscalls__sys_enter_execve(struct trace_event_raw_sys_enter *tp)
{
    const char *filename = (const char *)tp->args[0];
    const char *const *argv = (const char *const *)tp->args[1];
    const char *const *envp = (const char *const *)tp->args[2];

    return enter_execve(filename, argv, envp);
}

SEC("tp/syscalls/sys_exit_execve")
int tracepoint__syscalls__sys_exit_execve(struct trace_event_raw_sys_exit *tp)
{
    return exit_execve(tp->ret);
}

SEC("tp/syscalls/sys_execveat")
int tracepoint__syscalls__sys_enter_execveat(struct trace_event_raw_sys_enter *tp)
{
    const char *filename = (const char *)tp->args[1];
    const char *const *argv = (const char *const *)tp->args[2];
    const char *const *envp = (const char *const *)tp->args[3];

    return enter_execve(filename, argv, envp);
}

SEC("tp/syscalls/sys_exit_execveat")
int tracepoint__syscalls__sys_exit_execveat(struct trace_event_raw_sys_exit *tp)
{
    return exit_execve(tp->ret);
}

SEC("kprobe/do_exit")
int BPF_KPROBE(kprobe__do_exit, long code)
{
    struct data_t data = {};

    u64 pid_tgid = bpf_get_current_pid_tgid();
    u32 tgid = pid_tgid >> 32;
    u32 pid = (u32) pid_tgid;
    if (pid != tgid)
        // Non-main thread exiting, ignore
        return 0;

    if (!is_marked_executed(tgid))
        // Process never exec'ed anything
        return 0;

    fill_event_data_common(&data, tgid);

    data.type = EVENT_EXIT;
    data.retval = decode_exit_code(code);
    submit_event(&data);

    clear_executed(tgid);

    return 0;
}
