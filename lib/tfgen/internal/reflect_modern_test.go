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
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tfgen/internal"
)

func TestReflectModern(t *testing.T) {
	now := time.Now().UTC()

	input := &machineidv1.Bot{
		Kind:    types.KindBot,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:        "my-bot",
			Description: "hello",
			Expires:     timestamppb.New(now),
			Labels:      map[string]string{"foo": "bar"},
		},
		Spec: &machineidv1.BotSpec{
			Roles: []string{"foo"},
			Traits: []*machineidv1.Trait{
				{
					Name:   "logins",
					Values: []string{"root", "ubuntu"},
				},
			},
			MaxSessionTtl: durationpb.New(1 * time.Hour),
		},
	}

	expected := &internal.Message{
		Attributes: []internal.Attribute{
			attribute("kind", stringVal("bot")),
			attribute("sub_kind", stringVal("")),
			attribute("version", stringVal(types.V1)),
			attribute("metadata",
				messageVal(
					attribute("name", stringVal("my-bot")),
					attribute("namespace", stringVal("")),
					attribute("description", stringVal("hello")),
					attribute("labels",
						mapVal(
							map[any]*internal.Value{
								"foo": stringVal("bar"),
							},
						),
					),
					attribute("expires", timestampVal(now)),
					attribute("revision", stringVal("")),
				),
			),
			attribute("spec",
				messageVal(
					attribute("roles", listVal(stringVal("foo"))),
					attribute("traits",
						listVal(
							messageVal(
								attribute("name", stringVal("logins")),
								attribute("values",
									listVal(
										stringVal("root"),
										stringVal("ubuntu"),
									),
								),
							),
						),
					),
					attribute("max_session_ttl", durationVal(1*time.Hour)),
				),
			),
			attribute("status",
				messageVal(
					attribute("user_name", stringVal("")),
					attribute("role_name", stringVal("")),
				),
			),
		},
	}

	output, err := internal.ReflectModern(input)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected, output))
}
