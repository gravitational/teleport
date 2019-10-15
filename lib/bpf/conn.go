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
	"net"
	"unsafe"

	"github.com/gravitational/trace"

	"github.com/iovisor/gobpf/bcc"
	"github.com/sirupsen/logrus"
)

// separate data structs for ipv4 and ipv6
type conn4Event struct {
	PID      uint32
	CgroupID uint32
	SrcAddr  uint32
	DstAddr  uint32
	Version  uint64
	DstPort  uint16
	Command  [commMax]byte
}

type conn6Event struct {
	PID      uint32
	CgroupID uint32
	SrcAddr  [4]uint32
	DstAddr  [4]uint32
	Version  uint64
	DstPort  uint16
	Command  [commMax]byte
}

type conn struct {
	service *bpf.Service

	module   *bcc.Module
	perfMaps []*bcc.PerfMap
}

func newConn(service *bpf.Service) *conn {
	return &conn{
		closeContext: closeContext,
	}
}

func (e *conn) Start() error {
	e.module = bcc.NewModule(connSource, []string{})

	// Hook IPv4 connection attempts.
	err := e.attachProbe("tcp_v4_connect", "trace_connect_entry")
	if err != nil {
		return trace.Wrap(err)
	}
	err = e.attachRetProbe("tcp_v4_connect", "trace_connect_v4_return")
	if err != nil {
		return trace.Wrap(err)
	}

	// Hook IPv6 connection attempts.
	err = e.attachProbe("tcp_v6_connect", "trace_connect_entry")
	if err != nil {
		return trace.Wrap(err)
	}
	err = e.attachRetProbe("tcp_v6_connect", "trace_connect_v6_return")
	if err != nil {
		return trace.Wrap(err)
	}

	// Open perf buffer and start processing IPv4 events.
	v4EventCh, err := e.openPerfBuffer("ipv4_events")
	if err != nil {
		return trace.Wrap(err)
	}
	go e.handle4Events(v4EventCh)

	// Open perf buffer and start processing IPv6 events.
	v6EventCh, err := e.openPerfBuffer("ipv6_events")
	if err != nil {
		return trace.Wrap(err)
	}
	go e.handle6Events(v6EventCh)

	return nil
}

func (e *conn) handle4Events(eventCh <-chan []byte) {
	for {
		select {
		case eventBytes := <-eventCh:
			var event conn4Event

			err := binary.Read(bytes.NewBuffer(eventBytes), bcc.GetHostByteOrder(), &event)
			if err != nil {
				logrus.Debugf("Failed to read binary data: %v.", err)
				fmt.Printf("Failed to read binary data: %v.\n", err)
				continue
			}

			// Source.
			src := make([]byte, 4)
			binary.LittleEndian.PutUint32(src, uint32(event.SrcAddr))
			srcAddr := net.IP(src)

			// Destination.
			dst := make([]byte, 4)
			binary.LittleEndian.PutUint32(dst, uint32(event.DstAddr))
			dstAddr := net.IP(dst)

			// Convert C string that holds the command name into a Go string.
			command := C.GoString((*C.char)(unsafe.Pointer(&event.Command)))

			fmt.Printf("--> Event=conn4 CgroupID=%v PID=%v Command=%v Src=%v Dst=%v:%v.\n",
				event.CgroupID, event.PID, command, srcAddr, dstAddr, event.DstPort)
		}
	}
}

func (e *conn) handle6Events(eventCh <-chan []byte) {
	for {
		select {
		case eventBytes := <-eventCh:
			var event conn6Event

			err := binary.Read(bytes.NewBuffer(eventBytes), bcc.GetHostByteOrder(), &event)
			if err != nil {
				logrus.Debugf("Failed to read binary data: %v.", err)
				fmt.Printf("Failed to read binary data: %v.\n", err)
				continue
			}

			// Source.
			src := make([]byte, 16)
			binary.LittleEndian.PutUint32(src[0:], event.SrcAddr[0])
			binary.LittleEndian.PutUint32(src[4:], event.SrcAddr[1])
			binary.LittleEndian.PutUint32(src[8:], event.SrcAddr[2])
			binary.LittleEndian.PutUint32(src[12:], event.SrcAddr[3])
			srcAddr := net.IP(src)

			// Destination.
			dst := make([]byte, 16)
			binary.LittleEndian.PutUint32(dst[0:], event.DstAddr[0])
			binary.LittleEndian.PutUint32(dst[4:], event.DstAddr[1])
			binary.LittleEndian.PutUint32(dst[8:], event.DstAddr[2])
			binary.LittleEndian.PutUint32(dst[12:], event.DstAddr[3])
			dstAddr := net.IP(dst)

			// Convert C string that holds the command name into a Go string.
			command := C.GoString((*C.char)(unsafe.Pointer(&event.Command)))

			fmt.Printf("--> Event=conn6 CgroupID=%v PID=%v Command=%v Src=%v Dst=%v:%v.\n",
				event.CgroupID, event.PID, command, srcAddr, dstAddr, event.DstPort)
		}
	}
}

// TODO(russjones): Make sure this program is actually unloaded upon exit.
func (e *conn) Close() {
	for _, perfMap := range e.perfMaps {
		perfMap.Stop()
	}
	e.module.Close()
}

func (e *conn) attachProbe(eventName string, functionName string) error {
	kprobe, err := e.module.LoadKprobe(functionName)
	if err != nil {
		return trace.Wrap(err)
	}

	err = e.module.AttachKprobe(eventName, kprobe, -1)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (e *conn) attachRetProbe(eventName string, functionName string) error {
	kretprobe, err := e.module.LoadKprobe(functionName)
	if err != nil {
		return trace.Wrap(err)
	}

	err = e.module.AttachKretprobe(eventName, kretprobe, -1)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (e *conn) openPerfBuffer(name string) (<-chan []byte, error) {
	var err error

	eventCh := make(chan []byte, 1024)
	table := bcc.NewTable(e.module.TableId(name), e.module)

	perfMap, err := bcc.InitPerfMap(table, eventCh)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	perfMap.Start()

	e.perfMaps = append(e.perfMaps, perfMap)

	return eventCh, nil
}

const connSource string = `
#include <uapi/linux/ptrace.h>
#include <net/sock.h>
#include <bcc/proto.h>

BPF_HASH(currsock, u32, struct sock *);

// separate data structs for ipv4 and ipv6
struct ipv4_data_t {
    u32 pid;
    u32 cgroup;
    u32 saddr;
    u32 daddr;
    u64 ip;
    u16 dport;
    char task[TASK_COMM_LEN];
};
BPF_PERF_OUTPUT(ipv4_events);

struct ipv6_data_t {
    u32 pid;
    u32 cgroup;
    u32 saddr[4];
    u32 daddr[4];
    u64 ip;
    u16 dport;
    char task[TASK_COMM_LEN];
};
BPF_PERF_OUTPUT(ipv6_events);

int trace_connect_entry(struct pt_regs *ctx, struct sock *sk)
{
    u32 pid = bpf_get_current_pid_tgid();

    // Stash the sock ptr for lookup on return.
    currsock.update(&pid, &sk);

    return 0;
};

static int trace_connect_return(struct pt_regs *ctx, short ipver)
{
    int ret = PT_REGS_RC(ctx);
    u32 pid = bpf_get_current_pid_tgid();

    struct sock **skpp;
    skpp = currsock.lookup(&pid);
    if (skpp == 0) {
        return 0;   // missed entry
    }

    if (ret != 0) {
        // failed to send SYNC packet, may not have populated
        // socket __sk_common.{skc_rcv_saddr, ...}
        currsock.delete(&pid);
        return 0;
    }

    // pull in details
    struct sock *skp = *skpp;
    u16 dport = skp->__sk_common.skc_dport;

    if (ipver == 4) {
        struct ipv4_data_t data4 = {.pid = pid, .ip = ipver};
        data4.saddr = skp->__sk_common.skc_rcv_saddr;
        data4.daddr = skp->__sk_common.skc_daddr;
        data4.dport = ntohs(dport);
        data4.cgroup = bpf_get_current_cgroup_id();
        bpf_get_current_comm(&data4.task, sizeof(data4.task));
        ipv4_events.perf_submit(ctx, &data4, sizeof(data4));

    } else /* 6 */ {
        struct ipv6_data_t data6 = {.pid = pid, .ip = ipver};

		// Source.
        bpf_probe_read(&data6.saddr[0], sizeof(data6.saddr[0]),
            &skp->__sk_common.skc_v6_rcv_saddr.in6_u.u6_addr32[0]);
        bpf_probe_read(&data6.saddr[1], sizeof(data6.saddr[1]),
            &skp->__sk_common.skc_v6_rcv_saddr.in6_u.u6_addr32[1]);
        bpf_probe_read(&data6.saddr[2], sizeof(data6.saddr[2]),
            &skp->__sk_common.skc_v6_rcv_saddr.in6_u.u6_addr32[2]);
        bpf_probe_read(&data6.saddr[3], sizeof(data6.saddr[3]),
            &skp->__sk_common.skc_v6_rcv_saddr.in6_u.u6_addr32[3]);

		// Destination.
        bpf_probe_read(&data6.daddr[0], sizeof(data6.daddr[0]),
            &skp->__sk_common.skc_v6_daddr.in6_u.u6_addr32[0]);
        bpf_probe_read(&data6.daddr[1], sizeof(data6.daddr[1]),
            &skp->__sk_common.skc_v6_daddr.in6_u.u6_addr32[1]);
        bpf_probe_read(&data6.daddr[2], sizeof(data6.daddr[2]),
            &skp->__sk_common.skc_v6_daddr.in6_u.u6_addr32[2]);
        bpf_probe_read(&data6.daddr[3], sizeof(data6.daddr[3]),
            &skp->__sk_common.skc_v6_daddr.in6_u.u6_addr32[3]);

        data6.dport = ntohs(dport);
        data6.cgroup = bpf_get_current_cgroup_id();
        bpf_get_current_comm(&data6.task, sizeof(data6.task));
        ipv6_events.perf_submit(ctx, &data6, sizeof(data6));
    }

    currsock.delete(&pid);

    return 0;
}

int trace_connect_v4_return(struct pt_regs *ctx)
{
    return trace_connect_return(ctx, 4);
}

int trace_connect_v6_return(struct pt_regs *ctx)
{
    return trace_connect_return(ctx, 6);
}
`
