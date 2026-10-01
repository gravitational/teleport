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

package common

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"

	"github.com/go-git/go-git/v5/plumbing/format/pktline"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/utils/log"
)

// Repository is the repository path in the git command (e.g.
// "gravitational/teleport.git").
type Repository string

// Owner returns the first part of the repository path. If repository does not
// have multiple parts, empty string will be returned.
//
// For GitHub, owner is either the user or the organization that owns the repo.
func (r Repository) Owner() string {
	if owner, _, ok := strings.Cut(string(r), "/"); ok {
		return owner
	}
	return ""
}

// Command is the Git command to be executed.
type Command struct {
	// RequestLine is the client request string. For SSH, this is the exec
	// request (e.g. "git-upload-pack 'repo'"). For HTTP, this is the method
	// and path (e.g. "POST /org/repo.git/git-receive-pack").
	RequestLine string
	// Service is the git service of the command (either git-upload-pack or
	// git-receive-pack).
	Service string
	// Repository returns the repository path of the command.
	Repository Repository
}

// CommandRecorder records Git commands by implementing io.Writer to receive a
// copy of stdin from the git client.
type CommandRecorder interface {
	// Writer is the basic interface for the recorder to receive payload.
	io.Writer

	// GetCommand returns basic info of the command.
	GetCommand() Command
	// GetActions returns the action details of the command. The actions may
	// be incomplete when an error is returned.
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
}

func newPushCommandRecorder(parentCtx context.Context, command Command) *pushCommandRecorder {
	return &pushCommandRecorder{
		Command:   command,
		parentCtx: parentCtx,
		logger:    slog.With(teleport.ComponentKey, "git:packp"),
	}
}

func (r *pushCommandRecorder) GetCommand() Command {
	return r.Command
}

func (r *pushCommandRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// maxPushCommandPayloadSize is the maximum number of bytes the push
	// recorder keeps from the start of the request.
	const maxPushCommandPayloadSize = 64 * 1024

	// The command-list comes first and ends with a flush-pkt, followed by the
	// packfile which can be large. Writes are arbitrary chunks of the stream
	// so only keep a bounded prefix. This only caps the recorder's copy. The
	// stream itself is not affected.
	//
	// https://git-scm.com/docs/pack-protocol#_reference_update_request_and_packfile_transfer
	remaining := maxPushCommandPayloadSize - len(r.payload)
	if remaining <= 0 {
		r.logger.Log(r.parentCtx, log.TraceLevel, "Discarding packet protocol", "packet_length", len(p))
		return len(p), nil
	}

	kept := p[:min(len(p), remaining)]
	r.logger.Log(r.parentCtx, log.TraceLevel, "Recording Git command in packet protocol", "packet_length", len(kept))
	r.payload = append(r.payload, kept...)
	return len(p), nil
}

func (r *pushCommandRecorder) GetActions() ([]*apievents.GitCommandAction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Noop push (e.g. "Everything up-to-date")
	if bytes.Equal(r.payload, pktline.FlushPkt) {
		return nil, nil
	}

	// The payload is the start of the reference update request: the
	// command-list ending with a flush-pkt, followed by the packfile. Since the
	// payload is capped, it can be decoded as:
	// - Flush-pkt captured: the decoder stops at the flush-pkt with all
	//   commands and returns success. The rest of the payload is not read.
	// - Payload cut inside a pkt-line of the command-list: reading that
	//   pkt-line fails with io.ErrUnexpectedEOF.
	// - Payload cut exactly between pkt-lines of the command-list, so it is
	//   unknown whether more commands follow the cut: the decoder normally
	//   treats a clean end of data as the end of the command-list, so
	//   unexpectedEOFReader is chained here to make reading the next pkt-line
	//   fail instead.
	// For a partial capture, both the partial list and an error are returned.
	//
	// TODO(greedy52) add actions_truncated to git.command events to indicate a
	// partial list of actions.
	request := packp.NewReferenceUpdateRequest()
	decodeErr := request.Decode(io.MultiReader(bytes.NewReader(r.payload), unexpectedEOFReader{}))

	var actions []*apievents.GitCommandAction
	for _, command := range request.Commands {
		actions = append(actions, &apievents.GitCommandAction{
			Action:    string(command.Action()),
			Reference: string(command.Name),
			Old:       command.Old.String(),
			New:       command.New.String(),
		})
	}
	return actions, trace.Wrap(decodeErr)
}

type unexpectedEOFReader struct{}

func (unexpectedEOFReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}
