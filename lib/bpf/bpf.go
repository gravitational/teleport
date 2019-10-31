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

import (
	"context"
	"fmt"
	"sync"

	"github.com/gravitational/teleport"
	controlgroup "github.com/gravitational/teleport/lib/cgroup"
	"github.com/gravitational/teleport/lib/events"

	"github.com/gravitational/trace"

	"github.com/iovisor/gobpf/bcc"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentBPF,
})

var _ = fmt.Printf

// SessionContext ...
// TODO(russjones): This data has to be copied over to break circular
// imports with lib/srv.
type SessionContext struct {
	Namespace string
	SessionID string
	ServerID  string
	Login     string
	User      string
	PID       int
	AuditLog  events.IAuditLog
	//Recorder  events.SessionRecorder
}

type Service struct {
	// watch is a map of cgroup IDs that the BPF service is watching and
	// emitting events for.
	watch   map[uint64]*SessionContext
	watchMu sync.Mutex

	closeContext context.Context
	closeFunc    context.CancelFunc

	cgroup *controlgroup.Service

	exec *exec
	open *open
	conn *conn
}

func New() (*Service, error) {
	// Check if the host is running kernel 4.18 or above and has bcc-tools
	// installed.
	err := isHostCompatible()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a cgroup controller to add/remote cgroups.
	cgroup, err := controlgroup.New()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	closeContext, closeFunc := context.WithCancel(context.Background())

	s := &Service{
		watch: make(map[uint64]*SessionContext),

		closeContext: closeContext,
		closeFunc:    closeFunc,

		cgroup: cgroup,
	}

	// TODO(russjones): Pass in a debug flag.
	s.exec, err = newExec(closeContext)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.open, err = newOpen(closeContext)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.conn, err = newConn(closeContext)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Start processing events from exec, open, and conn in a loop.
	go s.loop()

	return s, nil
}

// TODO(russjones): Make sure this program is actually unloaded upon exit.
func (s *Service) Close() error {
	s.exec.close()
	s.open.close()
	s.conn.close()

	s.closeFunc()

	return nil
}

func (s *Service) loop() {
	for {
		select {
		case event := <-s.exec.eventsCh():
			ctx, ok := s.watch[event.CgroupID]
			if !ok {
				continue
			}

			// Emit "session.exec" event.
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
				events.Program:    event.Program,
				events.Path:       event.Path,
				events.Argv:       event.Argv,
				events.ReturnCode: event.ReturnCode,
			}
			ctx.AuditLog.EmitAuditEvent(events.SessionExec, eventFields)
		case event := <-s.open.eventsCh():
			ctx, ok := s.watch[event.CgroupID]
			if !ok {
				continue
			}

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
				events.Program:    event.Program,
				events.Path:       event.Path,
				events.Flags:      event.Flags,
				events.ReturnCode: event.ReturnCode,
			}
			ctx.AuditLog.EmitAuditEvent(events.SessionOpen, eventFields)
		case event := <-s.conn.eventsCh():
			ctx, ok := s.watch[event.CgroupID]
			if !ok {
				continue
			}

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
				events.Program:    event.Program,
				events.SrcAddr:    event.SrcAddr,
				events.DstAddr:    event.DstAddr,
				events.DstPort:    event.DstPort,
				events.TCPVersion: event.Version,
			}
			ctx.AuditLog.EmitAuditEvent(events.SessionConnect, eventFields)
		case <-s.closeContext.Done():
			return
		}
	}
}

func (s *Service) OpenSession(ctx *SessionContext) error {
	err := s.cgroup.Create(ctx.SessionID)
	if err != nil {
		return trace.Wrap(err)
	}

	cgroupID, err := controlgroup.ID(ctx.SessionID)
	if err != nil {
		return trace.Wrap(err)
	}

	// Start watching for any events that come from this cgroup.
	s.addWatch(cgroupID, ctx)

	// Place requested PID into cgroup.
	err = s.cgroup.Place(ctx.SessionID, ctx.PID)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *Service) CloseSession(ctx *SessionContext) error {
	cgroupID, err := controlgroup.ID(ctx.SessionID)
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

// TODO(russjones): Implement.
func isHostCompatible() error {
	return nil
}

func attachProbe(module *bcc.Module, eventName string, functionName string) error {
	kprobe, err := module.LoadKprobe(functionName)
	if err != nil {
		return trace.Wrap(err)
	}

	err = module.AttachKprobe(eventName, kprobe, -1)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func attachRetProbe(module *bcc.Module, eventName string, functionName string) error {
	kretprobe, err := module.LoadKprobe(functionName)
	if err != nil {
		return trace.Wrap(err)
	}

	err = module.AttachKretprobe(eventName, kretprobe, -1)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func openPerfBuffer(module *bcc.Module, perfMaps []*bcc.PerfMap, name string) (<-chan []byte, error) {
	var err error

	eventCh := make(chan []byte, 1024000)
	table := bcc.NewTable(module.TableId(name), module)

	perfMap, err := bcc.InitPerfMap(table, eventCh)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	perfMap.Start()

	perfMaps = append(perfMaps, perfMap)

	return eventCh, nil
}

const (
	// commMax is the maximum length of a command from linux/sched.h.
	commMax = 16

	// pathMax is the maximum length of a path from linux/limits.h.
	pathMax = 255

	// argvMax is the maximum length of the args vector.
	argvMax = 128

	// bufferSize is the size of the exec, open, and conn buffers. This number is
	// arbitrary if it needs to be bigger, feel free to bump it.
	bufferSize = 1000000
)
