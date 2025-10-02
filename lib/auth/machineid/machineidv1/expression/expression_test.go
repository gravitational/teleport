// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package expression

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
)

func TestBotInstanceExpressionParser(t *testing.T) {
	parser, err := NewBotInstanceExpressionParser()
	require.NoError(t, err)

	makeBaseEnv := func() Environment {
		return Environment{
			Metadata: &headerv1.Metadata{
				Name: "test-bot-1/76efb07a-3077-471c-988a-54d0fa49fc71",
			},
			Spec: &machineidv1.BotInstanceSpec{
				BotName:    "test-bot-1",
				InstanceId: "76efb07a-3077-471c-988a-54d0fa49fc71",
			},
			LatestAuthentication: &machineidv1.BotInstanceStatusAuthentication{
				JoinMethod: "kubernetes",
			},
			LatestHeartbeat: &machineidv1.BotInstanceStatusHeartbeat{
				IsStartup:    false,
				Version:      "19.0.1",
				OneShot:      false,
				Architecture: "arm64",
				Os:           "linux",
				Hostname:     "test-hostname-1",
			},
		}
	}

	tcs := []struct {
		name     string
		expTrue  string
		expFalse string
		envFns   []func(*Environment)
	}{
		{
			name:     "equals name",
			expTrue:  `metadata.name == "test-bot-1/76efb07a-3077-471c-988a-54d0fa49fc71"`,
			expFalse: `metadata.name == "test-bot-2/bf8ad485-9a8f-483d-8bcb-9d2f8f8c48d0"`,
		},
		{
			name:     "equals name (short)",
			expTrue:  `name == "test-bot-1/76efb07a-3077-471c-988a-54d0fa49fc71"`,
			expFalse: `name == "test-bot-2/bf8ad485-9a8f-483d-8bcb-9d2f8f8c48d0"`,
		},
		{
			name:     "equals bot name",
			expTrue:  `spec.bot_name == "test-bot-1"`,
			expFalse: `spec.bot_name == "test-bot-2"`,
		},
		{
			name:     "equals instance id",
			expTrue:  `spec.instance_id == "76efb07a-3077-471c-988a-54d0fa49fc71"`,
			expFalse: `spec.instance_id == "80eefb93-e79c-47f1-8170-a025013da490"`,
		},
		{
			name:     "equals architecture",
			expTrue:  `status.latest_heartbeat.architecture == "arm64"`,
			expFalse: `status.latest_heartbeat.architecture == "amd64"`,
		},
		{
			name:     "equals os",
			expTrue:  `status.latest_heartbeat.os == "linux"`,
			expFalse: `status.latest_heartbeat.os == "windows"`,
		},
		{
			name:     "equals hostname",
			expTrue:  `status.latest_heartbeat.hostname == "test-hostname-1"`,
			expFalse: `status.latest_heartbeat.hostname == "test-hostname-2"`,
		},
		{
			name:     "equals one shot",
			expTrue:  `status.latest_heartbeat.one_shot`,
			expFalse: `!status.latest_heartbeat.one_shot`,
			envFns: []func(*Environment){
				func(e *Environment) {
					e.LatestHeartbeat = &machineidv1.BotInstanceStatusHeartbeat{
						OneShot: true,
					}
				},
			},
		},
		{
			name:     "exact version",
			expTrue:  `exact_version(status.latest_heartbeat.version, "19.0.1")`,
			expFalse: `exact_version(status.latest_heartbeat.version, "19.0.2-rc.1+56001")`,
		},
		{
			name:     "between versions - lower",
			expTrue:  `between(status.latest_heartbeat.version, "19.0.1", "19.0.2")`,
			expFalse: `between(status.latest_heartbeat.version, "19.0.2", "19.0.3")`,
		},
		{
			name:     "between versions - upper",
			expTrue:  `between(status.latest_heartbeat.version, "19.0.0", "19.0.2")`,
			expFalse: `between(status.latest_heartbeat.version, "19.0.0", "19.0.1")`,
		},
		{
			name:     "newer than version",
			expTrue:  `newer_than(status.latest_heartbeat.version, "19.0.0")`,
			expFalse: `newer_than(status.latest_heartbeat.version, "19.0.1")`,
		},
		{
			name:     "older than version",
			expTrue:  `older_than(status.latest_heartbeat.version, "19.0.2")`,
			expFalse: `older_than(status.latest_heartbeat.version, "19.0.1")`,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			env := makeBaseEnv()
			if tc.envFns != nil {
				for _, fn := range tc.envFns {
					fn(&env)
				}
			}

			if tc.expTrue != "" {
				exp, err := parser.Parse(tc.expTrue)
				require.NoError(t, err)
				result, err := exp.Evaluate(&env)
				require.NoError(t, err)
				assert.True(t, result)
			}

			if tc.expFalse != "" {
				exp, err := parser.Parse(tc.expFalse)
				require.NoError(t, err)
				result, err := exp.Evaluate(&env)
				require.NoError(t, err)
				assert.False(t, result)
			}
		})
	}
}
