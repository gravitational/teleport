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

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_getLoggingOptsWithDefault(t *testing.T) {
	tests := []struct {
		name          string
		cf            *CLIConf
		debugEnvValue string
		osLogEnvValue string
		checkDebug    require.BoolAssertionFunc
		checkOSLog    require.BoolAssertionFunc
	}{
		{
			name:       "debug enabled by default",
			cf:         &CLIConf{},
			checkDebug: require.True,
			checkOSLog: require.False,
		},
		{
			name: "enabled by flag",
			cf: &CLIConf{
				DebugSetByUser: true,
				Debug:          true,
				OSLogSetByUser: true,
				OSLog:          true,
			},
			checkDebug: require.True,
			checkOSLog: require.True,
		},
		{
			name: "disabled by flag",
			cf: &CLIConf{
				DebugSetByUser: true,
			},
			checkDebug: require.False,
			checkOSLog: require.False,
		},
		{
			name:          "disabled by env",
			cf:            &CLIConf{},
			debugEnvValue: "false",
			osLogEnvValue: "false",
			checkDebug:    require.False,
			checkOSLog:    require.False,
		},
		{
			name:          "enabled by env",
			cf:            &CLIConf{},
			debugEnvValue: "true",
			osLogEnvValue: "true",
			checkDebug:    require.True,
			checkOSLog:    require.True,
		},
		{
			name:          "bad env",
			cf:            &CLIConf{},
			debugEnvValue: "what",
			osLogEnvValue: "what",
			checkDebug:    require.True,
			checkOSLog:    require.False,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(debugEnvVar, tt.debugEnvValue)
			t.Setenv(osLogEnvVar, tt.osLogEnvValue)
			opts := getLoggingOptsForMCPServer(tt.cf)
			tt.checkDebug(t, opts.debug)
			tt.checkOSLog(t, opts.osLog)
		})
	}
}
