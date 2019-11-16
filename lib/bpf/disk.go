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
	lostDiskEvents = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: teleport.MetricLostDiskEvents,
			Help: "Number of lost disk events.",
		},
	)
)

func init() {
	prometheus.MustRegister(lostDiskEvents)
}

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

// exec runs a BPF program (opensnoop) that hooks execve.
type open struct {
	closeContext context.Context

	eventCh <-chan []byte
	lostCh  <-chan uint64

	perfMaps []*bcc.PerfMap
	module   *bcc.Module
}

// startOpen will compile, load, start, and pull events off the perf buffer
// for the BPF program.
func startOpen(closeContext context.Context, pageCount int) (*open, error) {
	var err error

	e := &open{
		closeContext: closeContext,
	}

	// Compile the BPF program.
	e.module = bcc.NewModule(openSource, []string{})
	if e.module == nil {
		return nil, trace.BadParameter("failed to load libbcc")
	}

	// Hook open syscall.
	err = attachProbe(e.module, "do_sys_open", "trace_entry")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = attachRetProbe(e.module, "do_sys_open", "trace_return")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Open perf buffer and start processing open events.
	e.eventCh, e.lostCh, err = openPerfBuffer(e.module, e.perfMaps, pageCount, "open_events")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Start a loop that will emit lost events to prometheus.
	go e.lostLoop()

	return e, nil
}

// close will stop reading events off the perf buffer and unload the BPF
// program.
func (e *open) close() {
	for _, perfMap := range e.perfMaps {
		perfMap.Stop()
	}
	e.module.Close()
}

// lostLoop keeps emitting the number of lost events to prometheus.
func (e *open) lostLoop() {
	for {
		select {
		case n := <-e.lostCh:
			log.Debugf("Lost %v disk events.", n)
			lostDiskEvents.Add(float64(n))
		case <-e.closeContext.Done():
			return
		}
	}
}

// events contains raw events off the perf buffer.
func (e *open) events() <-chan []byte {
	return e.eventCh
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
}`
