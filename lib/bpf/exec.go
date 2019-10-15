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
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"unsafe"

	"github.com/gravitational/teleport/lib/bpf"

	"github.com/gravitational/trace"

	"github.com/iovisor/gobpf/bcc"
	"github.com/sirupsen/logrus"
)

const (
	eventArg = 0
	eventRet = 1
)

// rawExecEvent is sent by the eBPF program that Teleport pulls off the perf
// buffer.
type rawExecEvent struct {
	// PID is the ID of the process.
	PID uint64

	// PPID is the PID of the parent process.
	PPID uint64

	// Command is the executable.
	Command [16]byte

	// Type is the type of event.
	Type int32

	// Argv is the list of arguments to the program.
	Argv [128]byte

	// ReturnCode is the return code of execve.
	ReturnCode int32

	// CgroupID is the internal cgroupv2 ID of the event.
	CgroupID uint32
}

// execEvent is the parsed execve event.
type execEvent struct {
	// PID is the ID of the process.
	PID uint64

	// PPID is the PID of the parent process.
	PPID uint64

	// Program is name of the executable.
	Program string

	// Path is the full path to the executable.
	Path string

	// Argv is the list of arguments to the program. Note, the first element does
	// not contain the name of the process.
	Argv []string

	// ReturnCode is the return code of execve.
	ReturnCode int32

	// CgroupID is the internal cgroupv2 ID of the event.
	CgroupID uint32
}

type exec struct {
	closeContext context.Context

	events chan *execEvent

	perfMap *bcc.PerfMap
	module  *bcc.Module
}

func newExec(closeContext context.Context) *exec {
	return &exec{
		closeContext: closeContext,
		events:       make(chan *execEvent, 1024),
	}
}

func (e *exec) Start() error {
	e.module = bcc.NewModule(execveSource, []string{})

	fnName := bcc.GetSyscallFnName("execve")

	kprobe, err := e.module.LoadKprobe("syscall__execve")
	if err != nil {
		return trace.Wrap(err)
	}

	// passing -1 for maxActive signifies to use the default
	// according to the kernel kprobes documentation
	if err := e.module.AttachKprobe(fnName, kprobe, -1); err != nil {
		return trace.Wrap(err)
	}

	kretprobe, err := e.module.LoadKprobe("do_ret_sys_execve")
	if err != nil {
		return trace.Wrap(err)
	}

	// passing -1 for maxActive signifies to use the default
	// according to the kernel kretprobes documentation
	if err := e.module.AttachKretprobe(fnName, kretprobe, -1); err != nil {
		return trace.Wrap(err)
	}

	eventCh := make(chan []byte, 1024)
	table := bcc.NewTable(e.module.TableId("events"), e.module)

	perfMap, err := bcc.InitPerfMap(table, eventCh)
	if err != nil {
		return trace.Wrap(err)
	}
	perfMap.Start()

	go e.start(eventCh)

	return nil
}

func (e *exec) start(eventCh <-chan []byte) {
	// TODO(russjones): Replace with ttlmap.
	args := make(map[uint64][]string)

	for {
		select {
		case eventBytes := <-eventCh:
			var event rawExecEvent

			err := binary.Read(bytes.NewBuffer(eventBytes), bcc.GetHostByteOrder(), &event)
			if err != nil {
				logrus.Debugf("Failed to read binary data: %v.", err)
				continue
			}

			if eventArg == event.Type {
				buf, ok := args[event.PID]
				if !ok {
					buf = make([]string, 0)
				}

				argv := (*C.char)(unsafe.Pointer(&event.Argv))
				buf = append(buf, C.GoString(argv))
				args[event.PID] = buf
			} else {
				// The args should have come in a previous event, find them by PID.
				argv, ok := args[event.PID]
				if !ok {
					logrus.Debugf("Got event with missing args: skipping.")
					continue
				}

				// Convert C string that holds the command name into a Go string.
				path := C.GoString((*C.char)(unsafe.Pointer(&event.Command)))

				select {
				case e.events <- &execEvent{
					PID:        event.PPID,
					PPID:       event.PID,
					CgroupID:   event.CgroupID,
					Program:    filepath.Base(path),
					Path:       path,
					Argv:       argv,
					ReturnCode: event.ReturnCode,
				}:
				case <-time.After(100 * time.Millisecond):
					log.Debugf("Dropping event, timeout, buffer probably full.")
				}

				//fmt.Printf("--> Event=exec CgroupID=%v PID=%v PPID=%v Command=%v, Args=%v, ReturnCode=%v.\n",
				//	event.CgroupID, event.PID, event.PPID, command, argv, event.ReturnCode)
			}
		}
	}
}

// TODO(russjones): Make sure this program is actually unloaded upon exit.
func (e *exec) Close() {
	e.perfMap.Stop()
	e.module.Close()
}

func (e *exec) events() <-chan *execEvent {
	return e.events
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
    u32 cgroup;
};

BPF_PERF_OUTPUT(events);

static int __submit_arg(struct pt_regs *ctx, void *ptr, struct data_t *data)
{
    bpf_probe_read(data->argv, sizeof(data->argv), ptr);
    events.perf_submit(ctx, data, sizeof(struct data_t));
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
    events.perf_submit(ctx, &data, sizeof(data));

    return 0;
}
`
