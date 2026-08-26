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
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/usertasks"
	libevent "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/server/installer"
	"github.com/gravitational/teleport/lib/srv/server/installstatus"
)

type mockSSMClient struct {
	SSMClient
	commandOutput                    *ssm.SendCommandOutput
	waiterTimeout                    bool
	waiterMaxWaitError               error
	waitForContext                   bool
	waitForContextInstance           map[string]bool
	waiterCompletionDelay            time.Duration
	waiterStarted                    chan struct{}
	getCommandWaitForContextInstance map[string]bool
	commandInvokeOutput              map[string]*ssm.GetCommandInvocationOutput
	commandInvokeByInstance          map[string]*ssm.GetCommandInvocationOutput
	describeOutput                   *ssm.DescribeInstanceInformationOutput
	listCommandInvocations           *ssm.ListCommandInvocationsOutput
}

func TestTrimToRecentTail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *string
		maxChars int
		want     string
	}{
		{
			name:     "returns full string when below limit",
			input:    aws.String("line-a\nline-b"),
			maxChars: len([]rune("line-a\nline-b")) + 1,
			want:     "line-a\nline-b",
		},
		{
			name: "keeps tail and aligns to line boundary",
			input: aws.String(strings.Join([]string{
				"line-1",
				"line-2",
				"line-3",
				"line-4",
			}, "\n")),
			maxChars: len("e-2\nline-3\nline-4"),
			want:     "line-3\nline-4",
		},
		{
			name:     "handles multi-byte runes",
			input:    aws.String("🙂🙂🙂\nlast"),
			maxChars: 6,
			want:     "last",
		},
		{
			name:     "returns empty for nil input",
			input:    nil,
			maxChars: 10,
			want:     "",
		},
		{
			name:     "returns empty for non-positive max chars",
			input:    aws.String("line-a"),
			maxChars: 0,
			want:     "",
		},
		{
			name:     "keeps raw tail when no newline is present",
			input:    aws.String("aaaaabbbbb"),
			maxChars: 4,
			want:     "bbbb",
		},
		{
			name:     "keeps boundary-aligned tail when cut starts after newline",
			input:    aws.String("line-1\nline-2"),
			maxChars: len("line-2"),
			want:     "line-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, trimToRecentTail(tt.input, tt.maxChars))
		})
	}
}

const docWithoutSSHDConfigPathParam = "ssmdocument-without-sshdConfigPath-param"

const docWithoutEnvParam = "ssmdocument-without-env-param"

func TestGetAWSInstallTimeout(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{
			name: "unset uses default",
			want: defaultAWSInstallTimeout,
		},
		{
			name:  "valid duration",
			value: "3m45s",
			want:  3*time.Minute + 45*time.Second,
		},
		{
			name:  "invalid duration uses default",
			value: "invalid",
			want:  defaultAWSInstallTimeout,
		},
		{
			name:  "duration is clamped to minimum",
			value: "1s",
			want:  minAWSInstallTimeout,
		},
		{
			name:  "duration is clamped to maximum",
			value: "2h",
			want:  maxAWSInstallTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(awsInstallTimeoutEnvVar, tt.value)
			require.Equal(t, tt.want, getAWSInstallTimeout())
		})
	}
}

func (sm *mockSSMClient) SendCommand(_ context.Context, input *ssm.SendCommandInput, _ ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
	if _, hasExtraParam := input.Parameters["sshdConfigPath"]; hasExtraParam && aws.ToString(input.DocumentName) == docWithoutSSHDConfigPathParam {
		return nil, fmt.Errorf("InvalidParameters: document %s does not support parameters", docWithoutSSHDConfigPathParam)
	}

	if _, hasExtraParam := input.Parameters["env"]; hasExtraParam && aws.ToString(input.DocumentName) == docWithoutEnvParam {
		return nil, fmt.Errorf("InvalidParameters: document %s does not support parameters", docWithoutEnvParam)
	}

	return sm.commandOutput, nil
}

func (sm *mockSSMClient) GetCommandInvocation(ctx context.Context, input *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
	if sm.getCommandWaitForContextInstance[aws.ToString(input.InstanceId)] {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if result, found := sm.commandInvokeByInstance[aws.ToString(input.InstanceId)]; found {
		return result, nil
	}
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
	if sm.waiterStarted != nil {
		sm.waiterStarted <- struct{}{}
	}

	if sm.waiterCompletionDelay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sm.waiterCompletionDelay):
			return nil
		}
	}

	if sm.waitForContext || sm.waitForContextInstance[aws.ToString(params.InstanceId)] {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(maxWaitDur):
			if sm.waiterMaxWaitError != nil {
				return sm.waiterMaxWaitError
			}
			return trace.Errorf(waiterTimedOutErrorMessage)
		}
	}

	if sm.waiterTimeout {
		return trace.Errorf(waiterTimedOutErrorMessage)
	}

	var failureStates = []ssmtypes.CommandStatus{
		ssmtypes.CommandStatusCancelled,
		ssmtypes.CommandStatusFailed,
		ssmtypes.CommandStatusTimedOut,
		ssmtypes.CommandStatusCancelling,
	}

	if slices.Contains(failureStates, sm.commandOutput.Command.Status) {
		return trace.Errorf(waiterTransitionedToFailureErrorMessage)
	}
	return nil
}

func TestSSMInstallerHungCommandReportsPerInstanceResults(t *testing.T) {
	const installTimeout = 3*time.Minute + 45*time.Second
	t.Setenv(awsInstallTimeoutEnvVar, installTimeout.String())

	synctest.Test(t, func(t *testing.T) {
		client := &mockSSMClient{
			commandOutput: &ssm.SendCommandOutput{
				Command: &ssmtypes.Command{
					CommandId: aws.String("command-id"),
				},
			},
			waitForContextInstance: map[string]bool{
				"instance-hung": true,
			},
			commandInvokeByInstance: map[string]*ssm.GetCommandInvocationOutput{
				"instance-success": {
					Status:       ssmtypes.CommandInvocationStatusSuccess,
					ResponseCode: 0,
				},
				"instance-hung": {
					Status:       ssmtypes.CommandInvocationStatusInProgress,
					ResponseCode: -1,
				},
			},
		}

		installationResults := &mockInstallationResults{}
		inst, err := NewSSMInstaller(SSMInstallerConfig{
			ReportSSMInstallationResultFunc: installationResults.ReportInstallationResult,
			getWaiter: func(SSMClient) commandWaiter {
				return client
			},
		})
		require.NoError(t, err)

		err = inst.Run(t.Context(), SSMRunRequest{
			DocumentName: types.AWSSSMDocumentRunShellScript,
			SSM:          client,
			Instances: []EC2Instance{
				{InstanceID: "instance-success"},
				{InstanceID: "instance-hung"},
			},
		})
		require.NoError(t, err)

		require.Len(t, installationResults.installations, 2)
		resultsByInstance := make(map[string]*SSMInstallationResult, 2)
		for _, result := range installationResults.installations {
			resultsByInstance[result.SSMRunEvent.InstanceID] = result
		}
		require.Equal(t, string(ssmtypes.CommandInvocationStatusSuccess), resultsByInstance["instance-success"].SSMRunEvent.Status)
		require.Equal(t, libevent.SSMRunSuccessCode, resultsByInstance["instance-success"].SSMRunEvent.Metadata.Code)
		require.Equal(t, string(ssmtypes.CommandInvocationStatusInProgress), resultsByInstance["instance-hung"].SSMRunEvent.Status)
		require.Equal(t, libevent.SSMRunFailCode, resultsByInstance["instance-hung"].SSMRunEvent.Metadata.Code)
	})
}

func TestSSMInstallerBatchCompletesWithinSingleWaitPeriod(t *testing.T) {
	const installTimeout = 3*time.Minute + 45*time.Second
	t.Setenv(awsInstallTimeoutEnvVar, installTimeout.String())

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		instances := make([]EC2Instance, 0, awsEC2APIChunkSize)
		commandInvokeByInstance := make(map[string]*ssm.GetCommandInvocationOutput, awsEC2APIChunkSize)
		for i := range awsEC2APIChunkSize {
			instanceID := fmt.Sprintf("instance-%d", i)
			instances = append(instances, EC2Instance{InstanceID: instanceID})
			commandInvokeByInstance[instanceID] = &ssm.GetCommandInvocationOutput{
				Status:       ssmtypes.CommandInvocationStatusInProgress,
				ResponseCode: -1,
			}
		}

		client := &mockSSMClient{
			commandOutput: &ssm.SendCommandOutput{
				Command: &ssmtypes.Command{
					CommandId: aws.String("command-id"),
				},
			},
			waitForContext:          true,
			commandInvokeByInstance: commandInvokeByInstance,
		}
		installationResults := &mockInstallationResults{}
		inst, err := NewSSMInstaller(SSMInstallerConfig{
			ReportSSMInstallationResultFunc: installationResults.ReportInstallationResult,
			getWaiter: func(SSMClient) commandWaiter {
				return client
			},
		})
		require.NoError(t, err)

		err = inst.Run(t.Context(), SSMRunRequest{
			DocumentName: types.AWSSSMDocumentRunShellScript,
			SSM:          client,
			Instances:    instances,
		})
		require.NoError(t, err)
		require.Equal(t, installTimeout+awsSSMResultGracePeriod, time.Since(start))
		require.Len(t, installationResults.installations, awsEC2APIChunkSize)

		for _, result := range installationResults.installations {
			require.Equal(t, string(ssmtypes.CommandInvocationStatusInProgress), result.SSMRunEvent.Status)
			require.Equal(t, libevent.SSMRunFailCode, result.SSMRunEvent.Metadata.Code)
		}
	})
}

func TestSSMInstallerCollectionTimeoutIsInstanceLocal(t *testing.T) {
	const installTimeout = 3*time.Minute + 45*time.Second
	const blockedInstanceID = "instance-blocked"
	t.Setenv(awsInstallTimeoutEnvVar, installTimeout.String())

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		instances := make([]EC2Instance, 0, awsEC2APIChunkSize)
		commandInvokeByInstance := make(map[string]*ssm.GetCommandInvocationOutput, awsEC2APIChunkSize)
		for i := range awsEC2APIChunkSize - 1 {
			instanceID := fmt.Sprintf("instance-%d", i)
			instances = append(instances, EC2Instance{InstanceID: instanceID})
			commandInvokeByInstance[instanceID] = &ssm.GetCommandInvocationOutput{
				Status:       ssmtypes.CommandInvocationStatusSuccess,
				ResponseCode: 0,
			}
		}
		instances = append(instances, EC2Instance{InstanceID: blockedInstanceID})

		client := &mockSSMClient{
			commandOutput: &ssm.SendCommandOutput{
				Command: &ssmtypes.Command{
					CommandId: aws.String("command-id"),
				},
			},
			waitForContext: true,
			getCommandWaitForContextInstance: map[string]bool{
				blockedInstanceID: true,
			},
			commandInvokeByInstance: commandInvokeByInstance,
		}
		installationResults := &mockInstallationResults{}
		inst, err := NewSSMInstaller(SSMInstallerConfig{
			ReportSSMInstallationResultFunc: installationResults.ReportInstallationResult,
			getWaiter: func(SSMClient) commandWaiter {
				return client
			},
		})
		require.NoError(t, err)

		err = inst.Run(t.Context(), SSMRunRequest{
			DocumentName: types.AWSSSMDocumentRunShellScript,
			SSM:          client,
			Instances:    instances,
		})
		require.NoError(t, err)
		require.Equal(t, installTimeout+2*awsSSMResultGracePeriod, time.Since(start))

		require.Len(t, installationResults.installations, awsEC2APIChunkSize)
		resultsByInstance := make(map[string]*SSMInstallationResult, awsEC2APIChunkSize)
		for _, result := range installationResults.installations {
			instanceID := result.SSMRunEvent.InstanceID
			require.NotContains(t, resultsByInstance, instanceID)
			resultsByInstance[instanceID] = result
		}

		for _, instance := range instances {
			result := resultsByInstance[instance.InstanceID]
			require.NotNil(t, result)
			if instance.InstanceID == blockedInstanceID {
				require.Equal(t, libevent.SSMRunFailCode, result.SSMRunEvent.Metadata.Code)
				require.Equal(t, int64(-1), result.SSMRunEvent.ExitCode)
				require.Equal(t, usertasks.AutoDiscoverEC2IssueSSMInvocationFailure, result.IssueType)
				continue
			}

			require.Equal(t, libevent.SSMRunSuccessCode, result.SSMRunEvent.Metadata.Code)
			require.Equal(t, string(ssmtypes.CommandInvocationStatusSuccess), result.SSMRunEvent.Status)
		}
	})
}

func TestSSMInstallerReportErrorIsInstanceLocal(t *testing.T) {
	client := &mockSSMClient{
		commandOutput: &ssm.SendCommandOutput{
			Command: &ssmtypes.Command{
				CommandId: aws.String("command-id"),
			},
		},
		commandInvokeByInstance: map[string]*ssm.GetCommandInvocationOutput{
			"instance-report-error": {
				Status:       ssmtypes.CommandInvocationStatusSuccess,
				ResponseCode: 0,
			},
			"instance-success": {
				Status:       ssmtypes.CommandInvocationStatusSuccess,
				ResponseCode: 0,
			},
		},
	}

	var mu sync.Mutex
	reportedInstances := make(map[string]int, 2)
	inst, err := NewSSMInstaller(SSMInstallerConfig{
		ReportSSMInstallationResultFunc: func(_ context.Context, result *SSMInstallationResult) error {
			mu.Lock()
			defer mu.Unlock()
			reportedInstances[result.SSMRunEvent.InstanceID]++
			if result.SSMRunEvent.InstanceID == "instance-report-error" {
				return fmt.Errorf("report failed")
			}
			return nil
		},
		getWaiter: func(SSMClient) commandWaiter {
			return client
		},
	})
	require.NoError(t, err)

	err = inst.Run(t.Context(), SSMRunRequest{
		DocumentName: types.AWSSSMDocumentRunShellScript,
		SSM:          client,
		Instances: []EC2Instance{
			{InstanceID: "instance-report-error"},
			{InstanceID: "instance-success"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, map[string]int{
		"instance-report-error": 1,
		"instance-success":      1,
	}, reportedInstances)
}

func TestSSMInstallerWaitsThroughStatusHeadroom(t *testing.T) {
	const installTimeout = 3*time.Minute + 45*time.Second
	const terminalStatusDelay = installTimeout + 30*time.Second
	t.Setenv(awsInstallTimeoutEnvVar, installTimeout.String())

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		client := &mockSSMClient{
			commandOutput: &ssm.SendCommandOutput{
				Command: &ssmtypes.Command{
					CommandId: aws.String("command-id"),
				},
			},
			waiterCompletionDelay: terminalStatusDelay,
			commandInvokeByInstance: map[string]*ssm.GetCommandInvocationOutput{
				"instance-id": {
					Status:       ssmtypes.CommandInvocationStatusTimedOut,
					ResponseCode: -1,
				},
			},
		}
		installationResults := &mockInstallationResults{}
		inst, err := NewSSMInstaller(SSMInstallerConfig{
			ReportSSMInstallationResultFunc: installationResults.ReportInstallationResult,
			getWaiter: func(SSMClient) commandWaiter {
				return client
			},
		})
		require.NoError(t, err)

		err = inst.Run(t.Context(), SSMRunRequest{
			DocumentName: types.AWSSSMDocumentRunShellScript,
			SSM:          client,
			Instances: []EC2Instance{
				{InstanceID: "instance-id"},
			},
		})
		require.NoError(t, err)
		require.Equal(t, terminalStatusDelay, time.Since(start))

		require.Len(t, installationResults.installations, 1)
		require.Equal(t, string(ssmtypes.CommandInvocationStatusTimedOut), installationResults.installations[0].SSMRunEvent.Status)
		require.Equal(t, libevent.SSMRunFailCode, installationResults.installations[0].SSMRunEvent.Metadata.Code)
	})
}

func TestSSMInstallerCallerCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		client := &mockSSMClient{
			commandOutput: &ssm.SendCommandOutput{
				Command: &ssmtypes.Command{
					CommandId: aws.String("command-id"),
				},
			},
			waitForContext: true,
			waiterStarted:  make(chan struct{}, 1),
		}
		installationResults := &mockInstallationResults{}
		inst, err := NewSSMInstaller(SSMInstallerConfig{
			ReportSSMInstallationResultFunc: installationResults.ReportInstallationResult,
			getWaiter: func(SSMClient) commandWaiter {
				return client
			},
		})
		require.NoError(t, err)

		errCh := make(chan error, 1)
		go func() {
			errCh <- inst.Run(ctx, SSMRunRequest{
				DocumentName: types.AWSSSMDocumentRunShellScript,
				SSM:          client,
				Instances: []EC2Instance{
					{InstanceID: "instance-id"},
				},
			})
		}()

		<-client.waiterStarted
		cancel()
		require.ErrorIs(t, <-errCh, context.Canceled)
		require.Empty(t, installationResults.installations)
	})
}

func TestSSMInstallerHonorsTimeoutAboveLegacyWaiterLimit(t *testing.T) {
	t.Setenv(awsInstallTimeoutEnvVar, maxAWSInstallTimeout.String())

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		client := &mockSSMClient{
			commandOutput: &ssm.SendCommandOutput{
				Command: &ssmtypes.Command{
					CommandId: aws.String("command-id"),
				},
			},
			waitForContext:     true,
			waiterMaxWaitError: fmt.Errorf("request canceled while waiting: %w", context.DeadlineExceeded),
			commandInvokeByInstance: map[string]*ssm.GetCommandInvocationOutput{
				"instance-id": {
					Status:       ssmtypes.CommandInvocationStatusInProgress,
					ResponseCode: -1,
				},
			},
		}
		installationResults := &mockInstallationResults{}
		inst, err := NewSSMInstaller(SSMInstallerConfig{
			ReportSSMInstallationResultFunc: installationResults.ReportInstallationResult,
			getWaiter: func(SSMClient) commandWaiter {
				return client
			},
		})
		require.NoError(t, err)

		err = inst.Run(t.Context(), SSMRunRequest{
			DocumentName: types.AWSSSMDocumentRunShellScript,
			SSM:          client,
			Instances: []EC2Instance{
				{InstanceID: "instance-id"},
			},
		})
		require.NoError(t, err)
		require.Equal(t, maxAWSInstallTimeout+awsSSMResultGracePeriod, time.Since(start))

		require.Len(t, installationResults.installations, 1)
		require.Equal(t, string(ssmtypes.CommandInvocationStatusInProgress), installationResults.installations[0].SSMRunEvent.Status)
	})
}

type mockInstallationResults struct {
	mu            sync.Mutex
	installations []*SSMInstallationResult
}

func (me *mockInstallationResults) ReportInstallationResult(ctx context.Context, result *SSMInstallationResult) error {
	me.mu.Lock()
	defer me.mu.Unlock()
	me.installations = append(me.installations, result)
	return nil
}

func TestSSMInstaller(t *testing.T) {
	document := "ssmdocument"
	joinFailureTimeout := installer.JoinFailureTimeout.String()
	joinFailureMessage := fmt.Sprintf("node did not become ready (join cluster) within %s", joinFailureTimeout)
	joinFailureStatus := fmt.Sprintf("Teleport was installed successfully but the agent did not become ready within the configured timeout. Check standard error output for join diagnostics. (timeout: %s)", joinFailureTimeout)
	joinFailureStandardError := fmt.Sprintf("ERROR: join failure: token is expired or not found; %s", joinFailureMessage)

	for _, tc := range []struct {
		client                *mockSSMClient
		req                   SSMRunRequest
		expectedInstallations []*SSMInstallationResult
		expectedRunErrCheck   require.ErrorAssertionFunc
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
			name: "params do not include env",
			req: SSMRunRequest{
				Instances: []EC2Instance{
					{InstanceID: "instance-id-1"},
				},
				DocumentName: docWithoutEnvParam,
				Params:       map[string]string{"env": "FOO=bar BAZ=qux"},
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
			expectedRunErrCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(tt, err, "update the document")
			},
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
						Status:    ssmtypes.CommandStatusFailed,
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
					Status:         "Installation failed with exit code 1. Please check stdout and stderr and try again.",
					StandardOutput: "",
					StandardError:  "timeout error",
					InvocationURL:  "https://eu-central-1.console.aws.amazon.com/systems-manager/run-command/command-id-1/instance-id-1",
				},
				IssueType:       "ec2-ssm-script-failure",
				SSMDocumentName: "ssmdocument",
			}},
		},
		{
			name: "ssm run takes too long, and waiter times out",
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
				waiterTimeout: true,
				commandOutput: &ssm.SendCommandOutput{
					Command: &ssmtypes.Command{
						CommandId: aws.String("command-id-1"),
						Status:    ssmtypes.CommandStatusInProgress,
					},
				},
				commandInvokeOutput: map[string]*ssm.GetCommandInvocationOutput{
					"downloadContent": {
						Status:                ssmtypes.CommandInvocationStatusInProgress,
						StandardErrorContent:  aws.String("downloading..."),
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
					ExitCode:       -1,
					Status:         string(ssmtypes.CommandInvocationStatusInProgress),
					StandardOutput: "",
					StandardError:  "downloading...",
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
						Status:    ssmtypes.CommandStatusFailed,
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
					Status:         "Installation failed with exit code 1. Please check stdout and stderr and try again.",
					StandardOutput: "",
					StandardError:  "timeout error",
					InvocationURL:  "https://eu-central-1.console.aws.amazon.com/systems-manager/run-command/command-id-1/instance-id-1",
				},
				IssueType:       "ec2-ssm-script-failure",
				SSMDocumentName: "ssmdocument",
			}},
		},
		{
			name: "ssm run failed in run shell script with join failure exit code",
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
						Status:    ssmtypes.CommandStatusFailed,
					},
				},
				commandInvokeOutput: map[string]*ssm.GetCommandInvocationOutput{
					"downloadContent": {
						Status:       ssmtypes.CommandInvocationStatusSuccess,
						ResponseCode: 0,
					},
					"runShellScript": {
						Status:                ssmtypes.CommandInvocationStatusFailed,
						ResponseCode:          150,
						StandardErrorContent:  aws.String(joinFailureStandardError),
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
					ExitCode:       150,
					Status:         joinFailureStatus,
					StandardOutput: "",
					StandardError:  joinFailureStandardError,
					InvocationURL:  "https://eu-central-1.console.aws.amazon.com/systems-manager/run-command/command-id-1/instance-id-1",
				},
				IssueType:       "ec2-join-failure",
				SSMDocumentName: "ssmdocument",
			}},
		},
		{
			name: "non-failed command invocation status with join failure exit code remains script failure",
			req: SSMRunRequest{
				DocumentName: document,
				Instances: []EC2Instance{
					{InstanceID: "instance-id-1"},
				},
				Region:    "eu-central-1",
				AccountID: "account-id",
			},
			client: &mockSSMClient{
				waiterTimeout: true,
				commandOutput: &ssm.SendCommandOutput{
					Command: &ssmtypes.Command{
						CommandId: aws.String("command-id-1"),
						Status:    ssmtypes.CommandStatusInProgress,
					},
				},
				commandInvokeOutput: map[string]*ssm.GetCommandInvocationOutput{
					"downloadContent": {
						Status:                ssmtypes.CommandInvocationStatusInProgress,
						ResponseCode:          150,
						StandardErrorContent:  aws.String("still running"),
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
					ExitCode:       150,
					Status:         string(ssmtypes.CommandInvocationStatusInProgress),
					StandardOutput: "",
					StandardError:  "still running",
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
							InstanceId:      aws.String("instance-id-1"),
							PingStatus:      ssmtypes.PingStatusOnline,
							PlatformName:    aws.String("Amazon Linux"),
							PlatformType:    ssmtypes.PlatformTypeLinux,
							PlatformVersion: aws.String("2023.5.20240916"),
						},
						{
							InstanceId:      aws.String("instance-id-2"),
							PingStatus:      ssmtypes.PingStatusConnectionLost,
							PlatformName:    aws.String("Amazon Linux"),
							PlatformType:    ssmtypes.PlatformTypeLinux,
							PlatformVersion: aws.String("2023.5.20240916"),
						},
						{
							InstanceId:      aws.String("instance-id-3"),
							PingStatus:      ssmtypes.PingStatusOnline,
							PlatformName:    aws.String("Windows Server 2022 Datacenter"),
							PlatformType:    ssmtypes.PlatformTypeWindows,
							PlatformVersion: aws.String("10.0.20348"),
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
						CommandID:       "command-id-1",
						InstanceID:      "instance-id-1",
						AccountID:       "account-id",
						Region:          "eu-central-1",
						ExitCode:        0,
						Status:          string(ssmtypes.CommandInvocationStatusSuccess),
						InvocationURL:   "https://eu-central-1.console.aws.amazon.com/systems-manager/run-command/command-id-1/instance-id-1",
						PlatformName:    "Amazon Linux",
						PlatformType:    "Linux",
						PlatformVersion: "2023.5.20240916",
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
						CommandID:       "no-command",
						InstanceID:      "instance-id-2",
						AccountID:       "account-id",
						Region:          "eu-central-1",
						ExitCode:        -1,
						Status:          "SSM Agent in EC2 Instance is not connecting to SSM Service. Restart or reinstall the SSM service. See https://docs.aws.amazon.com/systems-manager/latest/userguide/ami-preinstalled-agent.html#verify-ssm-agent-status for more details.",
						PlatformName:    "Amazon Linux",
						PlatformType:    "Linux",
						PlatformVersion: "2023.5.20240916",
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
						CommandID:       "no-command",
						InstanceID:      "instance-id-3",
						AccountID:       "account-id",
						Region:          "eu-central-1",
						ExitCode:        -1,
						Status:          "EC2 instance is running an unsupported Operating System. Only Linux is supported.",
						PlatformName:    "Windows Server 2022 Datacenter",
						PlatformType:    "Windows",
						PlatformVersion: "10.0.20348",
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
			if tc.expectedRunErrCheck != nil {
				tc.expectedRunErrCheck(t, err)
			} else {
				require.NoError(t, err)
			}

			require.ElementsMatch(t, tc.expectedInstallations, installationResultsCollector.installations)
		})
	}
}

func TestClassifyEC2SSMInvocationIssueType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   ssmtypes.CommandInvocationStatus
		exitCode int64
		want     string
	}{
		{
			name:     "failed with join failure exit code maps to join failure issue",
			status:   ssmtypes.CommandInvocationStatusFailed,
			exitCode: int64(installstatus.JoinFailure),
			want:     usertasks.AutoDiscoverEC2IssueJoinFailure,
		},
		{
			name:     "failed with other exit code maps to script failure issue",
			status:   ssmtypes.CommandInvocationStatusFailed,
			exitCode: 1,
			want:     usertasks.AutoDiscoverEC2IssueSSMScriptFailure,
		},
		{
			name:     "in progress with join failure exit code stays script failure issue",
			status:   ssmtypes.CommandInvocationStatusInProgress,
			exitCode: int64(installstatus.JoinFailure),
			want:     usertasks.AutoDiscoverEC2IssueSSMScriptFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, classifyEC2SSMInvocationIssueType(tt.status, tt.exitCode))
		})
	}
}
