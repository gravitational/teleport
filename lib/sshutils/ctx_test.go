/*
Copyright 2022 Gravitational, Inc.

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

type nopReadWriter struct{}

func (n nopReadWriter) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (n nopReadWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

type mockChannel struct {
	nopReadWriter
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
	return nopReadWriter{}
}

func TestAgentChannelClose(t *testing.T) {
	aChannel := agentChannel{
		ch: &mockChannel{},
	}
	// Ensure write part of channel is closed first
	require.EqualError(t, aChannel.Close(),
		trace.NewAggregate(errCloseWrite, errClose).Error())
}
