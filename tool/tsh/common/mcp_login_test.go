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

package common

import (
	"bytes"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/prompt"
)

func TestMCPLoginOAuthClientCredentials(t *testing.T) {
	newCommand := func() *mcpLoginCommand {
		return &mcpLoginCommand{
			cf: &CLIConf{
				Context:        t.Context(),
				overrideStderr: &bytes.Buffer{},
			},
		}
	}

	t.Run("dynamic registration", func(t *testing.T) {
		clientID, clientSecret, err := newCommand().getOAuthClientCredentials()
		require.NoError(t, err)
		require.Empty(t, clientID)
		require.Empty(t, clientSecret)
	})

	t.Run("public pre-registered client", func(t *testing.T) {
		cmd := newCommand()
		cmd.clientID = "client-id"

		clientID, clientSecret, err := cmd.getOAuthClientCredentials()
		require.NoError(t, err)
		require.Equal(t, "client-id", clientID)
		require.Empty(t, clientSecret)
	})

	t.Run("prompt for confidential client secret", func(t *testing.T) {
		oldStdin := prompt.Stdin()
		t.Cleanup(func() {
			prompt.SetStdin(oldStdin)
		})
		prompt.SetStdin(prompt.NewFakeReader().AddString("client-secret"))

		cmd := newCommand()
		cmd.clientID = "client-id"
		cmd.promptSecret = true

		clientID, clientSecret, err := cmd.getOAuthClientCredentials()
		require.NoError(t, err)
		require.Equal(t, "client-id", clientID)
		require.Equal(t, "client-secret", clientSecret)
	})

	t.Run("secret requires client ID", func(t *testing.T) {
		cmd := newCommand()
		cmd.promptSecret = true

		_, _, err := cmd.getOAuthClientCredentials()
		require.True(t, trace.IsBadParameter(err))
		require.ErrorContains(t, err, "requires --client-id")
	})

	t.Run("empty client secret", func(t *testing.T) {
		oldStdin := prompt.Stdin()
		t.Cleanup(func() {
			prompt.SetStdin(oldStdin)
		})
		prompt.SetStdin(prompt.NewFakeReader().AddString(""))

		cmd := newCommand()
		cmd.clientID = "client-id"
		cmd.promptSecret = true

		_, _, err := cmd.getOAuthClientCredentials()
		require.True(t, trace.IsBadParameter(err))
		require.ErrorContains(t, err, "client secret is empty")
	})
}
