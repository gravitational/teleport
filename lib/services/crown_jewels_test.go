/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMarshalCrownJewelRoundTrip(t *testing.T) {
	t.Parallel()

	spec := &crownjewelv1.CrownJewelSpec{}
	obj := &crownjewelv1.CrownJewel{
		Metadata: &headerv1.Metadata{
			Name: "dummy-crown-jewel",
		},
		Spec: spec,
	}

	out, err := MarshalCrownJewel(obj)
	require.NoError(t, err)
	newObj, err := UnmarshalCrownJewel(out)
	require.NoError(t, err)
	require.True(t, proto.Equal(obj, newObj), "messages are not equal")
}

func TestUnmarshalCrownJewel(t *testing.T) {
	t.Parallel()

	data, err := utils.ToJSON([]byte(correctCrownJewelYAML))
	require.NoError(t, err)

	expected := &crownjewelv1.CrownJewel{
		Version: "v1",
		Kind:    "crown_jewel",
		Metadata: &headerv1.Metadata{
			Name: "example-crown-jewel",
			Labels: map[string]string{
				"env": "example",
			},
		},
		Spec: &crownjewelv1.CrownJewelSpec{
			TeleportMatchers: []*crownjewelv1.TeleportMatcher{
				{
					Kinds: []string{"node"},
					Labels: []*labelv1.Label{
						{
							Name:   "abc",
							Values: []string{"xyz"},
						},
					},
				},
			},
			AwsMatchers: []*crownjewelv1.AWSMatcher{
				{
					Types:   []string{"ec2"},
					Regions: []string{"us-west-1"},
					Tags: []*crownjewelv1.AWSTag{
						{
							Key: "env",
							Values: []*wrapperspb.StringValue{
								wrapperspb.String("prod"),
							},
						},
					},
				},
			},
		},
	}

	obj, err := UnmarshalCrownJewel(data)
	require.NoError(t, err)
	require.True(t, proto.Equal(expected, obj), "CrownJewel objects are not equal")
}

const correctCrownJewelYAML = `
version: v1
kind: crown_jewel
metadata:
  labels:
    env: example
  name: example-crown-jewel
spec:
  aws_matchers:
    - regions:
        - us-west-1
      tags:
        - key: env
          values:
            - prod
      types:
        - ec2
  teleport_matchers:
    - kinds:
        - node
      labels:
        - name: abc
          values:
            - xyz
`
