/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package sshutils

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
)

func TestReply(t *testing.T) {
	r := NewReply(slog.With(teleport.Component, "test"))

	t.Run("RejectChannel", func(t *testing.T) {
		m := newMockSSHNewChannel("session")
		r.RejectChannel(context.Background(), m, ssh.ResourceShortage, "test error")
		m.AssertCalled(t, "Reject", ssh.ResourceShortage, "test error")
	})

	t.Run("RejectUnknownChannel", func(t *testing.T) {
		m := newMockSSHNewChannel("unknown_channel")
		r.RejectUnknownChannel(context.Background(), m)
		m.AssertCalled(t, "Reject", ssh.UnknownChannelType, "unknown channel type: unknown_channel")
	})

	t.Run("RejectWithAcceptError", func(t *testing.T) {
		m := newMockSSHNewChannel("session")
		r.RejectWithAcceptError(context.Background(), m, errors.New("test error"))
		m.AssertCalled(t, "Reject", ssh.ConnectionFailed, "unable to accept channel: test error")
	})

	t.Run("RejectWithNewRemoteSessionError", func(t *testing.T) {
		t.Run("internal error", func(t *testing.T) {
			m := newMockSSHNewChannel("session")
			r.RejectWithNewRemoteSessionError(context.Background(), m, errors.New("test error"))
			m.AssertCalled(t, "Reject", ssh.ConnectionFailed, "remote session open failed: test error")
		})
		t.Run("remote error", func(t *testing.T) {
			m := newMockSSHNewChannel("session")
			r.RejectWithNewRemoteSessionError(context.Background(), m, &ssh.OpenChannelError{
				Reason:  ssh.ResourceShortage,
				Message: "test error",
			})
			m.AssertCalled(t, "Reject", ssh.ResourceShortage, "test error")
		})
	})

	t.Run("ReplyError", func(t *testing.T) {
		m := newMockSSHRequest()
		r.ReplyError(context.Background(), m, errors.New("test error"))
		m.AssertCalled(t, "Reply", false, []byte("test error"))
	})

	t.Run("ReplyRequest", func(t *testing.T) {
		t.Run("ok true", func(t *testing.T) {
			m := newMockSSHRequest()
			r.ReplyRequest(context.Background(), m, true, []byte("ok true"))
			m.AssertCalled(t, "Reply", true, []byte("ok true"))
		})
		t.Run("ok false", func(t *testing.T) {
			m := newMockSSHRequest()
			r.ReplyRequest(context.Background(), m, false, []byte("ok false"))
			m.AssertCalled(t, "Reply", false, []byte("ok false"))
		})
	})

	t.Run("SendExitStatus", func(t *testing.T) {
		m := newMockSSHChannel()
		r.SendExitStatus(context.Background(), m, 1)
		m.AssertCalled(t, "SendRequest", "exit-status", false, []byte{0, 0, 0, 1})
	})
}
