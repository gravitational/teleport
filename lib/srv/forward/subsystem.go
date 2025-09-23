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
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv"
)

// RemoteSubsystem is a handle for a remote subsystem.
type RemoteSubsystem interface {
	// Name is the name of the subsystem.
	Name() string
	// Start starts the subsystem on the given channel.
	Start(ctx context.Context, channel ssh.Channel) error
	// Wait waits for the subsystem to finish.
	Wait() error
}

// remoteSubsystem is a subsystem that executes on a remote node.
type remoteSubsystem struct {
	logger *slog.Logger

	serverContext *srv.ServerContext
	subsystemName string

	ctx     context.Context
	errorCh chan error
}

// parseRemoteSubsystem returns *remoteSubsystem which can be used to run a subsystem on a remote node.
func parseRemoteSubsystem(ctx context.Context, subsystemName string, serverContext *srv.ServerContext) RemoteSubsystem {
	r := &remoteSubsystem{
		logger: slog.With(
			teleport.ComponentKey, teleport.ComponentRemoteSubsystem,
			"name", subsystemName,
		),
		serverContext: serverContext,
		subsystemName: subsystemName,
		ctx:           ctx,
	}
	if subsystemName == teleport.SFTPSubsystem {
		r.errorCh = make(chan error, 1) // one error for the sftp proxy as a whole
		return &remoteSFTPSubsystem{
			subsystem: r,
		}
	}
	r.errorCh = make(chan error, 3) // one error each for stdin, stdout, and stderr
	return r
}

// Name returns the name of the subsystem.
func (r *remoteSubsystem) Name() string {
	return r.subsystemName
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

	for range 3 {
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
		ServerMetadata: r.serverContext.GetServer().EventMetadata(),
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

type remoteSFTPSubsystem struct {
	subsystem *remoteSubsystem
	proxy     *SFTPProxy
}

// Name returns the name of the subsystem.
func (r *remoteSFTPSubsystem) Name() string {
	return r.subsystem.Name()
}

// Start will begin execution of the remote SFTP subsystem on the passed in channel.
func (r *remoteSFTPSubsystem) Start(ctx context.Context, channel ssh.Channel) error {
	proxy, err := NewSFTPProxy(r.subsystem.serverContext, channel, r.subsystem.logger)
	if err != nil {
		return trace.Wrap(err)
	}
	r.proxy = proxy

	go func() {
		defer r.subsystem.serverContext.RemoteSession.Close()
		errCh := make(chan error)
		go func() {
			errCh <- proxy.Serve()
			close(errCh)
		}()

		var err error
		select {
		case err = <-errCh: // Serve finished on its own, error is ready.
		case <-ctx.Done(): // Stop Serve and wait for error.
			proxy.Close()
			select {
			case err = <-errCh:
			case <-time.After(5 * time.Second):
				err = trace.Errorf("SFTP server timed out while closing")
			}
		}
		r.subsystem.errorCh <- err
	}()

	return nil
}

// Wait waits until the remote SFTP subsystem has finished execution.
func (r *remoteSFTPSubsystem) Wait() error {
	var err error
	select {
	case err = <-r.subsystem.errorCh:
		if errors.Is(err, io.EOF) {
			err = nil
		}
		if err != nil {
			r.subsystem.logger.WarnContext(r.subsystem.ctx, "Connection problem", "error", err)
		}
	case <-r.subsystem.ctx.Done():
		err = trace.ConnectionProblem(nil, "context is closing")
	}

	var exitStatus int
	if err != nil {
		exitStatus = 1
	}
	r.subsystem.serverContext.SendExecResult(r.subsystem.ctx, srv.ExecResult{
		Code: exitStatus,
	})

	// emit an event to the audit log with the result of execution
	r.subsystem.emitAuditEvent(r.subsystem.ctx, err)
	return trace.Wrap(err)
}
