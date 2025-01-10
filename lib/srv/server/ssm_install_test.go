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
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/events"
	libevent "github.com/gravitational/teleport/lib/events"
)

type mockSSMClient struct {
	SSMClient
	commandOutput          *ssm.SendCommandOutput
	commandInvokeOutput    map[string]*ssm.GetCommandInvocationOutput
	describeOutput         *ssm.DescribeInstanceInformationOutput
	listCommandInvocations *ssm.ListCommandInvocationsOutput
}

const docWithoutSSHDConfigPathParam = "ssmdocument-without-sshdConfigPath-param"

func (sm *mockSSMClient) SendCommand(_ context.Context, input *ssm.SendCommandInput, _ ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
	if _, hasExtraParam := input.Parameters["sshdConfigPath"]; hasExtraParam && aws.ToString(input.DocumentName) == docWithoutSSHDConfigPathParam {
		return nil, fmt.Errorf("InvalidParameters: document %s does not support parameters", docWithoutSSHDConfigPathParam)
	}
	return sm.commandOutput, nil
}

func (sm *mockSSMClient) GetCommandInvocation(_ context.Context, input *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
	if stepResult, found := sm.commandInvokeOutput[aws.ToString(input.PluginName)]; found {
		return stepResult, nil
	}
	return nil, &ssmtypes.InvalidPluginName{}
}

func (sm *mockSSMClient) DescribeInstanceInformation(_ context.Context, input *ssm.DescribeInstanceInformationInput, _ ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error) {
	if sm.describeOutput == nil {
		return nil, trace.AccessDenied("")
	}
	return sm.describeOutput, nil
}

func (sm *mockSSMClient) ListCommandInvocations(_ context.Context, input *ssm.ListCommandInvocationsInput, _ ...func(*ssm.Options)) (*ssm.ListCommandInvocationsOutput, error) {
	if sm.listCommandInvocations == nil {
		return nil, trace.AccessDenied("")
	}
	return sm.listCommandInvocations, nil
}

func (sm *mockSSMClient) Wait(ctx context.Context, params *ssm.GetCommandInvocationInput, maxWaitDur time.Duration, optFns ...func(*ssm.CommandExecutedWaiterOptions)) error {
	if sm.commandOutput.Command.Status == ssmtypes.CommandStatusFailed {
		return trace.Errorf(waiterTimedOutErrorMessage)
	}
	return nil
}

type mockInstallationResults struct {
	installations []*SSMInstallationResult
}

func (me *mockInstallationResults) ReportInstallationResult(ctx context.Context, result *SSMInstallationResult) error {
	me.installations = append(me.installations, result)
	return nil
}

func TestSSMInstaller(t *testing.T) {
	document := "ssmdocument"

	for _, tc := range []struct {
		client                *mockSSMClient
		req                   SSMRunRequest
		expectedInstallations []*SSMInstallationResult
		name                  string
	}{
		{
			name: "ssm run was successful",
			req: SSMRunRequest{
				Instances: []EC2Instance{
					{InstanceID: "instance-id-1", InstanceName: "my-instance-name"},
				},
				DocumentName:        document,
				Params:              map[string]string{"token": "abcdefg"},
				IntegrationName:     "aws-integration",
				DiscoveryConfigName: "dc001",
				Region:              "eu-central-1",
				AccountID:           "account-id",
			},
			client: &mockSSMClient{
				commandOutput: &ssm.SendCommandOutput{
					Command: &ssmtypes.Command{
						CommandId: aws.String("command-id-1"),
					},
				},
				commandInvokeOutput: map[string]*ssm.GetCommandInvocationOutput{
					"downloadContent": {
						Status:       ssmtypes.CommandInvocationStatusSuccess,
						ResponseCode: 0,
					},
					"runShellScript": {
						Status:       ssmtypes.CommandInvocationStatusSuccess,
						ResponseCode: 0,
					},
				},
			},
			expectedInstallations: []*SSMInstallationResult{{
				IntegrationName:     "aws-integration",
				DiscoveryConfigName: "dc001",
				SSMRunEvent: &events.SSMRun{
					Metadata: events.Metadata{
						Type: libevent.SSMRunEvent,
						Code: libevent.SSMRunSuccessCode,
					},
					CommandID:     "command-id-1",
					InstanceID:    "instance-id-1",
					AccountID:     "account-id",
					Region:        "eu-central-1",
					ExitCode:      0,
					Status:        string(ssmtypes.CommandInvocationStatusSuccess),
					InvocationURL: "https://eu-central-1.console.aws.amazon.com/systems-manager/run-command/command-id-1/instance-id-1",
				},
				IssueType:       "ec2-ssm-script-failure",
				SSMDocumentName: "ssmdocument",
				InstanceName:    "my-instance-name",
			}},
		},
		{
			name: "params include sshdConfigPath",
			req: SSMRunRequest{
				Instances: []EC2Instance{
					{InstanceID: "instance-id-1"},
				},
				DocumentName: docWithoutSSHDConfigPathParam,
				Params:       map[string]string{"sshdConfigPath": "abcdefg"},
				Region:       "eu-central-1",
				AccountID:    "account-id",
			},
			client: &mockSSMClient{
				commandOutput: &ssm.SendCommandOutput{
					Command: &ssmtypes.Command{
						CommandId: aws.String("command-id-1"),
					},
				},
				commandInvokeOutput: map[string]*ssm.GetCommandInvocationOutput{
					"downloadContent": {
						Status:       ssmtypes.CommandInvocationStatusSuccess,
						ResponseCode: 0,
					},
					"runShellScript": {
						Status:       ssmtypes.CommandInvocationStatusSuccess,
						ResponseCode: 0,
					},
				},
			},
			expectedInstallations: []*SSMInstallationResult{{
				SSMRunEvent: &events.SSMRun{
					Metadata: events.Metadata{
						Type: libevent.SSMRunEvent,
						Code: libevent.SSMRunSuccessCode,
					},
					CommandID:     "command-id-1",
					InstanceID:    "instance-id-1",
					AccountID:     "account-id",
					Region:        "eu-central-1",
					ExitCode:      0,
					Status:        string(ssmtypes.CommandInvocationStatusSuccess),
					InvocationURL: "https://eu-central-1.console.aws.amazon.com/systems-manager/run-command/command-id-1/instance-id-1",
				},
				IssueType:       "ec2-ssm-script-failure",
				SSMDocumentName: "ssmdocument-without-sshdConfigPath-param",
			}},
		},
		{
			name: "ssm run failed in download content",
			req: SSMRunRequest{
				DocumentName: document,
				Instances: []EC2Instance{
					{InstanceID: "instance-id-1"},
				},
				IntegrationName: "aws-1",
				Params:          map[string]string{"token": "abcdefg"},
				Region:          "eu-central-1",
				AccountID:       "account-id",
			},
			client: &mockSSMClient{
				commandOutput: &ssm.SendCommandOutput{
					Command: &ssmtypes.Command{
						CommandId: aws.String("command-id-1"),
					},
				},
				commandInvokeOutput: map[string]*ssm.GetCommandInvocationOutput{
					"downloadContent": {
						Status:                ssmtypes.CommandInvocationStatusFailed,
						ResponseCode:          1,
						StandardErrorContent:  aws.String("timeout error"),
						StandardOutputContent: aws.String(""),
					},
				},
			},
			expectedInstallations: []*SSMInstallationResult{{
				IntegrationName: "aws-1",
				SSMRunEvent: &events.SSMRun{
					Metadata: events.Metadata{
						Type: libevent.SSMRunEvent,
						Code: libevent.SSMRunFailCode,
					},
					CommandID:      "command-id-1",
					InstanceID:     "instance-id-1",
					AccountID:      "account-id",
					Region:         "eu-central-1",
					ExitCode:       1,
					Status:         string(ssmtypes.CommandInvocationStatusFailed),
					StandardOutput: "",
					StandardError:  "timeout error",
					InvocationURL:  "https://eu-central-1.console.aws.amazon.com/systems-manager/run-command/command-id-1/instance-id-1",
				},
				IssueType:       "ec2-ssm-script-failure",
				SSMDocumentName: "ssmdocument",
			}},
		},
		{
			name: "ssm run failed in run shell script",
			req: SSMRunRequest{
				DocumentName: document,
				Instances: []EC2Instance{
					{InstanceID: "instance-id-1"},
				},
				Params:    map[string]string{"token": "abcdefg"},
				Region:    "eu-central-1",
				AccountID: "account-id",
			},
			client: &mockSSMClient{
				commandOutput: &ssm.SendCommandOutput{
					Command: &ssmtypes.Command{
						CommandId: aws.String("command-id-1"),
					},
				},
				commandInvokeOutput: map[string]*ssm.GetCommandInvocationOutput{
					"downloadContent": {
						Status:                ssmtypes.CommandInvocationStatusSuccess,
						ResponseCode:          0,
						StandardErrorContent:  aws.String("no error"),
						StandardOutputContent: aws.String(""),
					},
					"runShellScript": {
						Status:                ssmtypes.CommandInvocationStatusFailed,
						ResponseCode:          1,
						StandardErrorContent:  aws.String("timeout error"),
						StandardOutputContent: aws.String(""),
					},
				},
			},
			expectedInstallations: []*SSMInstallationResult{{
				SSMRunEvent: &events.SSMRun{
					Metadata: events.Metadata{
						Type: libevent.SSMRunEvent,
						Code: libevent.SSMRunFailCode,
					},
					CommandID:      "command-id-1",
					InstanceID:     "instance-id-1",
					AccountID:      "account-id",
					Region:         "eu-central-1",
					ExitCode:       1,
					Status:         string(ssmtypes.CommandInvocationStatusFailed),
					StandardOutput: "",
					StandardError:  "timeout error",
					InvocationURL:  "https://eu-central-1.console.aws.amazon.com/systems-manager/run-command/command-id-1/instance-id-1",
				},
				IssueType:       "ec2-ssm-script-failure",
				SSMDocumentName: "ssmdocument",
			}},
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
				Region:       "eu-central-1",
				AccountID:    "account-id",
			},
			client: &mockSSMClient{
				commandOutput: &ssm.SendCommandOutput{
					Command: &ssmtypes.Command{
						CommandId: aws.String("command-id-1"),
					},
				},
				commandInvokeOutput: map[string]*ssm.GetCommandInvocationOutput{
					"downloadContent": {
						Status:       ssmtypes.CommandInvocationStatusSuccess,
						ResponseCode: 0,
					},
					"runShellScript": {
						Status:       ssmtypes.CommandInvocationStatusSuccess,
						ResponseCode: 0,
					},
				},
				describeOutput: &ssm.DescribeInstanceInformationOutput{
					InstanceInformationList: []ssmtypes.InstanceInformation{
						{
							InstanceId:   aws.String("instance-id-1"),
							PingStatus:   ssmtypes.PingStatusOnline,
							PlatformType: ssmtypes.PlatformTypeLinux,
						},
						{
							InstanceId:   aws.String("instance-id-2"),
							PingStatus:   ssmtypes.PingStatusConnectionLost,
							PlatformType: ssmtypes.PlatformTypeLinux,
						},
						{
							InstanceId:   aws.String("instance-id-3"),
							PingStatus:   ssmtypes.PingStatusOnline,
							PlatformType: ssmtypes.PlatformTypeWindows,
						},
					},
				},
			},
			expectedInstallations: []*SSMInstallationResult{
				{
					SSMRunEvent: &events.SSMRun{
						Metadata: events.Metadata{
							Type: libevent.SSMRunEvent,
							Code: libevent.SSMRunSuccessCode,
						},
						CommandID:     "command-id-1",
						InstanceID:    "instance-id-1",
						AccountID:     "account-id",
						Region:        "eu-central-1",
						ExitCode:      0,
						Status:        string(ssmtypes.CommandInvocationStatusSuccess),
						InvocationURL: "https://eu-central-1.console.aws.amazon.com/systems-manager/run-command/command-id-1/instance-id-1",
					},
					IssueType:       "ec2-ssm-script-failure",
					SSMDocumentName: "ssmdocument",
				},
				{
					SSMRunEvent: &events.SSMRun{
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
					IssueType:       "ec2-ssm-agent-connection-lost",
					SSMDocumentName: "ssmdocument",
				},
				{
					SSMRunEvent: &events.SSMRun{
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
					IssueType:       "ec2-ssm-unsupported-os",
					SSMDocumentName: "ssmdocument",
				},
				{
					SSMRunEvent: &events.SSMRun{
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
					IssueType:       "ec2-ssm-agent-not-registered",
					SSMDocumentName: "ssmdocument",
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
				Region:       "eu-central-1",
				AccountID:    "account-id",
			},
			client: &mockSSMClient{
				commandOutput: &ssm.SendCommandOutput{
					Command: &ssmtypes.Command{
						CommandId: aws.String("command-id-1"),
					},
				},
				commandInvokeOutput: map[string]*ssm.GetCommandInvocationOutput{
					"downloadContentCustom": {
						Status:       ssmtypes.CommandInvocationStatusSuccess,
						ResponseCode: 0,
					},
					"runShellScriptCustom": {
						Status:                ssmtypes.CommandInvocationStatusSuccess,
						ResponseCode:          0,
						StandardOutputContent: aws.String("custom output"),
					},
				},
				listCommandInvocations: &ssm.ListCommandInvocationsOutput{
					CommandInvocations: []ssmtypes.CommandInvocation{{
						CommandPlugins: []ssmtypes.CommandPlugin{
							{Name: aws.String("downloadContentCustom")},
							{Name: aws.String("runShellScriptCustom")},
						},
					}},
				},
			},
			expectedInstallations: []*SSMInstallationResult{{
				SSMRunEvent: &events.SSMRun{
					Metadata: events.Metadata{
						Type: libevent.SSMRunEvent,
						Code: libevent.SSMRunSuccessCode,
					},
					CommandID:      "command-id-1",
					InstanceID:     "instance-id-1",
					AccountID:      "account-id",
					Region:         "eu-central-1",
					ExitCode:       0,
					Status:         string(ssmtypes.CommandInvocationStatusSuccess),
					StandardOutput: "custom output",
					InvocationURL:  "https://eu-central-1.console.aws.amazon.com/systems-manager/run-command/command-id-1/instance-id-1",
				},
				IssueType:       "ec2-ssm-script-failure",
				SSMDocumentName: "ssmdocument",
			}},
		},
		{
			name: "ssm with custom steps but without listing permissions only returns the overall result",
			req: SSMRunRequest{
				Instances: []EC2Instance{
					{InstanceID: "instance-id-1"},
				},
				DocumentName: document,
				Params:       map[string]string{"token": "abcdefg"},
				Region:       "eu-central-1",
				AccountID:    "account-id",
			},
			client: &mockSSMClient{
				commandOutput: &ssm.SendCommandOutput{
					Command: &ssmtypes.Command{
						CommandId: aws.String("command-id-1"),
					},
				},
				commandInvokeOutput: map[string]*ssm.GetCommandInvocationOutput{
					"": {
						Status:       ssmtypes.CommandInvocationStatusSuccess,
						ResponseCode: 0,
					},
				},
			},
			expectedInstallations: []*SSMInstallationResult{{
				SSMRunEvent: &events.SSMRun{
					Metadata: events.Metadata{
						Type: libevent.SSMRunEvent,
						Code: libevent.SSMRunSuccessCode,
					},
					CommandID:     "command-id-1",
					InstanceID:    "instance-id-1",
					AccountID:     "account-id",
					Region:        "eu-central-1",
					ExitCode:      0,
					Status:        string(ssmtypes.CommandInvocationStatusSuccess),
					InvocationURL: "https://eu-central-1.console.aws.amazon.com/systems-manager/run-command/command-id-1/instance-id-1",
				},
				IssueType:       "ec2-ssm-script-failure",
				SSMDocumentName: "ssmdocument",
			}},
		},
		// todo(amk): test that incomplete commands eventually return
		// an event once completed
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			tc.req.SSM = tc.client
			installationResultsCollector := &mockInstallationResults{}
			inst, err := NewSSMInstaller(SSMInstallerConfig{
				ReportSSMInstallationResultFunc: installationResultsCollector.ReportInstallationResult,
				getWaiter:                       func(s SSMClient) commandWaiter { return tc.client },
			})
			require.NoError(t, err)

			err = inst.Run(ctx, tc.req)
			require.NoError(t, err)

			require.ElementsMatch(t, tc.expectedInstallations, installationResultsCollector.installations)
		})
	}
}
