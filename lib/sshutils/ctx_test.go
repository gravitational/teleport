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

package sshutils

import (
	"errors"
	"io"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

var (
	errCloseWrite = errors.New("write closed")
	errClose      = errors.New("channel closed")
)

type fakeReaderWriter struct{}

func (n fakeReaderWriter) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (n fakeReaderWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

type mockChannel struct {
	fakeReaderWriter
}

func (mc *mockChannel) Close() error {
	return errClose
}

func (mc *mockChannel) CloseWrite() error {
	return errCloseWrite
}

func (mc *mockChannel) SendRequest(name string, wantReply bool, payload []byte) (bool, error) {
	return false, nil
}

func (mc *mockChannel) Stderr() io.ReadWriter {
	return fakeReaderWriter{}
}

func TestAgentChannelClose(t *testing.T) {
	aChannel := agentChannel{
		ch: &mockChannel{},
	}
	// Ensure write part of channel is closed first
	require.EqualError(t, aChannel.Close(),
		trace.NewAggregate(errCloseWrite, errClose).Error())
}
