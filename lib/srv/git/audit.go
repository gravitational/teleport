/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package git

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"sync"

	"github.com/go-git/go-git/v5/plumbing/format/pktline"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/utils/log"
)

// CommandRecorder records Git commands by implementing io.Writer to receive a
// copy of stdin from the git client.
type CommandRecorder interface {
	// Writer is the basic interface for the recorder to receive payload.
	io.Writer

	// GetCommand returns basic info of the command.
	GetCommand() Command
	// GetActions returns the action details of the command.
	GetActions() ([]*apievents.GitCommandAction, error)
}

// NewCommandRecorder returns a new Git command recorder.
//
// The recorder receives stdin input from the git client which is in Git pack
// protocol format:
// https://git-scm.com/docs/pack-protocol#_ssh_transport
//
// For SSH transport, the command type and the repository come from the command
// executed through the SSH session.
//
// Based on the command type, we can decode the pack protocol defined above. In
// general, some metadata are exchanged in pkt-lines followed by packfile sent
// via side-bands.
func NewCommandRecorder(parentCtx context.Context, command Command) CommandRecorder {
	// For now, only record details on the push. Fetch is not very interesting.
	if command.Service == transport.ReceivePackServiceName {
		return newPushCommandRecorder(parentCtx, command)
	}
	return newNoopRecorder(command)
}

// noopRecorder is a no-op recorder that implements CommandRecorder
type noopRecorder struct {
	Command
}

func newNoopRecorder(command Command) *noopRecorder {
	return &noopRecorder{
		Command: command,
	}
}

func (r *noopRecorder) GetCommand() Command {
	return r.Command
}
func (r *noopRecorder) GetActions() ([]*apievents.GitCommandAction, error) {
	return nil, nil
}
func (r *noopRecorder) Write(p []byte) (int, error) {
	return len(p), nil
}

// pushCommandRecorder records actions for git-receive-pack.
type pushCommandRecorder struct {
	Command

	parentCtx context.Context
	logger    *slog.Logger
	payload   []byte
	mu        sync.Mutex
	seenFlush bool
}

func newPushCommandRecorder(parentCtx context.Context, command Command) *pushCommandRecorder {
	return &pushCommandRecorder{
		Command: command,
		logger:  slog.With(teleport.ComponentKey, "git:packp"),
	}
}

func (r *pushCommandRecorder) GetCommand() Command {
	return r.Command
}

func (r *pushCommandRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Avoid caching packfile as it can be large. Look for flush-pkt which
	// comes after the command-list.
	//
	// https://git-scm.com/docs/pack-protocol#_reference_update_request_and_packfile_transfer
	if r.seenFlush {
		r.logger.Log(r.parentCtx, log.TraceLevel, "Discarding packet protocol", "packet_length", len(p))
		return len(p), nil
	}

	r.logger.Log(r.parentCtx, log.TraceLevel, "Recording Git command in packet protocol", "packet", string(p))
	r.payload = append(r.payload, p...)
	if bytes.HasSuffix(p, pktline.FlushPkt) {
		r.seenFlush = true
	}
	return len(p), nil
}

func (r *pushCommandRecorder) GetActions() ([]*apievents.GitCommandAction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Noop push (e.g. "Everything up-to-date")
	if bytes.Equal(r.payload, pktline.FlushPkt) {
		return nil, nil
	}

	request := packp.NewReferenceUpdateRequest()
	if err := request.Decode(bytes.NewReader(r.payload)); err != nil {
		return nil, trace.Wrap(err)
	}
	var actions []*apievents.GitCommandAction
	for _, command := range request.Commands {
		actions = append(actions, &apievents.GitCommandAction{
			Action:    string(command.Action()),
			Reference: string(command.Name),
			Old:       command.Old.String(),
			New:       command.New.String(),
		})
	}
	return actions, nil
}
