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
	"sync"
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

const (
	// CommandArgsBufferSize is the size of a command event args buffer.
	CommandArgsBufferSize = len(commandDataT{}.Args)
	// TruncatedArg is the string used to indicate that the arguments
	// were truncated.
	TruncatedArg = "[truncated]"

	// eventSendTimeout is the maximum time to wait for an event to be sent
	// to be emitted to the Audit log.
	eventSendTimeout = 10 * time.Second
)

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

	lostEvents EventCount

	wg sync.WaitGroup
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

	// Start pulling events off the perf buffers and emitting them to the
	// Audit Log.
	s.wg.Go(func() {
		for event := range s.exec.events() {
			s.emitCommandEvent(event)
		}
	})
	s.wg.Go(func() {
		for event := range s.open.events() {
			s.emitDiskEvent(event)
		}
	})
	s.wg.Go(func() {
		for event := range s.conn.v4Events() {
			s.emit4NetworkEvent(event)
		}
	})
	s.wg.Go(func() {
		for event := range s.conn.v6Events() {
			s.emit6NetworkEvent(event)
		}
	})

	// Log the number of lost events.
	s.wg.Go(s.logLostEvents)

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

	s.closeFunc()

	s.wg.Wait()

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

// LostEvents returns the total number of lost events for command, disk,
// and network events since the service was started.
func (s *Service) LostEvents() EventCount {
	return EventCount{
		commandEvents: s.exec.lostCounter.Count(),
		diskEvents:    s.open.lostCounter.Count(),
		networkEvents: s.conn.lostCounter.Count(),
	}
}

func sendEvents(eventType string, bpfEvents chan []byte, eventBuf *ringbuf.Reader) {
	defer eventBuf.Close()
	defer close(bpfEvents)

	timer := time.NewTimer(eventSendTimeout)
	defer timer.Stop()

	for {
		rec, err := eventBuf.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) || errors.Is(err, ringbuf.ErrFlushed) {
				logger.DebugContext(context.Background(), "Received signal, exiting")
				return
			}
			logger.ErrorContext(context.Background(), "Error reading from ring buffer", "error", err)
			return
		}

		// Avoid blocking on the channel if the buffer is full, this
		// could prevent the service from shutting down.
		timer.Reset(eventSendTimeout)
		select {
		case bpfEvents <- rec.RawSample[:]:
		case <-timer.C:
			logger.WarnContext(context.Background(), "Enhanced session recording event buffer is full, dropping event", "event_type", eventType)
		}
	}
}

func (s *Service) logLostEvents() {
	const interval = 5 * time.Second
	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			le := s.LostEvents()
			newlyLost := le.Delta(s.lostEvents)
			if !newlyLost.Empty() {
				logger.WarnContext(s.closeContext, "Lost some Enhanced Session Recording events in the last 5 seconds due to a full eBPF ringbuffer, consider increasing the buffer sizes; see https://goteleport.com/docs/enroll-resources/server-access/guides/bpf-session-recording/#create-a-configuration-file for more information",
					"command_events",
					newlyLost.commandEvents,
					"disk_events",
					newlyLost.diskEvents,
					"network_events",
					newlyLost.networkEvents,
				)
			}

			s.lostEvents = le
			timer.Reset(interval)
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

	argLen := event.ArgsLen
	if event.ArgsLen > uint32(len(event.Args)) {
		logger.WarnContext(s.closeContext, "Command event argument length is larger than the buffer size, truncating", "args_len", event.ArgsLen)
		argLen = uint32(len(event.Args))
	}
	args := convertArgs(event.Args[:argLen], event.ArgsTruncated)

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
			Program:        unix.ByteSliceToString(event.Command[:]),
			PID:            event.Pid,
		},
		PPID:       event.Ppid,
		Path:       unix.ByteSliceToString(event.Filename[:]),
		Argv:       args,
		ReturnCode: event.ReturnCode,
	}
	if err := ctx.Emitter.EmitAuditEvent(ctx.Context, sessionCommandEvent); err != nil {
		logger.WarnContext(ctx.Context, "Failed to emit command event", "error", err)
	}
}

// convertArgs converts a large buffer of null-terminated strings from
// command event arguments into a slice of strings.
func convertArgs(rawArgs []byte, truncated bool) []string {
	if len(rawArgs) == 0 {
		return nil
	}

	argc := bytes.Count(rawArgs, []byte{0x0})
	args := make([]string, 0, argc)

	parts := bytes.Split(rawArgs, []byte{0x0})
	for i, part := range parts {
		// Don't treat the final null byte as an empty argument
		if i == len(parts)-1 && len(part) == 0 {
			break
		}

		args = append(args, string(part))
	}

	if truncated {
		args = append(args, TruncatedArg)
	}

	return args
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
			Program:        unix.ByteSliceToString(event.Command[:]),
			PID:            event.Pid,
		},
		Flags:      event.Flags,
		Path:       unix.ByteSliceToString(event.FilePath[:]),
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
			Program:        unix.ByteSliceToString(event.Command[:]),
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
			Program:        unix.ByteSliceToString(event.Command[:]),
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

// SystemHasBPF returns true if the binary was build with support for BPF
// compiled in.
func SystemHasBPF() bool {
	return true
}
