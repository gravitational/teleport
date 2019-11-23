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

// #cgo LDFLAGS: -ldl
// #include <dlfcn.h>
// #include <stdlib.h>
import "C"

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"github.com/gravitational/teleport"
	controlgroup "github.com/gravitational/teleport/lib/cgroup"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"

	"github.com/coreos/go-semver/semver"
	"github.com/iovisor/gobpf/bcc"
)

// Service manages BPF and control groups orchestration.
type Service struct {
	*Config

	// watch is a map of cgroup IDs that the BPF service is watching and
	// emitting events for.
	watch   map[uint64]*SessionContext
	watchMu sync.Mutex

	// argsCache holds the arguments to execve because they come a different
	// event than the result.
	argsCache *ttlmap.TTLMap

	// closeContext is used to signal the BPF service is shutting down to all
	// goroutines.
	closeContext context.Context
	closeFunc    context.CancelFunc

	// cgroup is used to manage control groups.
	cgroup *controlgroup.Service

	// exec holds a BPF program that hooks execve.
	exec *exec

	// open holds a BPF program that hooks openat.
	open *open

	// conn is a BPF programs that hooks connect.
	conn *conn
}

// New creates a BPF service.
func New(config *Config) (BPF, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If BPF-based auditing is not enabled, don't configure anything return
	// right away.
	if !config.Enabled {
		return &NOP{}, nil
	}

	// Check if the host can run BPF programs.
	err = isHostCompatible()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a cgroup controller to add/remote cgroups.
	cgroup, err := controlgroup.New(&controlgroup.Config{
		MountPath: config.CgroupPath,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	closeContext, closeFunc := context.WithCancel(context.Background())

	s := &Service{
		Config: config,

		watch: make(map[uint64]*SessionContext),

		closeContext: closeContext,
		closeFunc:    closeFunc,

		cgroup: cgroup,
	}

	// Create args cache used by the exec BPF program.
	s.argsCache, err = ttlmap.New(defaults.ArgsCacheSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Compile and start BPF programs.
	s.exec, err = startExec(closeContext, config.CommandBufferSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.open, err = startOpen(closeContext, config.DiskBufferSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.conn, err = startConn(closeContext, config.NetworkBufferSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Debugf("Started enhanced auditing with buffer sizes %v %v %v and cgroup mount path: %v.",
		s.CommandBufferSize, s.DiskBufferSize, s.NetworkBufferSize, s.CgroupPath)

	// Start pulling events off the perf buffers and emitting them to the
	// Audit Log.
	go s.loop()

	return s, nil
}

// Close will stop any running BPF programs. Note this is only for a graceful
// shutdown, from the man page for BPF: "Generally, eBPF programs are loaded
// by the user process and automatically unloaded when the process exits."
func (s *Service) Close() error {
	// Unload the BPF programs.
	s.exec.close()
	s.open.close()
	s.conn.close()

	// Signal to the loop pulling events off the perf buffer to shutdown.
	s.closeFunc()

	return nil
}

// OpenSession will place a process within a cgroup and being monitoring all
// events from that cgroup and emitting the results to the audit log.
func (s *Service) OpenSession(ctx *SessionContext) (uint64, error) {
	err := s.cgroup.Create(ctx.SessionID)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	cgroupID, err := s.cgroup.ID(ctx.SessionID)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	// Start watching for any events that come from this cgroup.
	s.addWatch(cgroupID, ctx)

	// Place requested PID into cgroup.
	err = s.cgroup.Place(ctx.SessionID, ctx.PID)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return cgroupID, nil
}

// CloseSession will stop monitoring events from a particular cgroup and
// remove the cgroup.
func (s *Service) CloseSession(ctx *SessionContext) error {
	cgroupID, err := s.cgroup.ID(ctx.SessionID)
	if err != nil {
		return trace.Wrap(err)
	}

	// Stop watching for events from this PID.
	s.removeWatch(cgroupID)

	// Move all PIDs to the root cgroup and remove the cgroup created for this
	// session.
	err = s.cgroup.Remove(ctx.SessionID)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// loop pulls events off the perf ring buffer, parses them, and emits them to
// the audit log.
func (s *Service) loop() {
	for {
		select {
		// Program execution.
		case event := <-s.exec.events():
			s.emitCommandEvent(event)
		// Disk access.
		case event := <-s.open.events():
			s.emitDiskEvent(event)
		// Network access (IPv4).
		case event := <-s.conn.v4Events():
			s.emit4NetworkEvent(event)
		// Network access (IPv4).
		case event := <-s.conn.v6Events():
			s.emit6NetworkEvent(event)
		case <-s.closeContext.Done():
			return
		}
	}
}

// emitCommandEvent will parse and emit command events to the Audit Log.
func (s *Service) emitCommandEvent(eventBytes []byte) {
	// Unmarshal raw event bytes.
	var event rawExecEvent
	err := unmarshalEvent(eventBytes, &event)
	if err != nil {
		log.Debugf("Failed to read binary data: %v.", err)
		return
	}

	// If the event comes from a unmonitored process/cgroup, don't process it.
	ctx, ok := s.watch[event.CgroupID]
	if !ok {
		return
	}

	// If the command event is not being monitored, don't process it.
	_, ok = ctx.Events[teleport.EnhancedRecordingCommand]
	if !ok {
		return
	}

	switch event.Type {
	// Args are sent in their own event by execsnoop to save stack space. Store
	// the args in a ttlmap so they can be retrieved when the return event arrives.
	case eventArg:
		var buf []string
		buffer, ok := s.argsCache.Get(strconv.FormatUint(event.PID, 10))
		if !ok {
			buf = make([]string, 0)
		} else {
			buf = buffer.([]string)
		}

		argv := (*C.char)(unsafe.Pointer(&event.Argv))
		buf = append(buf, C.GoString(argv))
		s.argsCache.Set(strconv.FormatUint(event.PID, 10), buf, 24*time.Hour)
	// The event has returned, emit the fully parsed event.
	case eventRet:
		// The args should have come in a previous event, find them by PID.
		args, ok := s.argsCache.Get(strconv.FormatUint(event.PID, 10))
		if !ok {
			log.Debugf("Got event with missing args: skipping.")
			lostCommandEvents.Add(float64(1))
			return
		}
		argv := args.([]string)

		// Convert C string that holds the command name into a Go string.
		command := C.GoString((*C.char)(unsafe.Pointer(&event.Command)))

		// Emit "command" event.
		eventFields := events.EventFields{
			// Common fields.
			events.EventNamespace:  ctx.Namespace,
			events.SessionEventID:  ctx.SessionID,
			events.SessionServerID: ctx.ServerID,
			events.EventLogin:      ctx.Login,
			events.EventUser:       ctx.User,
			// Exec fields.
			events.PID:        event.PPID,
			events.PPID:       event.PID,
			events.CgroupID:   event.CgroupID,
			events.Program:    command,
			events.Path:       argv[0],
			events.Argv:       argv[1:],
			events.ReturnCode: event.ReturnCode,
		}
		ctx.AuditLog.EmitAuditEvent(events.SessionCommand, eventFields)

		//// Remove, only for debugging.
		//fmt.Printf("--> Event=exec CgroupID=%v PID=%v PPID=%v Program=%v Path=%v Args=%v ReturnCode=%v.\n",
		//	event.CgroupID, event.PID, event.PPID, command, argv[0], argv[1:], event.ReturnCode)
	}
}

// emitDiskEvent will parse and emit disk events to the Audit Log.
func (s *Service) emitDiskEvent(eventBytes []byte) {
	// Unmarshal raw event bytes.
	var event rawOpenEvent
	err := unmarshalEvent(eventBytes, &event)
	if err != nil {
		log.Debugf("Failed to read binary data: %v.", err)
		return
	}

	// If the event comes from a unmonitored process/cgroup, don't process it.
	ctx, ok := s.watch[event.CgroupID]
	if !ok {
		return
	}

	// If the network event is not being monitored, don't process it.
	_, ok = ctx.Events[teleport.EnhancedRecordingDisk]
	if !ok {
		return
	}

	// Convert C string that holds the command name into a Go string.
	command := C.GoString((*C.char)(unsafe.Pointer(&event.Command)))

	// Convert C string that holds the path into a Go string.
	path := C.GoString((*C.char)(unsafe.Pointer(&event.Path)))

	eventFields := events.EventFields{
		// Common fields.
		events.EventNamespace:  ctx.Namespace,
		events.SessionEventID:  ctx.SessionID,
		events.SessionServerID: ctx.ServerID,
		events.EventLogin:      ctx.Login,
		events.EventUser:       ctx.User,
		// Open fields.
		events.PID:        event.PID,
		events.CgroupID:   event.CgroupID,
		events.Program:    command,
		events.Path:       path,
		events.Flags:      event.Flags,
		events.ReturnCode: event.ReturnCode,
	}
	ctx.AuditLog.EmitAuditEvent(events.SessionDisk, eventFields)

	//// Remove, only for debugging.
	//fmt.Printf("Event=open CgroupID=%v PID=%v Command=%v ReturnCode=%v Flags=%#o Path=%v.\n",
	//	event.CgroupID, event.PID, command, event.ReturnCode, event.Flags, path)
}

// emit4NetworkEvent will parse and emit IPv4 events to the Audit Log.
func (s *Service) emit4NetworkEvent(eventBytes []byte) {
	// Unmarshal raw event bytes.
	var event rawConn4Event
	err := unmarshalEvent(eventBytes, &event)
	if err != nil {
		log.Debugf("Failed to read binary data: %v.", err)
		return
	}

	// If the event comes from a unmonitored process/cgroup, don't process it.
	ctx, ok := s.watch[event.CgroupID]
	if !ok {
		return
	}

	// If the network event is not being monitored, don't process it.
	_, ok = ctx.Events[teleport.EnhancedRecordingNetwork]
	if !ok {
		return
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

	eventFields := events.EventFields{
		// Common fields.
		events.EventNamespace:  ctx.Namespace,
		events.SessionEventID:  ctx.SessionID,
		events.SessionServerID: ctx.ServerID,
		events.EventLogin:      ctx.Login,
		events.EventUser:       ctx.User,
		// Connect fields.
		events.PID:        event.PID,
		events.CgroupID:   event.CgroupID,
		events.Program:    command,
		events.SrcAddr:    srcAddr,
		events.DstAddr:    dstAddr,
		events.DstPort:    event.DstPort,
		events.TCPVersion: 4,
	}
	ctx.AuditLog.EmitAuditEvent(events.SessionNetwork, eventFields)
}

// emit6NetworkEvent will parse and emit IPv6 events to the Audit Log.
func (s *Service) emit6NetworkEvent(eventBytes []byte) {
	// Unmarshal raw event bytes.
	var event rawConn6Event
	err := unmarshalEvent(eventBytes, &event)
	if err != nil {
		log.Debugf("Failed to read binary data: %v.", err)
		return
	}

	// If the event comes from a unmonitored process/cgroup, don't process it.
	ctx, ok := s.watch[event.CgroupID]
	if !ok {
		return
	}

	// If the network event is not being monitored, don't process it.
	_, ok = ctx.Events[teleport.EnhancedRecordingNetwork]
	if !ok {
		return
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

	eventFields := events.EventFields{
		// Common fields.
		events.EventNamespace:  ctx.Namespace,
		events.SessionEventID:  ctx.SessionID,
		events.SessionServerID: ctx.ServerID,
		events.EventLogin:      ctx.Login,
		events.EventUser:       ctx.User,
		// Connect fields.
		events.PID:        event.PID,
		events.CgroupID:   event.CgroupID,
		events.Program:    command,
		events.SrcAddr:    srcAddr,
		events.DstAddr:    dstAddr,
		events.DstPort:    event.DstPort,
		events.TCPVersion: 6,
	}
	ctx.AuditLog.EmitAuditEvent(events.SessionNetwork, eventFields)

	//// Remove, only for debugging.
	//fmt.Printf("--> Event=conn6 CgroupID=%v PID=%v Command=%v Src=%v Dst=%v:%v.\n",
	//	event.CgroupID, event.PID, command, srcAddr, dstAddr, event.DstPort)
}

func (s *Service) addWatch(cgroupID uint64, ctx *SessionContext) {
	s.watchMu.Lock()
	defer s.watchMu.Unlock()

	s.watch[cgroupID] = ctx
}

func (s *Service) removeWatch(cgroupID uint64) {
	s.watchMu.Lock()
	defer s.watchMu.Unlock()

	delete(s.watch, cgroupID)
}

func unmarshalEvent(data []byte, v interface{}) error {
	err := binary.Read(bytes.NewBuffer(data), bcc.GetHostByteOrder(), v)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func convertString(s unsafe.Pointer) string {
	return C.GoString((*C.char)(s))
}

// isHostCompatible checks that BPF programs can run on this host.
func isHostCompatible() error {
	// To find the cgroup ID of a program, bpf_get_current_cgroup_id is needed
	// which was introduced in 4.18.
	// https://github.com/torvalds/linux/commit/bf6fa2c893c5237b48569a13fa3c673041430b6c
	minKernel := semver.New("4.18.0")
	version, err := utils.KernelVersion()
	if err != nil {
		return trace.Wrap(err)
	}
	if version.LessThan(*minKernel) {
		return trace.BadParameter("incompatible kernel found, minimum supported kernel is %v", minKernel)
	}

	// Check that libbcc is on the system.
	libraryName := C.CString("libbcc.so.0")
	defer C.free(unsafe.Pointer(libraryName))
	handle := C.dlopen(libraryName, C.RTLD_NOW)
	if handle == nil {
		return trace.BadParameter("libbcc.so not found")
	}

	return nil
}
