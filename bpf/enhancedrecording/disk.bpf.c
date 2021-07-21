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
#include <linux/limits.h>
#include <bpf/bpf_helpers.h>       /* most used helpers: SEC, __always_inline, etc */
#include <bpf/bpf_core_read.h>     /* for BPF CO-RE helpers */
#include <bpf/bpf_tracing.h>       /* for getting kprobe arguments */

#include "../helpers.h"

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
    u64 cgroup;
    u64 pid;
    int ret;
    char comm[TASK_COMM_LEN];
    char fname[NAME_MAX];
    int flags;
};

BPF_HASH(infotmp, u64, struct val_t, INFLIGHT_MAX);

// open_events ring buffer
BPF_RING_BUF(open_events, EVENTS_BUF_SIZE);

BPF_COUNTER(lost);

static int enter_open(const char *filename, int flags) {
    struct val_t val = {};
    u64 id = bpf_get_current_pid_tgid();

    val.pid = id >> 32;
    val.fname = filename;
    val.flags = flags;
    bpf_map_update_elem(&infotmp, &id, &val, 0);

    return 0;
}

static int exit_open(int ret) {
    u64 id = bpf_get_current_pid_tgid();
    struct val_t *valp;
    struct data_t data = {};

    valp = bpf_map_lookup_elem(&infotmp, &id);
    if (valp == 0) {
        // Missed entry.
        return 0;
    }

    if (bpf_get_current_comm(&data.comm, sizeof(data.comm)) != 0) {
        data.comm[0] = '\0';
    }

    bpf_probe_read_user(&data.fname, sizeof(data.fname), (void *)valp->fname);

    data.pid = valp->pid;
    data.flags = valp->flags;
    data.ret = ret;
    data.cgroup = bpf_get_current_cgroup_id();

    if (bpf_ringbuf_output(&open_events, &data, sizeof(data), 0) != 0)
        INCR_COUNTER(lost);

    bpf_map_delete_elem(&infotmp, &id);

    return 0;
}


SEC("tp/syscalls/sys_enter_creat")
int tracepoint__syscalls__sys_enter_creat(struct trace_event_raw_sys_enter *tp)
{
    const char *filename = (const char*) tp->args[0];

    return enter_open(filename, 0);
}

SEC("tp/syscalls/sys_exit_creat")
int tracepoint__syscalls__sys_exit_creat(struct trace_event_raw_sys_exit *tp)
{
    return exit_open(tp->ret);
}

SEC("tp/syscalls/sys_enter_open")
int tracepoint__syscalls__sys_enter_open(struct trace_event_raw_sys_enter *tp)
{
    const char *filename = (const char*) tp->args[0];
    int flags = tp->args[1];

    return enter_open(filename, flags);
};

SEC("tp/syscalls/sys_exit_open")
int tracepoint__syscalls__sys_exit_open(struct trace_event_raw_sys_exit *tp)
{
    return exit_open(tp->ret);
}

SEC("tp/syscalls/sys_enter_openat")
int tracepoint__syscalls__sys_enter_openat(struct trace_event_raw_sys_enter *tp)
{
    const char *filename = (const char*) tp->args[1];
    int flags = tp->args[2];

    return enter_open(filename, flags);
};

SEC("tp/syscalls/sys_exit_openat")
int tracepoint__syscalls__sys_exit_openat(struct trace_event_raw_sys_exit *tp)
{
    return exit_open(tp->ret);
}

SEC("tp/syscalls/sys_enter_openat2")
int tracepoint__syscalls__sys_enter_openat2(struct trace_event_raw_sys_enter *tp)
{
    const char *filename = (const char*) tp->args[1];
    struct open_how *how = (struct open_how *) tp->args[2];

    return enter_open(filename, BPF_CORE_READ(how, flags));
};

SEC("tp/syscalls/sys_exit_openat2")
int tracepoint__syscalls__sys_exit_openat2(struct trace_event_raw_sys_exit *tp)
{
    return exit_open(tp->ret);
}
