/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
)

type fakeAppGateway struct {
	gateway.App
	protocol     string
	generatedUrl string
}

func (m fakeAppGateway) Protocol() string      { return m.protocol }
func (m fakeAppGateway) LocalProxyURL() string { return m.generatedUrl }

func TestNewAppCLICommand(t *testing.T) {
	testCases := []struct {
		name         string
		protocol     string
		generatedUrl string
		output       string
	}{
		{
			name:         "TCP app",
			protocol:     types.ApplicationProtocolTCP,
			generatedUrl: "tcp://localhost:8888",
			output:       "",
		},
		{
			name:         "HTTP app",
			protocol:     types.ApplicationProtocolHTTP,
			generatedUrl: "http://localhost:8888",
			output:       "curl http://localhost:8888",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			mockGateway := fakeAppGateway{
				protocol:     tc.protocol,
				generatedUrl: tc.generatedUrl,
			}

			command, err := NewAppCLICommand(mockGateway)

			require.NoError(t, err)
			cmdString := strings.TrimSpace(
				fmt.Sprintf("%s %s",
					strings.Join(command.Env, " "),
					strings.Join(command.Args, " ")))

			require.Equal(t, cmdString, tc.output)
		})
	}
}
