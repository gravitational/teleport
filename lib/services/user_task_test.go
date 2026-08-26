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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMarshalUserTaskRoundTrip(t *testing.T) {
	t.Parallel()

	obj := &usertasksv1.UserTask{
		Version: "v1",
		Kind:    "user_task",
		Metadata: &headerv1.Metadata{
			Name: "example-user-task",
			Labels: map[string]string{
				"env": "example",
			},
		},
		Spec: &usertasksv1.UserTaskSpec{
			Integration: "my-integration",
			TaskType:    "discover-ec2",
			IssueType:   "SSM_AGENT_MISSING",
			State:       "OPEN",
			DiscoverEc2: &usertasksv1.DiscoverEC2{
				Region:    "us-east-1",
				AccountId: "123456789012",
				Instances: map[string]*usertasksv1.DiscoverEC2Instance{
					"i-1234567890": {
						Name:            "instance-name",
						InvocationUrl:   "https://example.com/",
						DiscoveryConfig: "config",
						DiscoveryGroup:  "group",
						SyncTime:        timestamppb.Now(),
					},
				},
			},
		},
	}

	out, err := MarshalUserTask(obj)
	require.NoError(t, err)
	newObj, err := UnmarshalUserTask(out)
	require.NoError(t, err)
	require.True(t, proto.Equal(obj, newObj), "messages are not equal")
}

func TestUnmarshalUserTask(t *testing.T) {
	t.Parallel()

	syncTime := timestamppb.Now()
	syncTimeString := syncTime.AsTime().Format(time.RFC3339Nano)

	correctUserTaskYAML := fmt.Sprintf(`
version: v1
kind: user_task
metadata:
  name: example-user-task
  labels:
    env: example
spec:
  integration: my-integration
  task_type: discover-ec2
  issue_type: SSM_AGENT_MISSING
  state: OPEN
  discover_ec2:
    region: us-east-1
    account_id: "123456789012"
    instances:
      i-1234567890:
        name: instance-name
        invocation_url: https://example.com/
        discovery_config: config
        discovery_group: group
        sync_time: "%s"
`, syncTimeString)

	data, err := utils.ToJSON([]byte(correctUserTaskYAML))
	require.NoError(t, err)

	expected := &usertasksv1.UserTask{
		Version: "v1",
		Kind:    "user_task",
		Metadata: &headerv1.Metadata{
			Name: "example-user-task",
			Labels: map[string]string{
				"env": "example",
			},
		},
		Spec: &usertasksv1.UserTaskSpec{
			Integration: "my-integration",
			TaskType:    "discover-ec2",
			IssueType:   "SSM_AGENT_MISSING",
			State:       "OPEN",
			DiscoverEc2: &usertasksv1.DiscoverEC2{
				Region:    "us-east-1",
				AccountId: "123456789012",
				Instances: map[string]*usertasksv1.DiscoverEC2Instance{
					"i-1234567890": {
						Name:            "instance-name",
						InvocationUrl:   "https://example.com/",
						DiscoveryConfig: "config",
						DiscoveryGroup:  "group",
						SyncTime:        syncTime,
					},
				},
			},
		},
	}

	obj, err := UnmarshalUserTask(data)
	require.NoError(t, err)
	require.True(t, proto.Equal(expected, obj), "UserTask objects are not equal")
}
