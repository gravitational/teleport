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

package server

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/events"
	libevent "github.com/gravitational/teleport/lib/events"
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
	events []events.AuditEvent
}

func (me *mockEmitter) EmitAuditEvent(ctx context.Context, event events.AuditEvent) error {
	me.events = append(me.events, event)
	return nil
}

func TestSSMInstaller(t *testing.T) {
	document := "ssmdocument"

	for _, tc := range []struct {
		conf           SSMInstallerConfig
		req            SSMRunRequest
		expectedEvents []events.AuditEvent
		name           string
	}{
		{
			name: "ssm run was successful",
			req: SSMRunRequest{
				Instances: []EC2Instance{
					{InstanceID: "instance-id-1"},
				},
				DocumentName: document,
				Params:       map[string]string{"token": "abcdefg"},
				SSM: &mockSSMClient{
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
				Region:    "eu-central-1",
				AccountID: "account-id",
			},
			conf: SSMInstallerConfig{
				Emitter: &mockEmitter{},
			},
			expectedEvents: []events.AuditEvent{
				&events.SSMRun{
					Metadata: events.Metadata{
						Type: libevent.SSMRunEvent,
						Code: libevent.SSMRunSuccessCode,
					},
					CommandID:  "command-id-1",
					InstanceID: "instance-id-1",
					AccountID:  "account-id",
					Region:     "eu-central-1",
					ExitCode:   0,
					Status:     ssm.CommandStatusSuccess,
				},
			},
		},
		{
			name: "ssm run failed",
			req: SSMRunRequest{
				DocumentName: document,
				Instances: []EC2Instance{
					{InstanceID: "instance-id-1"},
				},
				Params: map[string]string{"token": "abcdefg"},
				SSM: &mockSSMClient{
					commandOutput: &ssm.SendCommandOutput{
						Command: &ssm.Command{
							CommandId: aws.String("command-id-1"),
						},
					},
					invokeOutput: &ssm.GetCommandInvocationOutput{
						Status:       aws.String(ssm.CommandStatusFailed),
						ResponseCode: aws.Int64(1),
					},
				},
				Region:    "eu-central-1",
				AccountID: "account-id",
			},
			conf: SSMInstallerConfig{
				Emitter: &mockEmitter{},
			},
			expectedEvents: []events.AuditEvent{
				&events.SSMRun{
					Metadata: events.Metadata{
						Type: libevent.SSMRunEvent,
						Code: libevent.SSMRunFailCode,
					},
					CommandID:  "command-id-1",
					InstanceID: "instance-id-1",
					AccountID:  "account-id",
					Region:     "eu-central-1",
					ExitCode:   1,
					Status:     ssm.CommandStatusFailed,
				},
			},
		},
		// todo(amk): test that incomplete commands eventually return
		// an event once completed
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			inst := NewSSMInstaller(tc.conf)
			err := inst.Run(ctx, tc.req)
			require.NoError(t, err)

			emitter := inst.Emitter.(*mockEmitter)
			require.Equal(t, tc.expectedEvents, emitter.events)
		})
	}
}
