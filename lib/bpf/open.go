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
	"unsafe"

	"github.com/gravitational/trace"

	"github.com/iovisor/gobpf/bcc"
)

// rawOpenEvent is sent by the eBPF program that Teleport pulls off the perf
// buffer.
type rawOpenEvent struct {
	// CgroupID is the internal cgroupv2 ID of the event.
	CgroupID uint64

	// PID is the ID of the process.
	PID uint64

	// ReturnCode is the return code of open.
	ReturnCode int32

	// Command is name of the executable opening the file.
	Command [commMax]byte

	// Path is the full path to the file being opened.
	Path [pathMax]byte

	// Flags are the flags passed to open.
	Flags int32
}

// openEvent is a parsed open event.
type openEvent struct {
	// PID is the ID of the process.
	PID uint64

	// ReturnCode is the return code of open.
	ReturnCode int32

	// Program is name of the executable opening the file.
	Program string

	// Path is the full path to the file being opened.
	Path string

	// Flags are the flags passed to open.
	Flags int32

	// CgroupID is the internal cgroupv2 ID of the event.
	CgroupID uint64
}

type open struct {
	closeContext context.Context

	events chan *openEvent

	perfMaps []*bcc.PerfMap
	module   *bcc.Module
}

func newOpen(closeContext context.Context) (*open, error) {
	e := &open{
		closeContext: closeContext,
		events:       make(chan *openEvent, bufferSize),
	}

	err := e.start()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return e, nil
}

func (e *open) start() error {
	e.module = bcc.NewModule(openSource, []string{})

	// Hook open syscall.
	err := attachProbe(e.module, "do_sys_open", "trace_entry")
	if err != nil {
		return trace.Wrap(err)
	}
	err = attachRetProbe(e.module, "do_sys_open", "trace_return")
	if err != nil {
		return trace.Wrap(err)
	}

	// Open perf buffer and start processing open events.
	eventCh, err := openPerfBuffer(e.module, e.perfMaps, "open_events")
	if err != nil {
		return trace.Wrap(err)
	}
	go e.handleEvents(eventCh)

	return nil
}

func (e *open) handleEvents(eventCh <-chan []byte) {
	for {
		select {
		case eventBytes := <-eventCh:
			go func() {
				var event rawOpenEvent

				err := binary.Read(bytes.NewBuffer(eventBytes), bcc.GetHostByteOrder(), &event)
				if err != nil {
					log.Debugf("Failed to read binary data: %v.", err)
					return
				}

				// Convert C string that holds the command name into a Go string.
				command := C.GoString((*C.char)(unsafe.Pointer(&event.Command)))

				// Convert C string that holds the path into a Go string.
				path := C.GoString((*C.char)(unsafe.Pointer(&event.Path)))

				select {
				case e.events <- &openEvent{
					PID:        event.PID,
					ReturnCode: event.ReturnCode,
					Program:    command,
					Path:       path,
					Flags:      event.Flags,
					CgroupID:   event.CgroupID,
				}:
				case <-e.closeContext.Done():
					return
				default:
					log.Warnf("Dropping open event %v/%v %v %v, events buffer full.", event.CgroupID, event.PID, path, event.Flags)
				}

				//// Remove, only for debugging.
				//fmt.Printf("Event=open CgroupID=%v PID=%v Command=%v ReturnCode=%v Flags=%#o Path=%v.\n",
				//	event.CgroupID, event.PID, command, event.ReturnCode, event.Flags, path)
			}()
		}
	}
}

// TODO(russjones): Make sure this program is actually unloaded upon exit.
func (e *open) close() {
	for _, perfMap := range e.perfMaps {
		perfMap.Stop()
	}
	e.module.Close()
}

func (e *open) eventsCh() <-chan *openEvent {
	return e.events
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
    u64 cgroup;
    u64 pid;
    int ret;
    char comm[TASK_COMM_LEN];
    char fname[NAME_MAX];
    int flags;
};

BPF_HASH(infotmp, u64, struct val_t);
BPF_PERF_OUTPUT(open_events);

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

    open_events.perf_submit(ctx, &data, sizeof(data));
    infotmp.delete(&id);

    return 0;
}
`
