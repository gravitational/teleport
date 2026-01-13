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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	cloudclusterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/cloudcluster/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMarshalCloudClusterRoundTrip(t *testing.T) {
	t.Parallel()

	obj := &cloudclusterv1.CloudCluster{
		Metadata: &headerv1.Metadata{
			Name: "test-cloud-cluster",
		},
		Spec: &cloudclusterv1.CloudClusterSpec{
			AuthRegion: "us-west-2",
			Bot: &cloudclusterv1.Bot{
				Name: "test",
			},
			Token: &cloudclusterv1.Token{
				JoinMethod: "iam",
				Allow: []*cloudclusterv1.Allow{
					{
						AwsAccount: "account",
						AwsArn:     "arn",
					},
				},
			},
		},
	}

	out, err := MarshalCloudCluster(obj)
	require.NoError(t, err)
	newObj, err := UnmarshalCloudCluster(out)
	require.NoError(t, err)
	require.True(t, proto.Equal(obj, newObj), "messages are not equal")
}

func TestUnmarshalCloudCluster(t *testing.T) {
	t.Parallel()

	data, err := utils.ToJSON([]byte(correctCloudClusterYAML))
	require.NoError(t, err)

	expected := &cloudclusterv1.CloudCluster{
		Version: "v1",
		Kind:    "cloud_cluster",
		Metadata: &headerv1.Metadata{
			Name: "test-cloud-cluster",
			Labels: map[string]string{
				"env": "example",
			},
		},
		Spec: &cloudclusterv1.CloudClusterSpec{
			AuthRegion: "us-west-2",
			Bot: &cloudclusterv1.Bot{
				Name: "test",
			},
			Token: &cloudclusterv1.Token{
				JoinMethod: "iam",
				Allow: []*cloudclusterv1.Allow{
					{
						AwsAccount: "account",
						AwsArn:     "arn",
					},
				},
			},
		},
	}

	obj, err := UnmarshalCloudCluster(data)
	require.NoError(t, err)
	require.True(t, proto.Equal(expected, obj), "CloudCluster objects are not equal")
}

const correctCloudClusterYAML = `
version: v1
kind: cloud_cluster
metadata:
  labels:
    env: example
  name: test-cloud-cluster
spec:
  auth_region: us-west-2
  bot:
    name: test
  token:
    join_method: iam
    allow:
      - aws_account: account
        aws_arn: arn
`
