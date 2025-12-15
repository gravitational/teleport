// Copyright 2025 Gravitational, Inc.
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
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestMakeIsHostAuthorityFunc(t *testing.T) {
	rawCA1, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	ca1, err := ssh.NewPublicKey(rawCA1)
	require.NoError(t, err)

	rawCA2, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	ca2, err := ssh.NewPublicKey(rawCA2)
	require.NoError(t, err)

	rawCA3, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	ca3, err := ssh.NewPublicKey(rawCA3)
	require.NoError(t, err)

	isHostAuthority := makeIsHostAuthorityFunc(func() ([]ssh.PublicKey, error) {
		return []ssh.PublicKey{ca1, ca2}, nil
	})

	cert1 := &ssh.Certificate{
		Key:          ca1,
		SignatureKey: ca1,
	}

	require.True(t, isHostAuthority(ca1, ""))
	require.True(t, isHostAuthority(ca2, ""))
	require.False(t, isHostAuthority(ca3, ""))

	require.False(t, isHostAuthority(cert1, ""), "a certificate signed by a certificate should not pass validation")
}
