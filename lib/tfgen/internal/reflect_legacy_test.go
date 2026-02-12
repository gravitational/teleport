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

package internal_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tfgen/internal"
)

func TestReflectLegacy(t *testing.T) {
	now := time.Now().UTC()

	token, err := types.NewProvisionTokenFromSpec(
		"my-token",
		now,
		types.ProvisionTokenSpecV2{
			Roles:      []types.SystemRole{types.RoleBot},
			BotName:    "my-bot",
			JoinMethod: types.JoinMethodGitHub,
			GitHub: &types.ProvisionTokenSpecV2GitHub{
				Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
					{
						Repository: "gravitational/teleport",
					},
				},
			},
			SuggestedLabels: types.Labels{
				"foo": {"bar", "baz"},
			},
		},
	)
	require.NoError(t, err)

	expected := &internal.Message{
		Attributes: []internal.Attribute{
			attribute("kind", stringVal(types.KindToken)),
			attribute("sub_kind", stringVal("")),
			attribute("version", stringVal(types.V2)),
			attribute("metadata",
				messageVal(
					attribute("name", stringVal("my-token")),
					attribute("description", stringVal("")),
					attribute("labels", mapVal(nil)),
					attribute("expires", timestampVal(now)),
					attribute("revision", stringVal("")),
				),
			),
			attribute("spec",
				messageVal(
					attribute("roles", listVal(stringVal("Bot"))),
					attribute("allow", listVal()),
					attribute("aws_iid_ttl", durationVal(0)),
					attribute("join_method", stringVal("github")),
					attribute("bot_name", stringVal("my-bot")),
					attribute("suggested_labels",
						mapVal(
							map[any]*internal.Value{
								"foo": listVal(stringVal("bar"), stringVal("baz")),
							},
						),
					),
					attribute("github",
						messageVal(
							attribute("allow",
								listVal(
									messageVal(
										attribute("sub", stringVal("")),
										attribute("repository", stringVal("gravitational/teleport")),
										attribute("repository_owner", stringVal("")),
										attribute("workflow", stringVal("")),
										attribute("environment", stringVal("")),
										attribute("actor", stringVal("")),
										attribute("ref", stringVal("")),
										attribute("ref_type", stringVal("")),
									),
								),
							),
							attribute("enterprise_server_host", stringVal("")),
							attribute("enterprise_slug", stringVal("")),
							attribute("static_jwks", stringVal("")),
						),
					),
					attribute("suggested_agent_matcher_labels", mapVal(nil)),
					attribute("integration", stringVal("")),
				),
			),
		},
	}

	output, err := internal.ReflectLegacy(token.(internal.LegacyProtoMessage))
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected, output))
}
