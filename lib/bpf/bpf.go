//go:build bpf && !386
// +build bpf,!386

/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package bpf

import "C"

import (
	"bytes"
	"context"
	"embed"
	"encoding/binary"
	"net"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"github.com/gravitational/trace"

	ossteleport "github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apievents "github.com/gravitational/teleport/api/types/events"
	controlgroup "github.com/gravitational/teleport/lib/cgroup"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

//go:embed bytecode
var embedFS embed.FS

// ArgsCacheSize is the number of args events to store before dropping args
// events.
const ArgsCacheSize = 1024

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

func (w *SessionWatch) Get(cgroupID uint64) (ctx *SessionContext, ok bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	ctx, ok = w.watch[cgroupID]
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
	*servicecfg.BPFConfig

	// watch is a map of cgroup IDs that the BPF service is watching and
	// emitting events for.
	watch SessionWatch

	// argsCache holds the arguments to execve because they come a different
	// event than the result.
	argsCache *utils.FnCache

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
	// conn is set only when restricted sessions are enabled.
	conn *conn
}

// New creates a BPF service.
func New(config *servicecfg.BPFConfig) (bpf BPF, err error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	closeContext, closeFunc := context.WithCancel(context.Background())

	// If BPF-based auditing is not enabled, don't configure anything return
	// right away.
	if !config.Enabled {
		logger.DebugContext(closeContext, "Enhanced session recording is not enabled, skipping")
		return &NOP{}, nil
	}

	s := &Service{
		BPFConfig:    config,
		watch:        NewSessionWatch(),
		closeContext: closeContext,
		closeFunc:    closeFunc,
	}

	// Create a cgroup controller to add/remote cgroups.
	s.cgroup, err = controlgroup.New(&controlgroup.Config{
		MountPath: config.CgroupPath,
		RootPath:  config.RootPath,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		if err != nil {
			if err := s.cgroup.Close(true); err != nil {
				logger.WarnContext(closeContext, "Failed to close cgroup", "error", err)
			}
		}
	}()

	s.argsCache, err = utils.NewFnCache(utils.FnCacheConfig{
		TTL: 24 * time.Hour,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	start := time.Now()
	logger.DebugContext(closeContext, "Starting enhanced session recording")

	// Compile and start BPF programs if they are enabled (buffer size given).
	s.exec, err = startExec(*config.CommandBufferSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.open, err = startOpen(*config.DiskBufferSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Load network BPF modules only when required.
	s.conn, err = startConn(*config.NetworkBufferSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logger.DebugContext(closeContext, "Started enhanced session recording",
		"command_buffer_size", *s.CommandBufferSize,
		"disk_buffer_size", *s.DiskBufferSize,
		"network_buffer_size", *s.NetworkBufferSize,
		"cgroup_mount_path", s.CgroupPath,
		"elapsed", time.Since(start),
	)

	go s.processNetworkEvents()

	// Start pulling events off the perf buffers and emitting them to the
	// Audit Log.
	go s.processAccessEvents()

	return s, nil
}

// Close will stop any running BPF programs. Note this is only for a graceful
// shutdown, from the man page for BPF: "Generally, eBPF programs are loaded by
// the user process and automatically unloaded when the process exits". The
// restarting parameter indicates that Teleport is shutting down because of a
// restart, and thus we should skip any deinitialization that would interfere
// with the new Teleport instance.
func (s *Service) Close(restarting bool) error {
	// Unload the BPF programs.
	s.exec.close()
	s.open.close()
	if s.conn != nil {
		s.conn.close()
	}

	// Close cgroup service. We should not unmount the cgroup filesystem if
	// we're restarting.
	skipCgroupUnmount := restarting
	if err := s.cgroup.Close(skipCgroupUnmount); err != nil {
		logger.WarnContext(s.closeContext, "Failed to close cgroup", "error", err)
	}

	// Signal to the processAccessEvents pulling events off the perf buffer to shutdown.
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

	// initializedModClosures holds all already opened modules closures.
	initializedModClosures := make([]interface{ endSession(uint64) error }, 0)
	for _, module := range []cgroupRegister{
		s.open,
		s.exec,
		s.conn,
	} {
		// Register cgroup in the BPF module.
		if err := module.startSession(cgroupID); err != nil {
			// Clean up all already opened modules.
			for _, closer := range initializedModClosures {
				if closeErr := closer.endSession(cgroupID); closeErr != nil {
					logger.DebugContext(s.closeContext, "failed to close session", "error", closeErr)
				}
			}
			return 0, trace.Wrap(err)
		}
		initializedModClosures = append(initializedModClosures, module)
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

	var errs []error
	// Move all PIDs to the root cgroup and remove the cgroup created for this
	// session.
	if err := s.cgroup.Remove(ctx.SessionID); err != nil {
		errs = append(errs, trace.Wrap(err))
	}

	for _, module := range []interface{ endSession(cgroupID uint64) error }{
		s.open,
		s.exec,
		s.conn,
	} {
		// Remove the cgroup from BPF module.
		if err := module.endSession(cgroupID); err != nil {
			errs = append(errs, trace.Wrap(err))
		}
	}

	return trace.NewAggregate(errs...)
}

func (s *Service) Enabled() bool {
	return true
}

// processAccessEvents pulls events off the perf ring buffer, parses them, and emits them to
// the audit log.
func (s *Service) processAccessEvents() {
	for {
		select {
		// Program execution.
		case event := <-s.exec.events():
			s.emitCommandEvent(event)
		// Disk access.
		case event := <-s.open.events():
			s.emitDiskEvent(event)
		case <-s.closeContext.Done():
			return
		}
	}
}

// processNetworkEvents pulls networks events of the ring buffer and emits them
// to the audit log.
func (s *Service) processNetworkEvents() {
	for {
		select {
		// Network access (IPv4).
		case event := <-s.conn.v4Events():
			s.emit4NetworkEvent(event)
		// Network access (IPv6).
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
		logger.DebugContext(s.closeContext, "Failed to read binary data", "error", err)
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
	// the args in a ttlmap, so they can be retrieved when the return event arrives.
	case eventArg:
		key := strconv.FormatUint(event.PID, 10)

		args, err := utils.FnCacheGet(s.closeContext, s.argsCache, key, func(ctx context.Context) ([]string, error) {
			return make([]string, 0), nil
		})
		if err != nil {
			logger.WarnContext(s.closeContext, "Unable to retrieve args from FnCache - this is a bug!", "error", err)
			args = []string{}
		}

		argv := (*C.char)(unsafe.Pointer(&event.Argv))
		args = append(args, C.GoString(argv))

		s.argsCache.SetWithTTL(key, args, 24*time.Hour)
	// The event has returned, emit the fully parsed event.
	case eventRet:
		// The args should have come in a previous event, find them by PID.
		key := strconv.FormatUint(event.PID, 10)

		args, err := utils.FnCacheGet(s.closeContext, s.argsCache, key, func(ctx context.Context) ([]string, error) {
			return nil, trace.NotFound("args missing")
		})

		if err != nil {
			logger.DebugContext(s.closeContext, "Got event with missing args, skipping")
			lostCommandEvents.Add(float64(1))
			return
		}

		// Emit "command" event.
		sessionCommandEvent := &apievents.SessionCommand{
			Metadata: apievents.Metadata{
				Type: events.SessionCommandEvent,
				Code: events.SessionCommandCode,
			},
			ServerMetadata: apievents.ServerMetadata{
				ServerVersion:   ossteleport.Version,
				ServerID:        ctx.ServerID,
				ServerHostname:  ctx.ServerHostname,
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
			Path:       args[0],
			Argv:       args[1:],
		}
		if err := ctx.Emitter.EmitAuditEvent(ctx.Context, sessionCommandEvent); err != nil {
			logger.WarnContext(ctx.Context, "Failed to emit command event", "error", err)
		}

		// Now that the event has been processed, remove from cache.
		s.argsCache.Remove(key)
	}
}

// emitDiskEvent will parse and emit disk events to the Audit Log.
func (s *Service) emitDiskEvent(eventBytes []byte) {
	// Unmarshal raw event bytes.
	var event rawOpenEvent
	err := unmarshalEvent(eventBytes, &event)
	if err != nil {
		logger.DebugContext(s.closeContext, "Failed to read binary data", "error", err)
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
			ServerVersion:   ossteleport.Version,
			ServerID:        ctx.ServerID,
			ServerHostname:  ctx.ServerHostname,
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
		logger.DebugContext(s.closeContext, "Failed to read binary data", "error", err)
		return
	}

	// If the event comes from an unmonitored process/cgroup, don't process it.
	ctx, ok := s.watch.Get(event.CgroupID)
	if !ok {
		return
	}

	// If the network event is not being monitored, don't process it.
	_, ok = ctx.Events[constants.EnhancedRecordingNetwork]
	if !ok {
		return
	}

	srcAddr := ipv4HostToIP(event.SrcAddr)
	dstAddr := ipv4HostToIP(event.DstAddr)
	sessionNetworkEvent := &apievents.SessionNetwork{
		Metadata: apievents.Metadata{
			Type: events.SessionNetworkEvent,
			Code: events.SessionNetworkCode,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion:   ossteleport.Version,
			ServerID:        ctx.ServerID,
			ServerHostname:  ctx.ServerHostname,
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
		logger.WarnContext(ctx.Context, "Failed to emit network event", "error", err)
	}
}

// emit6NetworkEvent will parse and emit IPv6 events to the Audit Log.
func (s *Service) emit6NetworkEvent(eventBytes []byte) {
	// Unmarshal raw event bytes.
	var event rawConn6Event
	err := unmarshalEvent(eventBytes, &event)
	if err != nil {
		logger.DebugContext(s.closeContext, "Failed to read binary data", "error", err)
		return
	}

	// If the event comes from an unmonitored process/cgroup, don't process it.
	ctx, ok := s.watch.Get(event.CgroupID)
	if !ok {
		return
	}

	// If the network event is not being monitored, don't process it.
	_, ok = ctx.Events[constants.EnhancedRecordingNetwork]
	if !ok {
		return
	}

	srcAddr := ipv6HostToIP(event.SrcAddr)
	dstAddr := ipv6HostToIP(event.DstAddr)
	sessionNetworkEvent := &apievents.SessionNetwork{
		Metadata: apievents.Metadata{
			Type: events.SessionNetworkEvent,
			Code: events.SessionNetworkCode,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion:   ossteleport.Version,
			ServerID:        ctx.ServerID,
			ServerHostname:  ctx.ServerHostname,
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
		logger.WarnContext(ctx.Context, "Failed to emit network event", "error", err)
	}
}

func ipv4HostToIP(addr uint32) net.IP {
	val := make([]byte, 4)
	binary.LittleEndian.PutUint32(val, addr)
	return val
}

func ipv6HostToIP(addr [4]uint32) net.IP {
	val := make([]byte, 16)
	binary.LittleEndian.PutUint32(val[0:], addr[0])
	binary.LittleEndian.PutUint32(val[4:], addr[1])
	binary.LittleEndian.PutUint32(val[8:], addr[2])
	binary.LittleEndian.PutUint32(val[12:], addr[3])
	return val
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
