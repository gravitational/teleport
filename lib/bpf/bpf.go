//go:build bpf && !386

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

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math"
	"net"
	"slices"
	"strconv"
	"time"

	"github.com/cilium/ebpf/ringbuf"
	"github.com/gravitational/trace"
	"golang.org/x/sys/unix"

	ossteleport "github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apievents "github.com/gravitational/teleport/api/types/events"
	controlgroup "github.com/gravitational/teleport/lib/cgroup"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

// ArgsCacheSize is the number of args events to store before dropping args
// events.
const ArgsCacheSize = len(commandDataT{}.Argv)

type sessionHandler interface {
	startSession(auditSessionID uint32) error
	endSession(auditSessionID uint32) error
}

// Service manages BPF and control groups orchestration.
type Service struct {
	*servicecfg.BPFConfig

	// sessions is a map of audit session IDs that the BPF service is
	// watching and emitting events for.
	sessions utils.SyncMap[uint32, *SessionContext]

	// argsCache holds the arguments to execve because they come a different
	// event than the result.
	argsCache *utils.FnCache

	// closeContext is used to signal the BPF service is shutting down to all
	// goroutines.
	closeContext context.Context
	closeFunc    context.CancelFunc

	// cgroup is used to potentially unmount the cgroup filesystem after upgrades.
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

	// If BPF-based auditing is not enabled, don't configure anything return
	// right away.
	if !config.Enabled {
		logger.DebugContext(context.Background(), "Enhanced session recording is not enabled, skipping")
		return &NOP{}, nil
	}

	closeContext, closeFunc := context.WithCancel(context.Background())

	s := &Service{
		BPFConfig:    config,
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

	// Compile and start BPF programs (buffer size given).
	s.exec, err = startExec(*config.CommandBufferSize)
	if err != nil {
		return nil, trace.Wrap(err, "failed to load command hooks")
	}
	s.open, err = startOpen(*config.DiskBufferSize)
	if err != nil {
		return nil, trace.Wrap(err, "failed to load disk hooks")
	}
	s.conn, err = startConn(*config.NetworkBufferSize)
	if err != nil {
		return nil, trace.Wrap(err, "failed to load network hooks")
	}

	logger.DebugContext(closeContext, "Started enhanced session recording",
		"command_buffer_size", *s.CommandBufferSize,
		"disk_buffer_size", *s.DiskBufferSize,
		"network_buffer_size", *s.NetworkBufferSize,
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
// the user process and automatically unloaded when the process exits".
func (s *Service) Close(restarting bool) error {
	// Unload the BPF programs.
	s.exec.close()
	s.open.close()
	s.conn.close()

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

// OpenSession will begin monitoring all events from the given session
// and emitting the results to the audit log.
func (s *Service) OpenSession(ctx *SessionContext) error {
	auditSessID := ctx.AuditSessionID

	// Sanity check the audit session ID just in case. If the auid is
	// MaxUint32 that means its unset; Linux uses -1 to indicate unset
	// which underflows to MaxUint32.
	if auditSessID == math.MaxUint32 {
		return trace.NotFound("audit session ID not set")
	}
	if sctx, found := s.sessions.Load(auditSessID); found {
		logger.WarnContext(s.closeContext, "Audit session ID already in use", "session_id", sctx.SessionID, "current_session_id", ctx.SessionID, "audit_session_id", auditSessID)
		return trace.BadParameter("audit session ID already in use")
	}

	logger.DebugContext(s.closeContext, "Opening session", "session_id", ctx.SessionID, "audit_session_id", auditSessID)

	// initializedModClosures holds all already opened modules closures.
	initializedModClosures := make([]sessionHandler, 0)
	for _, m := range []struct {
		eventName string
		module    sessionHandler
	}{
		{constants.EnhancedRecordingCommand, s.exec},
		{constants.EnhancedRecordingDisk, s.open},
		{constants.EnhancedRecordingNetwork, s.conn},
	} {
		// If the event is not being monitored in this session we
		// shouldn't start monitoring it.
		if _, ok := ctx.Events[m.eventName]; !ok {
			continue
		}

		// Register audit session ID in the BPF module.
		if err := m.module.startSession(auditSessID); err != nil {
			// Clean up all already opened modules.
			for _, closer := range initializedModClosures {
				if closeErr := closer.endSession(auditSessID); closeErr != nil {
					logger.DebugContext(s.closeContext, "failed to close session", "error", closeErr)
				}
			}
			return trace.Wrap(err)
		}
		initializedModClosures = append(initializedModClosures, m.module)
	}

	// Start watching for any events that come from this audit session ID.
	s.sessions.Store(auditSessID, ctx)

	return nil
}

// CloseSession will stop monitoring events from a particular session.
func (s *Service) CloseSession(ctx *SessionContext) error {
	// Stop watching for events for this session.
	s.sessions.Delete(ctx.AuditSessionID)

	var errs []error

	for _, m := range []struct {
		eventName string
		module    sessionHandler
	}{
		{constants.EnhancedRecordingCommand, s.exec},
		{constants.EnhancedRecordingDisk, s.open},
		{constants.EnhancedRecordingNetwork, s.conn},
	} {
		// If the event is not being monitored in this session we
		// shouldn't stop monitoring it.
		if _, ok := ctx.Events[m.eventName]; !ok {
			continue
		}

		// Remove the audit session ID from BPF module.
		if err := m.module.endSession(ctx.AuditSessionID); err != nil {
			errs = append(errs, trace.Wrap(err))
		}
	}

	return trace.NewAggregate(errs...)
}

func (s *Service) Enabled() bool {
	return true
}

func sendEvents(bpfEvents chan []byte, eventBuf *ringbuf.Reader) {
	defer eventBuf.Close()

	for {
		rec, err := eventBuf.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				logger.DebugContext(context.Background(), "Received signal, exiting")
				return
			}
			logger.ErrorContext(context.Background(), "Error reading from ring buffer", "error", err)
			return
		}

		bpfEvents <- rec.RawSample[:]
	}
}

// processAccessEvents pulls events off the perf ring buffer, parses them, and emits them to
// the audit log.
// TODO(capnspacehook): combine processAccessEvents and processNetworkEvents
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
	var event commandDataT
	err := unmarshalEvent(eventBytes, &event)
	if err != nil {
		logger.DebugContext(s.closeContext, "Failed to read binary data", "error", err)
		return
	}

	// If the event comes from a unmonitored process/audit session ID,
	// don't process it.
	ctx, ok := s.sessions.Load(event.AuditSessionId)
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
		key := strconv.FormatUint(event.Pid, 10)

		args, err := utils.FnCacheGet(s.closeContext, s.argsCache, key, func(ctx context.Context) ([]string, error) {
			return make([]string, 0), nil
		})
		if err != nil {
			logger.WarnContext(s.closeContext, "Unable to retrieve args from FnCache - this is a bug!", "error", err)
			args = []string{}
		}

		args = append(args, ConvertString(event.Argv[:]))

		s.argsCache.SetWithTTL(key, args, 24*time.Hour)
	// The event has returned, emit the fully parsed event.
	case eventRet:
		// The args should have come in a previous event, find them by PID.
		key := strconv.FormatUint(event.Pid, 10)

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
				User:            ctx.User,
				Login:           ctx.Login,
				UserClusterName: ctx.UserOriginClusterName,
				UserRoles:       slices.Clone(ctx.UserRoles),
				UserTraits:      ctx.UserTraits.Clone(),
			},
			BPFMetadata: apievents.BPFMetadata{
				CgroupID:       event.Cgroup,
				AuditSessionID: event.AuditSessionId,
				Program:        ConvertString(event.Command[:]),
				PID:            event.Pid,
			},
			PPID:       event.Ppid,
			ReturnCode: event.Retval,
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
	var event diskDataT
	err := unmarshalEvent(eventBytes, &event)
	if err != nil {
		logger.DebugContext(s.closeContext, "Failed to read binary data", "error", err)
		return
	}

	// If the event comes from a unmonitored process/audit session ID,
	// don't process it.
	ctx, ok := s.sessions.Load(event.AuditSessionId)
	if !ok {
		return
	}

	// If the disk event is not being monitored, don't process it.
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
			User:            ctx.User,
			Login:           ctx.Login,
			UserClusterName: ctx.UserOriginClusterName,
			UserRoles:       slices.Clone(ctx.UserRoles),
			UserTraits:      ctx.UserTraits.Clone(),
		},
		BPFMetadata: apievents.BPFMetadata{
			CgroupID:       event.Cgroup,
			AuditSessionID: event.AuditSessionId,
			Program:        ConvertString(event.Command[:]),
			PID:            event.Pid,
		},
		Flags:      event.Flags,
		Path:       ConvertString(event.FilePath[:]),
		ReturnCode: event.ReturnCode,
	}
	// Logs can be DoS by event failures here
	_ = ctx.Emitter.EmitAuditEvent(ctx.Context, sessionDiskEvent)
}

// emit4NetworkEvent will parse and emit IPv4 events to the Audit Log.
func (s *Service) emit4NetworkEvent(eventBytes []byte) {
	// Unmarshal raw event bytes.
	var event networkIpv4DataT
	err := unmarshalEvent(eventBytes, &event)
	if err != nil {
		logger.DebugContext(s.closeContext, "Failed to read binary data", "error", err)
		return
	}

	// If the event comes from a unmonitored process/audit session ID,
	// don't process it.
	ctx, ok := s.sessions.Load(event.AuditSessionId)
	if !ok {
		return
	}

	// If the network event is not being monitored, don't process it.
	_, ok = ctx.Events[constants.EnhancedRecordingNetwork]
	if !ok {
		return
	}

	srcAddr := ipv4HostToIP(event.Saddr)
	dstAddr := ipv4HostToIP(event.Daddr)
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
			User:            ctx.User,
			Login:           ctx.Login,
			UserClusterName: ctx.UserOriginClusterName,
			UserRoles:       slices.Clone(ctx.UserRoles),
			UserTraits:      ctx.UserTraits.Clone(),
		},
		BPFMetadata: apievents.BPFMetadata{
			CgroupID:       event.Cgroup,
			AuditSessionID: event.AuditSessionId,
			Program:        ConvertString(event.Command[:]),
			PID:            uint64(event.Pid),
		},
		DstPort:    int32(event.Dport),
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
	var event networkIpv6DataT
	err := unmarshalEvent(eventBytes, &event)
	if err != nil {
		logger.DebugContext(s.closeContext, "Failed to read binary data", "error", err)
		return
	}

	// If the event comes from a unmonitored process/audit session ID,
	// don't process it.
	ctx, ok := s.sessions.Load(event.AuditSessionId)
	if !ok {
		return
	}

	// If the network event is not being monitored, don't process it.
	_, ok = ctx.Events[constants.EnhancedRecordingNetwork]
	if !ok {
		return
	}

	srcAddr := net.IP(event.Saddr.In6U.U6Addr8[:])
	dstAddr := net.IP(event.Daddr.In6U.U6Addr8[:])
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
			User:            ctx.User,
			Login:           ctx.Login,
			UserClusterName: ctx.UserOriginClusterName,
			UserRoles:       slices.Clone(ctx.UserRoles),
			UserTraits:      ctx.UserTraits.Clone(),
		},
		BPFMetadata: apievents.BPFMetadata{
			CgroupID:       event.Cgroup,
			AuditSessionID: event.AuditSessionId,
			Program:        ConvertString(event.Command[:]),
			PID:            uint64(event.Pid),
		},
		DstPort:    int32(event.Dport),
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

// unmarshalEvent will unmarshal the perf event.
func unmarshalEvent(data []byte, v interface{}) error {
	err := binary.Read(bytes.NewBuffer(data), binary.LittleEndian, v)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ConvertString converts a NUL-terminated string to a Go string.
func ConvertString(s []byte) string {
	return unix.ByteSliceToString(s)
}

// SystemHasBPF returns true if the binary was build with support for BPF
// compiled in.
func SystemHasBPF() bool {
	return true
}
