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

	"github.com/gravitational/trace"

	"github.com/iovisor/gobpf/bcc"
	"github.com/sirupsen/logrus"
)

const (
	// commMax is the maximum length of a command from linux/sched.h.
	commMax = 16

	// pathMax is the maximum length of a path from linux/limits.h.
	pathMax = 255
)

type openEvent struct {
	PID        uint64
	ReturnCode int32
	CgroupID   uint32
	Command    [commMax]byte
	Path       [pathMax]byte
	Flags      int32
}

type open struct {
	service *bpf.Service

	perfMap *bcc.PerfMap
	module  *bcc.Module
}

func newOpen(service *bpf.Service) *open {
	return &open{
		closeContext: closeContext,
	}
}

func (e *open) Start() error {
	e.module = bcc.NewModule(openSource, []string{})

	// Enter open syscall.
	kprobe, err := e.module.LoadKprobe("trace_entry")
	if err != nil {
		return trace.Wrap(err)
	}
	err = e.module.AttachKprobe("do_sys_open", kprobe, -1)
	if err != nil {
		return trace.Wrap(err)
	}

	// Return from open syscall.
	kretprobe, err := e.module.LoadKprobe("trace_return")
	if err != nil {
		return trace.Wrap(err)
	}
	err = e.module.AttachKretprobe("do_sys_open", kretprobe, -1)
	if err != nil {
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

func (e *open) start(eventCh <-chan []byte) {
	for {
		select {
		case eventBytes := <-eventCh:
			var event openEvent

			err := binary.Read(bytes.NewBuffer(eventBytes), bcc.GetHostByteOrder(), &event)
			if err != nil {
				logrus.Debugf("Failed to read binary data: %v.", err)
				fmt.Printf("Failed to read binary data: %v.\n", err)
				continue
			}

			// Convert C string that holds the command name into a Go string.
			command := C.GoString((*C.char)(unsafe.Pointer(&event.Command)))

			// Convert C string that holds the path into a Go string.
			path := C.GoString((*C.char)(unsafe.Pointer(&event.Path)))

			fmt.Printf("--> Event=open CgroupID=%v PID=%v Command=%v ReturnCode=%v Flags=%#o Path=%v.\n",
				event.CgroupID, event.PID, command, event.ReturnCode, event.Flags, path)
		}
	}
}

// TODO(russjones): Make sure this program is actually unloaded upon exit.
func (e *open) Close() {
	e.perfMap.Stop()
	e.module.Close()
}

const openSource string = `
#include <uapi/linux/ptrace.h>
#include <uapi/linux/limits.h>
#include <linux/sched.h>
#include <linux/fs.h>
#include <linux/audit.h>

struct val_t {
    u64 pid;
    char comm[TASK_COMM_LEN];
    const char *fname;
    int flags;
};

struct data_t {
    u64 pid;
    int ret;
    u32 cgroup;
    char comm[TASK_COMM_LEN];
    char fname[NAME_MAX];
    int flags;
};

BPF_HASH(infotmp, u64, struct val_t);
BPF_PERF_OUTPUT(events);

int trace_entry(struct pt_regs *ctx, int dfd, const char __user *filename, int flags)
{
    struct val_t val = {};
    u64 id = bpf_get_current_pid_tgid();

    if (bpf_get_current_comm(&val.comm, sizeof(val.comm)) == 0) {
        val.pid = id >> 32;
        val.fname = filename;
        val.flags = flags;
        infotmp.update(&id, &val);
    }

    return 0;
};

int trace_return(struct pt_regs *ctx)
{
    u64 id = bpf_get_current_pid_tgid();
    struct val_t *valp;
    struct data_t data = {};

    valp = infotmp.lookup(&id);
    if (valp == 0) {
        // Missed entry.
        return 0;
    }
    bpf_probe_read(&data.comm, sizeof(data.comm), valp->comm);
    bpf_probe_read(&data.fname, sizeof(data.fname), (void *)valp->fname);
    data.pid = valp->pid;
    data.flags = valp->flags;
    data.ret = PT_REGS_RC(ctx);
    data.cgroup = bpf_get_current_cgroup_id();

    events.perf_submit(ctx, &data, sizeof(data));
    infotmp.delete(&id);

    return 0;
}
`
