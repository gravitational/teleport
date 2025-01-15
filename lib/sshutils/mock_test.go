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

package sshutils

import (
	"errors"
	"io"

	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/ssh"
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
	io.ReadWriter
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

type mockSSHConn struct {
	mockChan *mockChannel
}

func (mc *mockSSHConn) OpenChannel(name string, data []byte) (ssh.Channel, <-chan *ssh.Request, error) {
	return mc.mockChan, make(<-chan *ssh.Request), nil
}

type mockSSHNewChannel struct {
	mock.Mock
	ssh.NewChannel
}

func newMockSSHNewChannel(channelType string) *mockSSHNewChannel {
	m := new(mockSSHNewChannel)
	m.On("ChannelType").Return(channelType)
	m.On("Reject", mock.Anything, mock.Anything).Return(nil)
	return m
}

func (m *mockSSHNewChannel) ChannelType() string {
	return m.Called().String(0)
}

func (m *mockSSHNewChannel) Reject(reason ssh.RejectionReason, message string) error {
	args := m.Called(reason, message)
	return args.Error(0)
}

type mockSSHChannel struct {
	mock.Mock
	ssh.Channel
}

func newMockSSHChannel() *mockSSHChannel {
	m := new(mockSSHChannel)
	m.On("SendRequest", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
	return m
}

func (m *mockSSHChannel) SendRequest(name string, wantReply bool, payload []byte) (bool, error) {
	args := m.Called(name, wantReply, payload)
	return args.Bool(0), args.Error(1)
}

type mockSSHRequest struct {
	mock.Mock
}

func newMockSSHRequest() *mockSSHRequest {
	m := new(mockSSHRequest)
	m.On("Reply", mock.Anything, mock.Anything).Return(nil)
	return m
}

func (m *mockSSHRequest) Reply(ok bool, payload []byte) error {
	args := m.Called(ok, payload)
	return args.Error(0)
}
