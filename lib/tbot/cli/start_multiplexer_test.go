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

package cli

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/services/ssh"
)

func TestSSHMultiplexerCommand(t *testing.T) {
	testStartConfigureCommand(t, NewSSHMultiplexerCommand, []startConfigureTestCase{
		{
			name: "success",
			args: []string{
				"start",
				"ssh-multiplexer",
				"--destination=/foo",
				"--proxy-command=fdpass-teleport",
				"--proxy-command=foo",
				"--no-enable-resumption",
				"--proxy-templates-path=/bar.yaml",
			},
			assertConfig: func(t *testing.T, cfg *config.BotConfig) {
				require.Len(t, cfg.Services, 1)

				svc := cfg.Services[0]
				mx, ok := svc.(*ssh.MultiplexerConfig)
				require.True(t, ok)

				dir, ok := mx.Destination.(*destination.Directory)
				require.True(t, ok)
				require.Equal(t, "/foo", dir.Path)

				require.False(t, mx.SessionResumptionEnabled())
				require.Equal(t, []string{"fdpass-teleport", "foo"}, mx.ProxyCommand)
				require.Equal(t, "/bar.yaml", mx.ProxyTemplatesPath)
			},
		},
		{
			name: "minimal",
			args: []string{
				"start",
				"ssh-multiplexer",
				"--destination=/foo",
			},
			assertConfig: func(t *testing.T, cfg *config.BotConfig) {
				require.Len(t, cfg.Services, 1)

				svc := cfg.Services[0]
				mx, ok := svc.(*ssh.MultiplexerConfig)
				require.True(t, ok)

				dir, ok := mx.Destination.(*destination.Directory)
				require.True(t, ok)
				require.Equal(t, "/foo", dir.Path)

				require.True(t, mx.SessionResumptionEnabled())
				require.Empty(t, mx.ProxyCommand)
				require.Empty(t, mx.ProxyTemplatesPath)
			},
		},
	})
}
