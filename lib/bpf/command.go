// +build bpf,linux

/*
Copyright 2019 Gravitational, Inc.

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

package bpf

import "C"

import (
	"context"

	"github.com/gravitational/teleport"

	"github.com/gravitational/trace"

	"github.com/iovisor/gobpf/bcc"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	lostCommandEvents = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: teleport.MetricLostCommandEvents,
			Help: "Number of lost command events.",
		},
	)
)

func init() {
	prometheus.MustRegister(lostCommandEvents)
}

// rawExecEvent is sent by the eBPF program that Teleport pulls off the perf
// buffer.
type rawExecEvent struct {
	// PID is the ID of the process.
	PID uint64

	// PPID is the PID of the parent process.
	PPID uint64

	// Command is the executable.
	Command [commMax]byte

	// Type is the type of event.
	Type int32

	// Argv is the list of arguments to the program.
	Argv [argvMax]byte

	// ReturnCode is the return code of execve.
	ReturnCode int32

	// CgroupID is the internal cgroupv2 ID of the event.
	CgroupID uint64
}

// exec runs a BPF program (execsnoop) that hooks execve.
type exec struct {
	closeContext context.Context

	eventCh <-chan []byte
	lostCh  <-chan uint64

	perfMaps []*bcc.PerfMap
	module   *bcc.Module
}

// startExec will compile, load, start, and pull events off the perf buffer
// for the BPF program.
func startExec(closeContext context.Context, pageCount int) (*exec, error) {
	var err error

	e := &exec{
		closeContext: closeContext,
	}

	// Compile the BPF program.
	e.module = bcc.NewModule(execveSource, []string{})
	if e.module == nil {
		return nil, trace.BadParameter("failed to load libbcc")
	}

	// Hook execve syscall.
	err = attachProbe(e.module, bcc.GetSyscallFnName("execve"), "syscall__execve")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = attachRetProbe(e.module, bcc.GetSyscallFnName("execve"), "do_ret_sys_execve")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Open perf buffer and start processing execve events.
	e.eventCh, e.lostCh, err = openPerfBuffer(e.module, e.perfMaps, pageCount, "execve_events")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Start a loop that will emit lost events to prometheus.
	go e.lostLoop()

	return e, nil
}

// close will stop reading events off the perf buffer and unload the BPF
// program.
func (e *exec) close() {
	for _, perfMap := range e.perfMaps {
		perfMap.Stop()
	}
	e.module.Close()
}

// events contains raw events off the perf buffer.
func (e *exec) events() <-chan []byte {
	return e.eventCh
}

// lostLoop keeps emitting the number of lost events to prometheus.
func (e *exec) lostLoop() {
	for {
		select {
		case n := <-e.lostCh:
			log.Debugf("Lost %v command events.", n)
			lostCommandEvents.Add(float64(n))
		case <-e.closeContext.Done():
			return
		}
	}
}

const execveSource string = `
#include <uapi/linux/ptrace.h>
#include <linux/sched.h>
#include <linux/fs.h>

#define ARGSIZE  128

enum event_type {
    EVENT_ARG,
    EVENT_RET,
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

BPF_PERF_OUTPUT(execve_events);

static int __submit_arg(struct pt_regs *ctx, void *ptr, struct data_t *data)
{
    bpf_probe_read(data->argv, sizeof(data->argv), ptr);
    execve_events.perf_submit(ctx, data, sizeof(struct data_t));
    return 1;
}

static int submit_arg(struct pt_regs *ctx, void *ptr, struct data_t *data)
{
    const char *argp = NULL;
    bpf_probe_read(&argp, sizeof(argp), ptr);
    if (argp) {
        return __submit_arg(ctx, (void *)(argp), data);
    }
    return 0;
}

int syscall__execve(struct pt_regs *ctx,
    const char __user *filename,
    const char __user *const __user *__argv,
    const char __user *const __user *__envp)
{
    // create data here and pass to submit_arg to save stack space (#555)
    struct data_t data = {};
    struct task_struct *task;

    data.pid = bpf_get_current_pid_tgid() >> 32;
    data.cgroup = bpf_get_current_cgroup_id();

    task = (struct task_struct *)bpf_get_current_task();
    // Some kernels, like Ubuntu 4.13.0-generic, return 0
    // as the real_parent->tgid.
    // We use the getPpid function as a fallback in those cases.
    // See https://github.com/iovisor/bcc/issues/1883.
    data.ppid = task->real_parent->tgid;

    bpf_get_current_comm(&data.comm, sizeof(data.comm));
    data.type = EVENT_ARG;

    __submit_arg(ctx, (void *)filename, &data);

    // skip first arg, as we submitted filename
    #pragma unroll
    for (int i = 1; i < 20; i++) {
        if (submit_arg(ctx, (void *)&__argv[i], &data) == 0)
             goto out;
    }

    // handle truncated argument list
    char ellipsis[] = "...";
    __submit_arg(ctx, (void *)ellipsis, &data);
out:
    return 0;
}

int do_ret_sys_execve(struct pt_regs *ctx)
{
    struct data_t data = {};
    struct task_struct *task;

    data.pid = bpf_get_current_pid_tgid() >> 32;
    data.cgroup = bpf_get_current_cgroup_id();

    task = (struct task_struct *)bpf_get_current_task();
    // Some kernels, like Ubuntu 4.13.0-generic, return 0
    // as the real_parent->tgid.
    // We use the getPpid function as a fallback in those cases.
    // See https://github.com/iovisor/bcc/issues/1883.
    data.ppid = task->real_parent->tgid;

    bpf_get_current_comm(&data.comm, sizeof(data.comm));
    data.type = EVENT_RET;
    data.retval = PT_REGS_RC(ctx);
    execve_events.perf_submit(ctx, &data, sizeof(data));

    return 0;
}`
