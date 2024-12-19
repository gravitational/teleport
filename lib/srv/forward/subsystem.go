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

package forward

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv"
)

// remoteSubsystem is a subsystem that executes on a remote node.
type remoteSubsystem struct {
	logger *slog.Logger

	serverContext *srv.ServerContext
	subsystemName string

	ctx     context.Context
	errorCh chan error
}

// parseRemoteSubsystem returns *remoteSubsystem which can be used to run a subsystem on a remote node.
func parseRemoteSubsystem(ctx context.Context, subsystemName string, serverContext *srv.ServerContext) *remoteSubsystem {
	return &remoteSubsystem{
		logger: slog.With(
			teleport.ComponentKey, teleport.ComponentRemoteSubsystem,
			"name", subsystemName,
		),
		serverContext: serverContext,
		subsystemName: subsystemName,
		ctx:           ctx,
		errorCh:       make(chan error, 3),
	}
}

// Start will begin execution of the remote subsystem on the passed in channel.
func (r *remoteSubsystem) Start(ctx context.Context, channel ssh.Channel) error {
	session := r.serverContext.RemoteSession

	stdout, err := session.StdoutPipe()
	if err != nil {
		return trace.Wrap(err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return trace.Wrap(err)
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		return trace.Wrap(err)
	}

	// request the subsystem from the remote node. if successful, the user can
	// interact with the remote subsystem with stdin, stdout, and stderr.
	err = session.RequestSubsystem(ctx, r.subsystemName)
	if err != nil {
		// emit an event to the audit log with the reason remote execution failed
		r.emitAuditEvent(ctx, err)

		return trace.Wrap(err)
	}

	// copy back and forth between stdin, stdout, and stderr and the SSH channel.
	go func() {
		defer session.Close()

		_, err := io.Copy(channel, stdout)
		r.errorCh <- err
	}()
	go func() {
		defer session.Close()

		_, err := io.Copy(channel.Stderr(), stderr)
		r.errorCh <- err
	}()
	go func() {
		defer session.Close()

		_, err := io.Copy(stdin, channel)
		r.errorCh <- err
	}()

	return nil
}

// Wait until the remote subsystem has finished execution and then return the last error.
func (r *remoteSubsystem) Wait() error {
	var lastErr error

	for i := 0; i < 3; i++ {
		select {
		case err := <-r.errorCh:
			if err != nil && !errors.Is(err, io.EOF) {
				r.logger.WarnContext(r.ctx, "Connection problem", "error", err)
				lastErr = err
			}
		case <-r.ctx.Done():
			lastErr = trace.ConnectionProblem(nil, "context is closing")
		}
	}

	// emit an event to the audit log with the result of execution
	r.emitAuditEvent(r.ctx, lastErr)

	return lastErr
}

func (r *remoteSubsystem) emitAuditEvent(ctx context.Context, err error) {
	subsystemEvent := &apievents.Subsystem{
		Metadata: apievents.Metadata{
			Type: events.SubsystemEvent,
		},
		UserMetadata: r.serverContext.Identity.GetUserMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			LocalAddr:  r.serverContext.RemoteClient.LocalAddr().String(),
			RemoteAddr: r.serverContext.RemoteClient.RemoteAddr().String(),
		},
		Name:           r.subsystemName,
		ServerMetadata: r.serverContext.GetServer().TargetMetadata(),
	}

	if err != nil {
		subsystemEvent.Code = events.SubsystemFailureCode
		subsystemEvent.Error = err.Error()
	} else {
		subsystemEvent.Code = events.SubsystemCode
	}

	if err := r.serverContext.GetServer().EmitAuditEvent(ctx, subsystemEvent); err != nil {
		r.logger.WarnContext(ctx, "Failed to emit subsystem audit event", "error", err)
	}
}
