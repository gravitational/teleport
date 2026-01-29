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

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadclusterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadcluster/v1"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMarshalWorkloadClusterRoundTrip(t *testing.T) {
	t.Parallel()

	obj := &workloadclusterv1.WorkloadCluster{
		Metadata: &headerv1.Metadata{
			Name: "test-workload-cluster",
		},
		Spec: &workloadclusterv1.WorkloadClusterSpec{
			Regions: []*workloadclusterv1.Region{
				{
					Name: "us-west-2",
				},
			},
			Bot: &workloadclusterv1.Bot{
				Name: "test",
			},
			Token: &workloadclusterv1.Token{
				JoinMethod: "iam",
				Allow: []*workloadclusterv1.Allow{
					{
						AwsAccount: "account",
						AwsArn:     "arn",
					},
				},
			},
		},
	}

	out, err := MarshalWorkloadCluster(obj)
	require.NoError(t, err)
	newObj, err := UnmarshalWorkloadCluster(out)
	require.NoError(t, err)
	require.True(t, proto.Equal(obj, newObj), "messages are not equal")
}

func TestUnmarshalWorkloadCluster(t *testing.T) {
	t.Parallel()

	data, err := utils.ToJSON([]byte(correctWorkloadClusterYAML))
	require.NoError(t, err)

	expected := &workloadclusterv1.WorkloadCluster{
		Version: "v1",
		Kind:    "workload_cluster",
		Metadata: &headerv1.Metadata{
			Name: "test-workload-cluster",
			Labels: map[string]string{
				"env": "example",
			},
		},
		Spec: &workloadclusterv1.WorkloadClusterSpec{
			Regions: []*workloadclusterv1.Region{
				{
					Name: "us-west-2",
				},
			},
			Bot: &workloadclusterv1.Bot{
				Name: "test",
			},
			Token: &workloadclusterv1.Token{
				JoinMethod: "iam",
				Allow: []*workloadclusterv1.Allow{
					{
						AwsAccount: "account",
						AwsArn:     "arn",
					},
				},
			},
		},
	}

	obj, err := UnmarshalWorkloadCluster(data)
	require.NoError(t, err)
	require.True(t, proto.Equal(expected, obj), "WorkloadCluster objects are not equal")
}

const correctWorkloadClusterYAML = `
version: v1
kind: workload_cluster
metadata:
  labels:
    env: example
  name: test-workload-cluster
spec:
  regions:
    - name: us-west-2
  bot:
    name: test
  token:
    join_method: iam
    allow:
      - aws_account: account
        aws_arn: arn
`
