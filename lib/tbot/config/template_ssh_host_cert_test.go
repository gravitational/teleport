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

package config

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/fixtures"
)

func TestTemplateSSHHostCertRender(t *testing.T) {
	ctx := context.Background()
	cfg, err := newTestConfig("example.com")
	require.NoError(t, err)

	mockBot := newMockProvider(cfg)

	tmpl := templateSSHHostCert{
		principals: []string{"foo.example.com"},
	}

	dest := &DestinationMemory{}
	require.NoError(t, dest.CheckAndSetDefaults())

	ident := getTestIdent(t, "bot-test")
	err = tmpl.render(ctx, mockBot, ident, dest)
	require.NoError(t, err)

	// Make sure a cert is written. We just use a dummy cert (the CA fixture)
	certBytes, err := dest.Read(ctx, defaultSSHHostCertPrefix+sshHostCertSuffix)
	require.NoError(t, err)

	require.Equal(t, fixtures.SSHCAPublicKey, string(certBytes))

	// Make sure a CA is written.
	caBytes, err := dest.Read(ctx, defaultSSHHostCertPrefix+sshHostUserCASuffix)
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(string(caBytes), fixtures.SSHCAPublicKey))
}
