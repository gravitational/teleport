/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package server

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/events"
	libevent "github.com/gravitational/teleport/lib/events"
)

type mockSSMClient struct {
	ssmiface.SSMAPI
	commandOutput          *ssm.SendCommandOutput
	commandInvokeOutput    map[string]*ssm.GetCommandInvocationOutput
	describeOutput         *ssm.DescribeInstanceInformationOutput
	listCommandInvocations *ssm.ListCommandInvocationsOutput
}

const docWithoutSSHDConfigPathParam = "ssmdocument-without-sshdConfigPath-param"

func (sm *mockSSMClient) SendCommandWithContext(_ context.Context, input *ssm.SendCommandInput, _ ...request.Option) (*ssm.SendCommandOutput, error) {
	if _, hasExtraParam := input.Parameters["sshdConfigPath"]; hasExtraParam && aws.StringValue(input.DocumentName) == docWithoutSSHDConfigPathParam {
		return nil, fmt.Errorf("InvalidParameters: document %s does not support parameters", docWithoutSSHDConfigPathParam)
	}
	return sm.commandOutput, nil
}

func (sm *mockSSMClient) GetCommandInvocationWithContext(_ context.Context, input *ssm.GetCommandInvocationInput, _ ...request.Option) (*ssm.GetCommandInvocationOutput, error) {
	if stepResult, found := sm.commandInvokeOutput[aws.StringValue(input.PluginName)]; found {
		return stepResult, nil
	}
	return nil, &ssm.InvalidPluginName{}
}

func (sm *mockSSMClient) DescribeInstanceInformationWithContext(_ context.Context, input *ssm.DescribeInstanceInformationInput, _ ...request.Option) (*ssm.DescribeInstanceInformationOutput, error) {
	if sm.describeOutput == nil {
		return nil, awserr.NewRequestFailure(awserr.New("AccessDeniedException", "message", nil), http.StatusBadRequest, uuid.NewString())
	}
	return sm.describeOutput, nil
}

func (sm *mockSSMClient) ListCommandInvocationsWithContext(aws.Context, *ssm.ListCommandInvocationsInput, ...request.Option) (*ssm.ListCommandInvocationsOutput, error) {
	if sm.listCommandInvocations == nil {
		return nil, awserr.NewRequestFailure(awserr.New("AccessDeniedException", "message", nil), http.StatusBadRequest, uuid.NewString())
	}
	return sm.listCommandInvocations, nil
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
					commandInvokeOutput: map[string]*ssm.GetCommandInvocationOutput{
						"downloadContent": {
							Status:       aws.String(ssm.CommandStatusSuccess),
							ResponseCode: aws.Int64(0),
						},
						"runShellScript": {
							Status:       aws.String(ssm.CommandStatusSuccess),
							ResponseCode: aws.Int64(0),
						},
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
					CommandID:     "command-id-1",
					InstanceID:    "instance-id-1",
					AccountID:     "account-id",
					Region:        "eu-central-1",
					ExitCode:      0,
					Status:        ssm.CommandStatusSuccess,
					InvocationURL: "https://eu-central-1.console.aws.amazon.com/systems-manager/run-command/command-id-1/instance-id-1",
				},
			},
		},
		{
			name: "params include sshdConfigPath",
			req: SSMRunRequest{
				Instances: []EC2Instance{
					{InstanceID: "instance-id-1"},
				},
				DocumentName: docWithoutSSHDConfigPathParam,
				Params:       map[string]string{"sshdConfigPath": "abcdefg"},
				SSM: &mockSSMClient{
					commandOutput: &ssm.SendCommandOutput{
						Command: &ssm.Command{
							CommandId: aws.String("command-id-1"),
						},
					},
					commandInvokeOutput: map[string]*ssm.GetCommandInvocationOutput{
						"downloadContent": {
							Status:       aws.String(ssm.CommandStatusSuccess),
							ResponseCode: aws.Int64(0),
						},
						"runShellScript": {
							Status:       aws.String(ssm.CommandStatusSuccess),
							ResponseCode: aws.Int64(0),
						},
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
					CommandID:     "command-id-1",
					InstanceID:    "instance-id-1",
					AccountID:     "account-id",
					Region:        "eu-central-1",
					ExitCode:      0,
					Status:        ssm.CommandStatusSuccess,
					InvocationURL: "https://eu-central-1.console.aws.amazon.com/systems-manager/run-command/command-id-1/instance-id-1",
				},
			},
		},
		{
			name: "ssm run failed in download content",
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
					commandInvokeOutput: map[string]*ssm.GetCommandInvocationOutput{
						"downloadContent": {
							Status:                aws.String(ssm.CommandStatusFailed),
							ResponseCode:          aws.Int64(1),
							StandardErrorContent:  aws.String("timeout error"),
							StandardOutputContent: aws.String(""),
						},
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
					CommandID:      "command-id-1",
					InstanceID:     "instance-id-1",
					AccountID:      "account-id",
					Region:         "eu-central-1",
					ExitCode:       1,
					Status:         ssm.CommandStatusFailed,
					StandardOutput: "",
					StandardError:  "timeout error",
					InvocationURL:  "https://eu-central-1.console.aws.amazon.com/systems-manager/run-command/command-id-1/instance-id-1",
				},
			},
		},
		{
			name: "ssm run failed in run shell script",
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
					commandInvokeOutput: map[string]*ssm.GetCommandInvocationOutput{
						"downloadContent": {
							Status:                aws.String(ssm.CommandStatusSuccess),
							ResponseCode:          aws.Int64(0),
							StandardErrorContent:  aws.String("no error"),
							StandardOutputContent: aws.String(""),
						},
						"runShellScript": {
							Status:                aws.String(ssm.CommandStatusFailed),
							ResponseCode:          aws.Int64(1),
							StandardErrorContent:  aws.String("timeout error"),
							StandardOutputContent: aws.String(""),
						},
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
					CommandID:      "command-id-1",
					InstanceID:     "instance-id-1",
					AccountID:      "account-id",
					Region:         "eu-central-1",
					ExitCode:       1,
					Status:         ssm.CommandStatusFailed,
					StandardOutput: "",
					StandardError:  "timeout error",
					InvocationURL:  "https://eu-central-1.console.aws.amazon.com/systems-manager/run-command/command-id-1/instance-id-1",
				},
			},
		},
		{
			name: "detailed events if ssm:DescribeInstanceInformation is available",
			req: SSMRunRequest{
				Instances: []EC2Instance{
					{InstanceID: "instance-id-1"},
					{InstanceID: "instance-id-2"},
					{InstanceID: "instance-id-3"},
					{InstanceID: "instance-id-4"},
				},
				DocumentName: document,
				Params:       map[string]string{"token": "abcdefg"},
				SSM: &mockSSMClient{
					commandOutput: &ssm.SendCommandOutput{
						Command: &ssm.Command{
							CommandId: aws.String("command-id-1"),
						},
					},
					commandInvokeOutput: map[string]*ssm.GetCommandInvocationOutput{
						"downloadContent": {
							Status:       aws.String(ssm.CommandStatusSuccess),
							ResponseCode: aws.Int64(0),
						},
						"runShellScript": {
							Status:       aws.String(ssm.CommandStatusSuccess),
							ResponseCode: aws.Int64(0),
						},
					},
					describeOutput: &ssm.DescribeInstanceInformationOutput{
						InstanceInformationList: []*ssm.InstanceInformation{
							{
								InstanceId:   aws.String("instance-id-1"),
								PingStatus:   aws.String("Online"),
								PlatformType: aws.String("Linux"),
							},
							{
								InstanceId:   aws.String("instance-id-2"),
								PingStatus:   aws.String("ConnectionLost"),
								PlatformType: aws.String("Linux"),
							},
							{
								InstanceId:   aws.String("instance-id-3"),
								PingStatus:   aws.String("Online"),
								PlatformType: aws.String("Windows"),
							},
						},
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
					CommandID:     "command-id-1",
					InstanceID:    "instance-id-1",
					AccountID:     "account-id",
					Region:        "eu-central-1",
					ExitCode:      0,
					Status:        ssm.CommandStatusSuccess,
					InvocationURL: "https://eu-central-1.console.aws.amazon.com/systems-manager/run-command/command-id-1/instance-id-1",
				},
				&events.SSMRun{
					Metadata: events.Metadata{
						Type: libevent.SSMRunEvent,
						Code: libevent.SSMRunFailCode,
					},
					CommandID:  "no-command",
					InstanceID: "instance-id-2",
					AccountID:  "account-id",
					Region:     "eu-central-1",
					ExitCode:   -1,
					Status:     "SSM Agent in EC2 Instance is not connecting to SSM Service. Restart or reinstall the SSM service. See https://docs.aws.amazon.com/systems-manager/latest/userguide/ami-preinstalled-agent.html#verify-ssm-agent-status for more details.",
				},
				&events.SSMRun{
					Metadata: events.Metadata{
						Type: libevent.SSMRunEvent,
						Code: libevent.SSMRunFailCode,
					},
					CommandID:  "no-command",
					InstanceID: "instance-id-3",
					AccountID:  "account-id",
					Region:     "eu-central-1",
					ExitCode:   -1,
					Status:     "EC2 instance is running an unsupported Operating System. Only Linux is supported.",
				},
				&events.SSMRun{
					Metadata: events.Metadata{
						Type: libevent.SSMRunEvent,
						Code: libevent.SSMRunFailCode,
					},
					CommandID:  "no-command",
					InstanceID: "instance-id-4",
					AccountID:  "account-id",
					Region:     "eu-central-1",
					ExitCode:   -1,
					Status:     "EC2 Instance is not registered in SSM. Make sure that the instance has AmazonSSMManagedInstanceCore policy assigned.",
				},
			},
		},
		{
			name: "ssm with custom steps",
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
					commandInvokeOutput: map[string]*ssm.GetCommandInvocationOutput{
						"downloadContentCustom": {
							Status:       aws.String(ssm.CommandStatusSuccess),
							ResponseCode: aws.Int64(0),
						},
						"runShellScriptCustom": {
							Status:                aws.String(ssm.CommandStatusSuccess),
							ResponseCode:          aws.Int64(0),
							StandardOutputContent: aws.String("custom output"),
						},
					},
					listCommandInvocations: &ssm.ListCommandInvocationsOutput{
						CommandInvocations: []*ssm.CommandInvocation{{
							CommandPlugins: []*ssm.CommandPlugin{
								{Name: aws.String("downloadContentCustom")},
								{Name: aws.String("runShellScriptCustom")},
							},
						}},
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
					CommandID:      "command-id-1",
					InstanceID:     "instance-id-1",
					AccountID:      "account-id",
					Region:         "eu-central-1",
					ExitCode:       0,
					Status:         ssm.CommandStatusSuccess,
					StandardOutput: "custom output",
					InvocationURL:  "https://eu-central-1.console.aws.amazon.com/systems-manager/run-command/command-id-1/instance-id-1",
				},
			},
		},
		{
			name: "ssm with custom steps but without listing permissions only returns the overall result",
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
					commandInvokeOutput: map[string]*ssm.GetCommandInvocationOutput{
						"": {
							Status:       aws.String(ssm.CommandStatusSuccess),
							ResponseCode: aws.Int64(0),
						},
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
					CommandID:     "command-id-1",
					InstanceID:    "instance-id-1",
					AccountID:     "account-id",
					Region:        "eu-central-1",
					ExitCode:      0,
					Status:        ssm.CommandStatusSuccess,
					InvocationURL: "https://eu-central-1.console.aws.amazon.com/systems-manager/run-command/command-id-1/instance-id-1",
				},
			},
		},
		// todo(amk): test that incomplete commands eventually return
		// an event once completed
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			inst, err := NewSSMInstaller(tc.conf)
			require.NoError(t, err)

			err = inst.Run(ctx, tc.req)
			require.NoError(t, err)

			emitter := inst.Emitter.(*mockEmitter)
			require.ElementsMatch(t, tc.expectedEvents, emitter.events)
		})
	}
}
