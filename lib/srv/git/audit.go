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
	GetActions() []*apievents.GitCommandAction
}

// NewCommandRecorder returns a new Git command recorder.
func NewCommandRecorder(command Command) CommandRecorder {
	// For now, only record details on the push. Fetch is not very interesting.
	if command.Service == transport.ReceivePackServiceName {
		return newPushCommandRecorder(command)
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
func (r *noopRecorder) GetActions() []*apievents.GitCommandAction {
	return nil
}
func (r *noopRecorder) Write(p []byte) (int, error) {
	return len(p), nil
}

// pushCommandRecoder records actions for git-receive-pack.
type pushCommandRecorder struct {
	Command

	logger  *slog.Logger
	payload []byte
	mu      sync.Mutex
}

func newPushCommandRecorder(command Command) *pushCommandRecorder {
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
	if bytes.HasSuffix(r.payload, pktline.FlushPkt) {
		if len(p) > 0 {
			r.logger.Log(context.Background(), log.TraceLevel, "Discarding packet protocol", "packet_length", len(p))
		}
		return len(p), nil
	}

	r.logger.Log(context.Background(), log.TraceLevel, "Recording Git command in packet protocol", "packet", string(p))
	r.payload = append(r.payload, p...)
	return len(p), nil
}

func (r *pushCommandRecorder) GetActions() (actions []*apievents.GitCommandAction) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Noop push (e.g. "Everything up-to-date")
	if bytes.Equal(r.payload, pktline.FlushPkt) {
		return nil
	}

	request := packp.NewReferenceUpdateRequest()
	if err := request.Decode(bytes.NewReader(r.payload)); err != nil {
		r.logger.WarnContext(context.Background(), "failed to decode push command", "error", err)
		return
	}
	for _, command := range request.Commands {
		actions = append(actions, &apievents.GitCommandAction{
			Action:    string(command.Action()),
			Reference: string(command.Name),
			Old:       command.Old.String(),
			New:       command.New.String(),
		})
	}
	return
}
