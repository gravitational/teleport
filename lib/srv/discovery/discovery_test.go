/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package discovery

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/cloud"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

type mockSSMClient struct {
	ssmiface.SSMAPI
	commandOutput *ssm.SendCommandOutput
	invokeOutput  *ssm.GetCommandInvocationOutput
}

func (sm *mockSSMClient) SendCommandWithContext(_ context.Context, input *ssm.SendCommandInput, _ ...request.Option) (*ssm.SendCommandOutput, error) {
	return sm.commandOutput, nil
}

func (sm *mockSSMClient) GetCommandInvocationWithContext(_ context.Context, input *ssm.GetCommandInvocationInput, _ ...request.Option) (*ssm.GetCommandInvocationOutput, error) {
	return sm.invokeOutput, nil
}

func (sm *mockSSMClient) WaitUntilCommandExecutedWithContext(aws.Context, *ssm.GetCommandInvocationInput, ...request.WaiterOption) error {
	if aws.StringValue(sm.commandOutput.Command.Status) == ssm.CommandStatusFailed {
		return awserr.New(request.WaiterResourceNotReadyErrorCode, "err", nil)
	}
	return nil
}

type mockEmitter struct {
	eventHandler func(*testing.T, events.AuditEvent, *Server)
	server       *Server
	t            *testing.T
}

func (me *mockEmitter) EmitAuditEvent(ctx context.Context, event events.AuditEvent) error {
	if me.eventHandler != nil {
		me.eventHandler(me.t, event, me.server)
	}
	return nil
}

type mockEC2Client struct {
	ec2iface.EC2API
	output *ec2.DescribeInstancesOutput
}

func (m *mockEC2Client) DescribeInstancesPagesWithContext(
	ctx context.Context, input *ec2.DescribeInstancesInput,
	f func(dio *ec2.DescribeInstancesOutput, b bool) bool, opts ...request.Option) error {

	f(m.output, true)
	return nil
}

type testClient struct {
	services.Presence
	types.Events
}

func genEC2Instances(n int) []*ec2.Instance {
	var ec2Instances []*ec2.Instance
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("instance-id-%d", i)
		ec2Instances = append(ec2Instances, &ec2.Instance{
			InstanceId: aws.String(id),
			Tags: []*ec2.Tag{{
				Key:   aws.String("env"),
				Value: aws.String("dev"),
			}},
			State: &ec2.InstanceState{
				Name: aws.String(ec2.InstanceStateNameRunning),
			},
		})
	}
	return ec2Instances
}

func TestDiscoveryServer(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		name string
		// presentInstances is a list of servers already present in teleport
		presentInstances  []types.Server
		foundEC2Instances []*ec2.Instance
		ssm               *mockSSMClient
		emitter           *mockEmitter
		logHandler        func(*testing.T, io.Reader, chan struct{})
	}{
		{
			name:             "no nodes present, 1 found ",
			presentInstances: []types.Server{},
			foundEC2Instances: []*ec2.Instance{
				{
					InstanceId: aws.String("instance-id-1"),
					Tags: []*ec2.Tag{{
						Key:   aws.String("env"),
						Value: aws.String("dev"),
					}},
					State: &ec2.InstanceState{
						Name: aws.String(ec2.InstanceStateNameRunning),
					},
				},
			},
			ssm: &mockSSMClient{
				commandOutput: &ssm.SendCommandOutput{
					Command: &ssm.Command{
						CommandId: aws.String("command-id-1"),
					},
				},
				invokeOutput: &ssm.GetCommandInvocationOutput{
					Status:       aws.String(ssm.CommandStatusSuccess),
					ResponseCode: aws.Int64(0),
				},
			},
			emitter: &mockEmitter{
				eventHandler: func(t *testing.T, ae events.AuditEvent, server *Server) {
					t.Helper()
					defer server.Stop()
					require.Equal(t, ae, &events.SSMRun{
						Metadata: events.Metadata{
							Type: libevents.SSMRunEvent,
							Code: libevents.SSMRunSuccessCode,
						},
						CommandID:  "command-id-1",
						AccountID:  "owner",
						InstanceID: "instance-id-1",
						Region:     "eu-central-1",
						ExitCode:   0,
						Status:     ssm.CommandStatusSuccess,
					})
				},
			},
		},
		{
			name: "nodes present, instance filtered",
			presentInstances: []types.Server{
				&types.ServerV2{
					Kind: types.KindNode,
					Metadata: types.Metadata{
						Name: "name",
						Labels: map[string]string{
							types.AWSAccountIDLabel:  "owner",
							types.AWSInstanceIDLabel: "instance-id-1",
						},
						Namespace: defaults.Namespace,
					},
				},
			},
			foundEC2Instances: []*ec2.Instance{
				{
					InstanceId: aws.String("instance-id-1"),
					Tags: []*ec2.Tag{{
						Key:   aws.String("env"),
						Value: aws.String("dev"),
					}},
					State: &ec2.InstanceState{
						Name: aws.String(ec2.InstanceStateNameRunning),
					},
				},
			},
			ssm: &mockSSMClient{
				commandOutput: &ssm.SendCommandOutput{
					Command: &ssm.Command{
						CommandId: aws.String("command-id-1"),
					},
				},
				invokeOutput: &ssm.GetCommandInvocationOutput{
					Status:       aws.String(ssm.CommandStatusSuccess),
					ResponseCode: aws.Int64(0),
				},
			},
			emitter: &mockEmitter{},
			logHandler: func(t *testing.T, logs io.Reader, done chan struct{}) {
				scanner := bufio.NewScanner(logs)
				for scanner.Scan() {
					if strings.Contains(scanner.Text(),
						"All discovered EC2 instances are already part of the cluster.") {
						done <- struct{}{}
						return
					}
				}
			},
		},
		{
			name: "nodes present, instance not filtered",
			presentInstances: []types.Server{
				&types.ServerV2{
					Kind: types.KindNode,
					Metadata: types.Metadata{
						Name: "name",
						Labels: map[string]string{
							types.AWSAccountIDLabel:  "owner",
							types.AWSInstanceIDLabel: "wow-its-a-different-instance",
						},
						Namespace: defaults.Namespace,
					},
				},
			},
			foundEC2Instances: []*ec2.Instance{
				{
					InstanceId: aws.String("instance-id-1"),
					Tags: []*ec2.Tag{{
						Key:   aws.String("env"),
						Value: aws.String("dev"),
					}},
					State: &ec2.InstanceState{
						Name: aws.String(ec2.InstanceStateNameRunning),
					},
				},
			},
			ssm: &mockSSMClient{
				commandOutput: &ssm.SendCommandOutput{
					Command: &ssm.Command{
						CommandId: aws.String("command-id-1"),
					},
				},
				invokeOutput: &ssm.GetCommandInvocationOutput{
					Status:       aws.String(ssm.CommandStatusSuccess),
					ResponseCode: aws.Int64(0),
				},
			},
			emitter: &mockEmitter{},
			logHandler: func(t *testing.T, logs io.Reader, done chan struct{}) {
				scanner := bufio.NewScanner(logs)
				for scanner.Scan() {
					if strings.Contains(scanner.Text(),
						"Running Teleport installation on these instances: AccountID: owner, Instances: [instance-id-1]") {
						done <- struct{}{}
						return
					}
				}
			},
		},
		{
			name:              "chunked nodes get 2 log messages",
			presentInstances:  []types.Server{},
			foundEC2Instances: genEC2Instances(58),
			ssm: &mockSSMClient{
				commandOutput: &ssm.SendCommandOutput{
					Command: &ssm.Command{
						CommandId: aws.String("command-id-1"),
					},
				},
				invokeOutput: &ssm.GetCommandInvocationOutput{
					Status:       aws.String(ssm.CommandStatusSuccess),
					ResponseCode: aws.Int64(0),
				},
			},
			emitter: &mockEmitter{},
			logHandler: func(t *testing.T, logs io.Reader, done chan struct{}) {
				scanner := bufio.NewScanner(logs)
				instances := genEC2Instances(58)
				findAll := []string{genInstancesLogStr(instances[:50]), genInstancesLogStr(instances[50:])}
				index := 0
				for scanner.Scan() {
					if index == len(findAll) {
						done <- struct{}{}
						return
					}
					if strings.Contains(scanner.Text(), findAll[index]) {
						index++
					}
				}
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			testClients := cloud.TestCloudClients{
				EC2: &mockEC2Client{
					output: &ec2.DescribeInstancesOutput{
						Reservations: []*ec2.Reservation{
							{
								OwnerId:   aws.String("owner"),
								Instances: tc.foundEC2Instances,
							},
						},
					},
				},
				SSM: tc.ssm,
			}

			ctx := context.Background()

			bk, err := memory.New(memory.Config{
				Context: ctx,
			})
			require.NoError(t, err)

			client := testClient{
				Presence: local.NewPresenceService(bk),
				Events:   local.NewEventsService(bk),
			}

			for _, instance := range tc.presentInstances {
				_, err = client.UpsertNode(ctx, instance)
				require.NoError(t, err)
			}

			nodeWatcher, err := services.NewNodeWatcher(ctx, services.NodeWatcherConfig{
				ResourceWatcherConfig: services.ResourceWatcherConfig{
					Component: "discovery",
					Client:    client,
				},
			})
			require.NoError(t, err)
			defer nodeWatcher.Close()

			for !nodeWatcher.IsInitialized() {
				time.Sleep(100 * time.Millisecond)
			}

			server, err := New(context.Background(), &Config{
				Clients: &testClients,
				Matchers: []services.AWSMatcher{{
					Types:   []string{"EC2"},
					Regions: []string{"eu-central-1"},
					Tags:    map[string]utils.Strings{"teleport": {"yes"}},
					SSM:     &services.AWSSSM{DocumentName: "document"},
				}},
				Emitter:     tc.emitter,
				NodeWatcher: nodeWatcher,
			})
			require.NoError(t, err)

			tc.emitter.server = server
			tc.emitter.t = t

			r, w := io.Pipe()
			require.NoError(t, err)
			if tc.logHandler != nil {
				logger := logrus.New()
				logger.SetOutput(w)
				logger.SetLevel(logrus.DebugLevel)
				server.log = logrus.NewEntry(logger)
			}
			go server.Start()

			if tc.logHandler != nil {
				done := make(chan struct{})
				go tc.logHandler(t, r, done)
				timeoutCtx, cancelfn := context.WithTimeout(ctx, time.Second*5)
				defer cancelfn()
				select {
				case <-timeoutCtx.Done():
					t.Fatal("Timeout waiting for log entries")
					return
				case <-done:
					server.Stop()
					return
				}
			}

			server.Wait()
		})
	}

}
