// +build bpf,!386

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

	"github.com/gravitational/teleport/api/constants"
	apievents "github.com/gravitational/teleport/api/types/events"
	controlgroup "github.com/gravitational/teleport/lib/cgroup"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"

	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
)

// SessionWatch is a map of cgroup IDs that the BPF service is watching and
// emitting events for.
type SessionWatch struct {
	watch map[uint64]*SessionContext
	mu    sync.Mutex
}

func NewSessionWatch() SessionWatch {
	return SessionWatch{
		watch: make(map[uint64]*SessionContext),
	}
}

func (w *SessionWatch) Get(cgoupID uint64) (ctx *SessionContext, ok bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	ctx, ok = w.watch[cgoupID]
	return
}

func (w *SessionWatch) Add(cgroupID uint64, ctx *SessionContext) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.watch[cgroupID] = ctx
}

func (w *SessionWatch) Remove(cgroupID uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()

	delete(w.watch, cgroupID)
}


// Service manages BPF and control groups orchestration.
type Service struct {
	*Config

	// watch is a map of cgroup IDs that the BPF service is watching and
	// emitting events for.
	watch SessionWatch

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
		log.Debugf("Enhanced session recording is not enabled, skipping.")
		return &NOP{}, nil
	}

	// Check if the host can run BPF programs.
	err = IsHostCompatible()
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

		watch: NewSessionWatch(),

		closeContext: closeContext,
		closeFunc:    closeFunc,

		cgroup: cgroup,
	}

	// Create args cache used by the exec BPF program.
	s.argsCache, err = ttlmap.New(defaults.ArgsCacheSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	start := time.Now()
	log.Debugf("Starting enhanced session recording.")

	// Compile and start BPF programs if they are enabled (buffer size given).
	s.exec, err = startExec(*config.CommandBufferSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.open, err = startOpen(*config.DiskBufferSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.conn, err = startConn(*config.NetworkBufferSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Debugf("Started enhanced session recording with buffer sizes (command=%v, "+
		"disk=%v, network=%v) and cgroup mount path: %v. Took %v.",
		*s.CommandBufferSize, *s.DiskBufferSize, *s.NetworkBufferSize, s.CgroupPath,
		time.Since(start))

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

	// Close cgroup service.
	s.cgroup.Close()

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
	s.watch.Add(cgroupID, ctx)

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
	s.watch.Remove(cgroupID)

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
	ctx, ok := s.watch.Get(event.CgroupID)
	if !ok {
		return
	}

	// If the command event is not being monitored, don't process it.
	_, ok = ctx.Events[constants.EnhancedRecordingCommand]
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

		// Emit "command" event.
		sessionCommandEvent := &apievents.SessionCommand{
			Metadata: apievents.Metadata{
				Type: events.SessionCommandEvent,
				Code: events.SessionCommandCode,
			},
			ServerMetadata: apievents.ServerMetadata{
				ServerID:        ctx.ServerID,
				ServerNamespace: ctx.Namespace,
			},
			SessionMetadata: apievents.SessionMetadata{
				SessionID: ctx.SessionID,
			},
			UserMetadata: apievents.UserMetadata{
				User:  ctx.User,
				Login: ctx.Login,
			},
			BPFMetadata: apievents.BPFMetadata{
				CgroupID: event.CgroupID,
				Program:  ConvertString(unsafe.Pointer(&event.Command)),
				PID:      event.PID,
			},
			PPID:       event.PPID,
			ReturnCode: event.ReturnCode,
			Path:       argv[0],
			Argv:       argv[1:],
		}
		if err := ctx.Emitter.EmitAuditEvent(ctx.Context, sessionCommandEvent); err != nil {
			log.WithError(err).Warn("Failed to emit command event.")
		}

		// Now that the event has been processed, remove from cache.
		s.argsCache.Remove(strconv.FormatUint(event.PID, 10))
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
	ctx, ok := s.watch.Get(event.CgroupID)
	if !ok {
		return
	}

	// If the network event is not being monitored, don't process it.
	_, ok = ctx.Events[constants.EnhancedRecordingDisk]
	if !ok {
		return
	}

	sessionDiskEvent := &apievents.SessionDisk{
		Metadata: apievents.Metadata{
			Type: events.SessionDiskEvent,
			Code: events.SessionDiskCode,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerID:        ctx.ServerID,
			ServerNamespace: ctx.Namespace,
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: ctx.SessionID,
		},
		UserMetadata: apievents.UserMetadata{
			User:  ctx.User,
			Login: ctx.Login,
		},
		BPFMetadata: apievents.BPFMetadata{
			CgroupID: event.CgroupID,
			Program:  ConvertString(unsafe.Pointer(&event.Command)),
			PID:      event.PID,
		},
		Flags:      event.Flags,
		Path:       ConvertString(unsafe.Pointer(&event.Path)),
		ReturnCode: event.ReturnCode,
	}
	// Logs can be DoS by event failures here
	_ = ctx.Emitter.EmitAuditEvent(ctx.Context, sessionDiskEvent)
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
	ctx, ok := s.watch.Get(event.CgroupID)
	if !ok {
		return
	}

	// If the network event is not being monitored, don't process it.
	_, ok = ctx.Events[constants.EnhancedRecordingNetwork]
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

	sessionNetworkEvent := &apievents.SessionNetwork{
		Metadata: apievents.Metadata{
			Type: events.SessionNetworkEvent,
			Code: events.SessionNetworkCode,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerID:        ctx.ServerID,
			ServerNamespace: ctx.Namespace,
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: ctx.SessionID,
		},
		UserMetadata: apievents.UserMetadata{
			User:  ctx.User,
			Login: ctx.Login,
		},
		BPFMetadata: apievents.BPFMetadata{
			CgroupID: event.CgroupID,
			Program:  ConvertString(unsafe.Pointer(&event.Command)),
			PID:      uint64(event.PID),
		},
		DstPort:    int32(event.DstPort),
		DstAddr:    dstAddr.String(),
		SrcAddr:    srcAddr.String(),
		TCPVersion: 4,
	}
	if err := ctx.Emitter.EmitAuditEvent(ctx.Context, sessionNetworkEvent); err != nil {
		log.WithError(err).Warn("Failed to emit network event.")
	}
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
	ctx, ok := s.watch.Get(event.CgroupID)
	if !ok {
		return
	}

	// If the network event is not being monitored, don't process it.
	_, ok = ctx.Events[constants.EnhancedRecordingNetwork]
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

	sessionNetworkEvent := &apievents.SessionNetwork{
		Metadata: apievents.Metadata{
			Type: events.SessionNetworkEvent,
			Code: events.SessionNetworkCode,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerID:        ctx.ServerID,
			ServerNamespace: ctx.Namespace,
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: ctx.SessionID,
		},
		UserMetadata: apievents.UserMetadata{
			User:  ctx.User,
			Login: ctx.Login,
		},
		BPFMetadata: apievents.BPFMetadata{
			CgroupID: event.CgroupID,
			Program:  ConvertString(unsafe.Pointer(&event.Command)),
			PID:      uint64(event.PID),
		},
		DstPort:    int32(event.DstPort),
		DstAddr:    dstAddr.String(),
		SrcAddr:    srcAddr.String(),
		TCPVersion: 6,
	}
	if err := ctx.Emitter.EmitAuditEvent(ctx.Context, sessionNetworkEvent); err != nil {
		log.WithError(err).Warn("Failed to emit network event.")
	}
}

// unmarshalEvent will unmarshal the perf event.
func unmarshalEvent(data []byte, v interface{}) error {
	err := binary.Read(bytes.NewBuffer(data), binary.LittleEndian, v)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ConvertString converts a C string to a Go string.
func ConvertString(s unsafe.Pointer) string {
	return C.GoString((*C.char)(s))
}

// SystemHasBPF returns true if the binary was build with support for BPF
// compiled in.
func SystemHasBPF() bool {
	return true
}
