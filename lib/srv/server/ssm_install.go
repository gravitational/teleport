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
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	libevents "github.com/gravitational/teleport/lib/events"
)

// SSMInstallerConfig represents configuration for an SSM install
// script executor.
type SSMInstallerConfig struct {
	// Emitter is an events emitter.
	Emitter apievents.Emitter
	// Logger is used to log messages.
	// Optional. A logger is created if one not supplied.
	Logger *slog.Logger
}

// SSMInstaller handles running SSM commands that install Teleport on EC2 instances.
type SSMInstaller struct {
	SSMInstallerConfig
}

// SSMRunRequest combines parameters for running SSM commands on a set of EC2 instances.
type SSMRunRequest struct {
	// DocumentName is the name of the SSM document to run.
	DocumentName string
	// SSM is an SSM API client.
	SSM ssmiface.SSMAPI
	// Instances is the list of instances that will have the SSM
	// document executed on them.
	Instances []EC2Instance
	// Params is a list of parameters to include when executing the
	// SSM document.
	Params map[string]string
	// Region is the region instances are present in, used in audit
	// events.
	Region string
	// AccountID is the AWS account being used to execute the SSM document.
	AccountID string
	// DiscoveryConfigName is the DiscoveryConfig that originated this SSMRunRequest.
	// Empty if it was generated from a static configuration in discovery_service.
	DiscoveryConfigName string
	// IntegrationName is the integration name when using integration credentials.
	// Empty if using ambient credentials.
	IntegrationName string
}

// CheckAndSetDefaults ensures the emitter is present and creates a default logger if one is not provided.
func (c *SSMInstallerConfig) checkAndSetDefaults() error {
	if c.Emitter == nil {
		return trace.BadParameter("missing audit event emitter")
	}

	if c.Logger == nil {
		c.Logger = slog.Default().With(teleport.ComponentKey, "ssminstaller")
	}

	return nil
}

// NewSSMInstaller returns a new instance of the SSM installer that installs Teleport on EC2 instances.
func NewSSMInstaller(cfg SSMInstallerConfig) (*SSMInstaller, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &SSMInstaller{
		SSMInstallerConfig: cfg,
	}, nil
}

// Run executes the SSM document and then blocks until the command has completed.
func (si *SSMInstaller) Run(ctx context.Context, req SSMRunRequest) error {
	ids := make([]string, 0, len(req.Instances))
	for _, inst := range req.Instances {
		ids = append(ids, inst.InstanceID)
	}

	params := make(map[string][]*string)
	for k, v := range req.Params {
		params[k] = []*string{aws.String(v)}
	}

	validInstances := ids
	instancesState, err := si.describeSSMAgentState(ctx, req, ids)
	switch {
	case trace.IsAccessDenied(err):
		// describeSSMAgentState uses `ssm:DescribeInstanceInformation` to gather all the instances information.
		// Previous Docs versions (pre-v16) did not ask for that permission.
		// If the IAM role does not have access to that action, an Access Denied is returned here.
		// The process continues but the user is warned that they should add that permission to get better diagnostics.
		si.Logger.WarnContext(ctx,
			"Add ssm:DescribeInstanceInformation action to IAM Role to improve diagnostics of EC2 Teleport installation failures",
			"error", err)

	case err != nil:
		return trace.Wrap(err)

	default:
		if err := si.emitInvalidInstanceEvents(ctx, req, instancesState); err != nil {
			si.Logger.ErrorContext(ctx,
				"Failed to emit invalid instances",
				"instances", instancesState,
				"error", err)
		}
		validInstances = instancesState.valid
	}

	output, err := req.SSM.SendCommandWithContext(ctx, &ssm.SendCommandInput{
		DocumentName: aws.String(req.DocumentName),
		InstanceIds:  aws.StringSlice(validInstances),
		Parameters:   params,
	})
	if err != nil {
		invalidParamErrorMessage := fmt.Sprintf("InvalidParameters: document %s does not support parameters", req.DocumentName)
		_, hasSSHDConfigParam := params[ParamSSHDConfigPath]
		if !strings.Contains(err.Error(), invalidParamErrorMessage) || !hasSSHDConfigParam {
			return trace.Wrap(err)
		}

		// This might happen when teleport sends Parameters that are not part of the Document.
		// One example is when it uses the default SSM Document awslib.EC2DiscoverySSMDocument
		// and Parameters include "sshdConfigPath" (only sent when installTeleport=false).
		//
		// As a best effort, we try to call ssm.SendCommand again but this time without the "sshdConfigPath" param
		// We must not remove the Param "sshdConfigPath" beforehand because customers might be using custom SSM Documents for ec2 auto discovery.
		delete(params, ParamSSHDConfigPath)
		output, err = req.SSM.SendCommandWithContext(ctx, &ssm.SendCommandInput{
			DocumentName: aws.String(req.DocumentName),
			InstanceIds:  aws.StringSlice(validInstances),
			Parameters:   params,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(10)
	for _, inst := range validInstances {
		inst := inst
		g.Go(func() error {
			return trace.Wrap(si.checkCommand(ctx, req, output.Command.CommandId, &inst))
		})
	}
	return trace.Wrap(g.Wait())
}

func invalidSSMInstanceEvent(accountID, region, instanceID, status string) apievents.SSMRun {
	return apievents.SSMRun{
		Metadata: apievents.Metadata{
			Type: libevents.SSMRunEvent,
			Code: libevents.SSMRunFailCode,
		},
		CommandID:  "no-command",
		AccountID:  accountID,
		Region:     region,
		ExitCode:   -1,
		InstanceID: instanceID,
		Status:     status,
	}
}

func (si *SSMInstaller) emitInvalidInstanceEvents(ctx context.Context, req SSMRunRequest, instanceIDsState *instanceIDsSSMState) error {
	var errs []error
	for _, instanceID := range instanceIDsState.missing {
		event := invalidSSMInstanceEvent(req.AccountID, req.Region, instanceID,
			"EC2 Instance is not registered in SSM. Make sure that the instance has AmazonSSMManagedInstanceCore policy assigned.",
		)
		if err := si.Emitter.EmitAuditEvent(ctx, &event); err != nil {
			errs = append(errs, trace.Wrap(err))
		}
	}

	for _, instanceID := range instanceIDsState.connectionLost {
		event := invalidSSMInstanceEvent(req.AccountID, req.Region, instanceID,
			"SSM Agent in EC2 Instance is not connecting to SSM Service. Restart or reinstall the SSM service. See https://docs.aws.amazon.com/systems-manager/latest/userguide/ami-preinstalled-agent.html#verify-ssm-agent-status for more details.",
		)
		if err := si.Emitter.EmitAuditEvent(ctx, &event); err != nil {
			errs = append(errs, trace.Wrap(err))
		}
	}

	for _, instanceID := range instanceIDsState.unsupportedOS {
		event := invalidSSMInstanceEvent(req.AccountID, req.Region, instanceID,
			"EC2 instance is running an unsupported Operating System. Only Linux is supported.",
		)
		if err := si.Emitter.EmitAuditEvent(ctx, &event); err != nil {
			errs = append(errs, trace.Wrap(err))
		}
	}

	return errors.Join(errs...)
}

// instanceIDsSSMState contains a list of EC2 Instance IDs for a given state.
type instanceIDsSSMState struct {
	valid          []string
	missing        []string
	connectionLost []string
	unsupportedOS  []string
}

// describeSSMAgentState returns the instanceIDsSSMState for all the instances.
func (si *SSMInstaller) describeSSMAgentState(ctx context.Context, req SSMRunRequest, allInstanceIDs []string) (*instanceIDsSSMState, error) {
	ret := &instanceIDsSSMState{}

	ssmInstancesInfo, err := req.SSM.DescribeInstanceInformationWithContext(ctx, &ssm.DescribeInstanceInformationInput{
		Filters: []*ssm.InstanceInformationStringFilter{
			{Key: aws.String(ssm.InstanceInformationFilterKeyInstanceIds), Values: aws.StringSlice(allInstanceIDs)},
		},
		MaxResults: aws.Int64(awsEC2APIChunkSize),
	})
	if err != nil {
		return nil, trace.Wrap(awslib.ConvertRequestFailureError(err))
	}

	instanceStateByInstanceID := make(map[string]*ssm.InstanceInformation, len(ssmInstancesInfo.InstanceInformationList))
	for _, instanceState := range ssmInstancesInfo.InstanceInformationList {
		// instanceState.InstanceId always has the InstanceID value according to AWS Docs.
		instanceStateByInstanceID[aws.StringValue(instanceState.InstanceId)] = instanceState
	}

	for _, instanceID := range allInstanceIDs {
		instanceState, found := instanceStateByInstanceID[instanceID]
		if !found {
			ret.missing = append(ret.missing, instanceID)
			continue
		}

		if aws.StringValue(instanceState.PingStatus) == ssm.PingStatusConnectionLost {
			ret.connectionLost = append(ret.connectionLost, instanceID)
			continue
		}

		if aws.StringValue(instanceState.PlatformType) != ssm.PlatformTypeLinux {
			ret.unsupportedOS = append(ret.unsupportedOS, instanceID)
			continue
		}

		ret.valid = append(ret.valid, instanceID)
	}

	return ret, nil
}

// skipAWSWaitErr is used to ignore the error returned from
// WaitUntilCommandExecutedWithContext if it is a resource not ready
// code as this can represent one of several different errors which
// are handled by checking the command invocation after calling this
// to get more information about the error.
func skipAWSWaitErr(err error) error {
	var aErr awserr.Error
	if errors.As(err, &aErr) && aErr.Code() == request.WaiterResourceNotReadyErrorCode {
		return nil
	}
	return trace.Wrap(err)
}

func (si *SSMInstaller) checkCommand(ctx context.Context, req SSMRunRequest, commandID, instanceID *string) error {
	err := req.SSM.WaitUntilCommandExecutedWithContext(ctx, &ssm.GetCommandInvocationInput{
		CommandId:  commandID,
		InstanceId: instanceID,
	})

	if err := skipAWSWaitErr(err); err != nil {
		return trace.Wrap(err)
	}

	invocationSteps, err := si.getInvocationSteps(ctx, req, commandID, instanceID)
	switch {
	case trace.IsAccessDenied(err):
		// getInvocationSteps uses `ssm:ListCommandInvocations` to gather all the executed steps.
		// Using `ssm:ListCommandInvocations` is not always possible because previous Docs versions (pre-v16) did not ask for that permission.
		// If the IAM role does not have access to that action, an Access Denied is returned here.
		// The process continues but the user is warned that they should add that permission to get better diagnostics.
		si.Logger.WarnContext(ctx,
			"Add ssm:ListCommandInvocations action to IAM Role to improve diagnostics of EC2 Teleport installation failures",
			"error", err)

		invocationSteps = awslib.EC2DiscoverySSMDocumentSteps

	case err != nil:
		return trace.Wrap(err)
	}

	for i, step := range invocationSteps {
		stepResultEvent, err := si.getCommandStepStatusEvent(ctx, step, req, commandID, instanceID)
		if err != nil {
			var invalidPluginNameErr *ssm.InvalidPluginName
			if errors.As(err, &invalidPluginNameErr) {
				// If using a custom SSM Document and the client does not have access to ssm:ListCommandInvocations
				// the list of invocationSteps (ie plugin name) might be wrong.
				// If that's the case, emit an event with the overall invocation result (ignoring specific steps' stdout and stderr).
				invocationResultEvent, err := si.getCommandStepStatusEvent(ctx, "" /*no step*/, req, commandID, instanceID)
				if err != nil {
					return trace.Wrap(err)
				}

				return trace.Wrap(si.Emitter.EmitAuditEvent(ctx, invocationResultEvent))
			}

			return trace.Wrap(err)
		}

		// Emit an event for the first failed step or for the latest step.
		lastStep := i+1 == len(invocationSteps)
		if stepResultEvent.Metadata.Code != libevents.SSMRunSuccessCode || lastStep {
			return trace.Wrap(si.Emitter.EmitAuditEvent(ctx, stepResultEvent))
		}
	}

	return nil
}

func (si *SSMInstaller) getInvocationSteps(ctx context.Context, req SSMRunRequest, commandID, instanceID *string) ([]string, error) {
	// ssm:ListCommandInvocations is used to list the actual steps because users might be using a custom SSM Document.
	listCommandInvocationResp, err := req.SSM.ListCommandInvocationsWithContext(ctx, &ssm.ListCommandInvocationsInput{
		CommandId:  commandID,
		InstanceId: instanceID,
		Details:    aws.Bool(true),
	})
	if err != nil {
		return nil, trace.Wrap(awslib.ConvertRequestFailureError(err))
	}

	// We only expect a single invocation because we are sending both the CommandID and the InstanceID.
	// This call happens after WaitUntilCommandExecuted, so there's no reason for this to ever return 0 elements.
	if len(listCommandInvocationResp.CommandInvocations) == 0 {
		si.Logger.WarnContext(ctx,
			"No command invocation was found.",
			"command_id", aws.StringValue(commandID),
			"instance_id", aws.StringValue(instanceID),
		)
		return nil, trace.BadParameter("no command invocation was found")
	}
	commandInvocation := listCommandInvocationResp.CommandInvocations[0]

	documentSteps := make([]string, 0, len(commandInvocation.CommandPlugins))
	for _, step := range commandInvocation.CommandPlugins {
		documentSteps = append(documentSteps, aws.StringValue(step.Name))
	}
	return documentSteps, nil
}

func (si *SSMInstaller) getCommandStepStatusEvent(ctx context.Context, step string, req SSMRunRequest, commandID, instanceID *string) (*apievents.SSMRun, error) {
	getCommandInvocationReq := &ssm.GetCommandInvocationInput{
		CommandId:  commandID,
		InstanceId: instanceID,
	}
	if step != "" {
		getCommandInvocationReq.PluginName = aws.String(step)
	}
	stepResult, err := req.SSM.GetCommandInvocationWithContext(ctx, getCommandInvocationReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	status := aws.StringValue(stepResult.Status)
	exitCode := aws.Int64Value(stepResult.ResponseCode)

	eventCode := libevents.SSMRunSuccessCode
	if status != ssm.CommandStatusSuccess {
		eventCode = libevents.SSMRunFailCode
		if exitCode == 0 {
			exitCode = -1
		}
	}

	// Format for invocation url:
	// https://<region>.console.aws.amazon.com/systems-manager/run-command/<command-id>/<instance-id>
	// Example:
	// https://eu-west-2.console.aws.amazon.com/systems-manager/run-command/3cb11aaa-11aa-1111-aaaa-2188108225de/i-0775091aa11111111
	invocationURL := fmt.Sprintf("https://%s.console.aws.amazon.com/systems-manager/run-command/%s/%s",
		req.Region, aws.StringValue(commandID), aws.StringValue(instanceID),
	)

	return &apievents.SSMRun{
		Metadata: apievents.Metadata{
			Type: libevents.SSMRunEvent,
			Code: eventCode,
		},
		CommandID:      aws.StringValue(commandID),
		InstanceID:     aws.StringValue(instanceID),
		AccountID:      req.AccountID,
		Region:         req.Region,
		ExitCode:       exitCode,
		Status:         status,
		StandardOutput: aws.StringValue(stepResult.StandardOutputContent),
		StandardError:  aws.StringValue(stepResult.StandardErrorContent),
		InvocationURL:  invocationURL,
	}, nil
}
