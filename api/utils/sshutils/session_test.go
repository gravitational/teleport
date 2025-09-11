// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sshutils

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

type mockSSHChannel struct {
	ssh.Channel
}

func TestWrappedSSHConn(t *testing.T) {
	sshCh := new(mockSSHChannel)
	reqs := make(<-chan *ssh.Request)

	// ensure that OpenChannel returns the same SSH channel and requests
	// chan that wrappedSSHConn was given
	wrappedConn := &wrappedSSHConn{
		ch:   sshCh,
		reqs: reqs,
	}
	retCh, retReqs, err := wrappedConn.OpenChannel("", nil)
	require.NoError(t, err)
	require.Equal(t, sshCh, retCh)
	require.Equal(t, reqs, retReqs)

	// ensure the wrapped SSH conn will panic if OpenChannel is called
	// twice
	require.Panics(t, func() {
		wrappedConn.OpenChannel("", nil)
	})
}
