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
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/gravitational/teleport/api/types/events"
	libevent "github.com/gravitational/teleport/lib/events"
	"github.com/stretchr/testify/require"
)

type mockSSMClient struct {
	ssmiface.SSMAPI
	commandOutput *ssm.SendCommandOutput
	invokeOutput  *ssm.GetCommandInvocationOutput
}

func (sm *mockSSMClient) SendCommand(input *ssm.SendCommandInput) (*ssm.SendCommandOutput, error) {
	return sm.commandOutput, nil
}

func (sm *mockSSMClient) GetCommandInvocation(input *ssm.GetCommandInvocationInput) (*ssm.GetCommandInvocationOutput, error) {
	return sm.invokeOutput, nil
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
		expectedEvents []events.AuditEvent
		mutate         func(*ssm.GetCommandInvocationOutput)
	}{
		{
			conf: SSMInstallerConfig{
				Instances: []*ec2.Instance{
					{InstanceId: aws.String("instance-id-1")},
				},
				Params: map[string]string{"token": "abcdefg"},
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
				Emitter:   &mockEmitter{},
				Ctx:       context.Background(),
				Region:    "eu-central-1",
				AccountID: "account-id",
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
			conf: SSMInstallerConfig{
				Instances: []*ec2.Instance{
					{InstanceId: aws.String("instance-id-1")},
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
				Emitter:   &mockEmitter{},
				Ctx:       context.Background(),
				Region:    "eu-central-1",
				AccountID: "account-id",
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
		inst := NewSSMInstaller(tc.conf)
		timer := time.NewTicker(1)
		defer timer.Stop()
		inst.recheckTimer = timer

		err := inst.Run(document)
		require.NoError(t, err)

		if tc.mutate != nil {
			cl := tc.conf.SSM.(*mockSSMClient)
			tc.mutate(cl.invokeOutput)
		}

		emitter := inst.emitter.(*mockEmitter)

		require.Equal(t, tc.expectedEvents, emitter.events)
	}
}
