/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package service

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	apidefaults "github.com/gravitational/teleport/api/defaults"
)

func TestConnectorClientSSHClientConfigSetsTeleportClientVersion(t *testing.T) {
	t.Parallel()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	signer, err := ssh.NewSignerFromSigner(privateKey)
	require.NoError(t, err)

	connector := &Connector{
		hostID: "test-host",
	}
	connector.clientState.Store(&connectorState{
		sshCertSigner: signer,
	})

	cfg, err := connector.clientSSHClientConfig(false)
	require.NoError(t, err)
	require.Equal(t, apidefaults.SSHClientVersion, cfg.ClientVersion)
}
