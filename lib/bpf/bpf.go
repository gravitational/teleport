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
	"sync"

	"github.com/gravitational/teleport/lib/cgroup"
	"github.com/gravitational/teleport/lib/events"

	"github.com/gravitational/trace"
)

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
	Recorder  events.ForwardRecorder
}

type Config struct {
	Cgroup *cgroup.Service
}

func (c *Config) CheckAndSetDefaults() error {
	if c.Cgroup == nil {
		return trace.BadParameter("cgroup service required")
	}

	// Check if the host is running kernel 4.18 or above and has bcc-tools
	// installed.
	err := isHostCompatible()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

type Service struct {
	*Config

	// watch is a map of cgroup IDs that the BPF service is watching and
	// emitting events for.
	watch   map[string]srv.Context
	watchMu sync.Mutex

	exec *exec
	//open *open
	//conn *conn
}

func New(config *Config) (*Service, error) {
	err := config.CheckAndSetDefaults()
	return &Service{
		Config: config,

		exec: newExec(closeContext),
		//open: newOpen(closeContext),
		//conn: newConn(closeContext),
	}
}

func (s *Service) Start() error {
	err := s.exec.Start(watch)
	if err != nil {
		return trace.Wrap(err)
	}

	//err = s.open.Start(watch)
	//if err != nil {
	//	return trace.Wrap(err)
	//}

	//err = s.conn.Start(watch)
	//if err != nil {
	//	return trace.Wrap(err)
	//}

	return nil
}

// TODO(russjones): Make sure this program is actually unloaded upon exit.
func (s *Service) Close() {
	s.exec.Close()
	//s.open.Close()
	//s.conn.Close()
}

func (s *Service) loop() {
	for {
		select {
		case event := <-s.exec.Events():
			ctx, ok := s.watch[event.CgroupID]
			if !ok {
				continue
			}

			// Emit "session.exec" event.
			eventFields := events.EventFields{
				// Common fields.
				events.EventNamespace:  ctx.Server().GetNamespace(),
				events.SessionEventID:  ctx.Session().ID(),
				events.SessionServerID: ctx.Server().HostUUID(),
				events.EventLogin:      ctx.Identity.Login,
				events.EventUser:       ctx.Identity.TeleportUser,
				// Exec fields.
				events.PID:        event.PPID,
				events.PPID:       event.PID,
				events.CgroupID:   event.CgroupID,
				events.Program:    event.Program,
				events.Path:       event.Path,
				events.Argv:       event.Argv,
				events.ReturnCode: event.ReturnCode,
			}
			session.Recorder().GetAuditLog().EmitAuditEvent(events.SessionExec, eventFields)
		case <-s.closeContext.Done():
			return
		}
	}
}

func (s *Service) OpenSession(ctx *ServerContext) error {
	sess := ctx.Sesssion()

	err := s.Cgroup.Create(sess.ID())
	if err != nil {
		return trace.Wrap(err)
	}

	// Start watching for any events that come from this cgroup.
	s.addWatch(cgroup.ID(sess.ID()), ctx)

	// Place requested PID into cgroup.
	err := s.Cgroup.Place(sess.ID(), sess.PID())
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *Service) CloseSession(ctx *ServerContext) error {
	sess := ctx.Sesssion()

	// Stop watching for events from this PID.
	s.removeWatch(cgroup.ID(sess.ID()))

	// Move any existing PIDs into root cgroup.
	err := s.Cgroup.Unplace(sess.PID())
	if err != nil {
		return trace.Wrap(err)
	}

	err := s.Cgroup.Remove(sess.ID())
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *Service) addWatch(cgroupID uint64, recorder events.SessionRecorder) {
	s.watchMu.Lock()
	defer s.watchMu.Unlock()

	s.watch[strconv.FormatUint(cgroupID, 10)] = recorder
}

func (s *Service) removeWatch(cgroupID uint64) {
	s.watchMu.Lock()
	defer s.watchMu.Unlock()

	delete(s.watchMu, strconv.FormatUint(cgroupID, 10))
}

// TODO(russjones): Implement.
func isHostCompatible() error {
	return nil
}
