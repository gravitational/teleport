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
	"maps"
	"slices"
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
	"github.com/gravitational/teleport/api/types/usertasks"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	libevents "github.com/gravitational/teleport/lib/events"
)

// SSMInstallerConfig represents configuration for an SSM install
// script executor.
type SSMInstallerConfig struct {
	// ReportSSMInstallationResultFunc is a func that must be called after getting the result of running the Installer script in a single instance.
	ReportSSMInstallationResultFunc func(context.Context, *SSMInstallationResult) error
	// Logger is used to log messages.
	// Optional. A logger is created if one not supplied.
	Logger *slog.Logger
}

// SSMInstallationResult contains the result of trying to install teleport
type SSMInstallationResult struct {
	// SSMRunEvent is an Audit Event that will be emitted.
	SSMRunEvent *apievents.SSMRun
	// IntegrationName is the integration name when using integration credentials.
	// Empty if using ambient credentials.
	IntegrationName string
	// DiscoveryConfigName is the DiscoveryConfig name which originated this Run Request.
	// Empty if using static matchers (coming from the `teleport.yaml`).
	DiscoveryConfigName string
	// IssueType identifies the type of issue that occurred if the installation failed.
	// These are well known identifiers that can be found at types.AutoDiscoverEC2Issue*.
	IssueType string
	// SSMDocumentName is the Amazon SSM Document Name used to install Teleport into the instance.
	SSMDocumentName string
	// InstallerScript is the Teleport Installer script name used to install Teleport into the instance.
	InstallerScript string
	// InstanceName is the Instance's name.
	// Might be empty.
	InstanceName string
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
	// IntegrationName is the integration name when using integration credentials.
	// Empty if using ambient credentials.
	IntegrationName string
	// DiscoveryConfigName is the DiscoveryConfig name which originated this Run Request.
	// Empty if using static matchers (coming from the `teleport.yaml`).
	DiscoveryConfigName string
}

// InstallerScriptName returns the Teleport Installer script name.
// Returns empty string if not defined.
func (r *SSMRunRequest) InstallerScriptName() string {
	if r == nil || r.Params == nil {
		return ""
	}

	return r.Params[ParamScriptName]
}

// CheckAndSetDefaults ensures the emitter is present and creates a default logger if one is not provided.
func (c *SSMInstallerConfig) checkAndSetDefaults() error {
	if c.ReportSSMInstallationResultFunc == nil {
		return trace.BadParameter("missing report installation result function")
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
	instances := make(map[string]string, len(req.Instances))
	for _, inst := range req.Instances {
		instances[inst.InstanceID] = inst.InstanceName
	}

	params := make(map[string][]*string)
	for k, v := range req.Params {
		params[k] = []*string{aws.String(v)}
	}

	validInstances := instances
	instancesState, err := si.describeSSMAgentState(ctx, req, instances)
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

	if len(validInstances) == 0 {
		return nil
	}

	validInstanceIDs := instanceIDsFrom(validInstances)
	output, err := req.SSM.SendCommandWithContext(ctx, &ssm.SendCommandInput{
		DocumentName: aws.String(req.DocumentName),
		InstanceIds:  aws.StringSlice(validInstanceIDs),
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
			InstanceIds:  aws.StringSlice(validInstanceIDs),
			Parameters:   params,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(10)
	for instanceID, instanceName := range validInstances {
		instanceID := instanceID
		instanceName := instanceName
		g.Go(func() error {
			return trace.Wrap(si.checkCommand(ctx, req, output.Command.CommandId, &instanceID, instanceName))
		})
	}
	return trace.Wrap(g.Wait())
}

func invalidSSMInstanceInstallationResult(req SSMRunRequest, instanceID, instanceName, status, issueType string) *SSMInstallationResult {
	return &SSMInstallationResult{
		SSMRunEvent: &apievents.SSMRun{
			Metadata: apievents.Metadata{
				Type: libevents.SSMRunEvent,
				Code: libevents.SSMRunFailCode,
			},
			CommandID:  "no-command",
			AccountID:  req.AccountID,
			Region:     req.Region,
			ExitCode:   -1,
			InstanceID: instanceID,
			Status:     status,
		},
		IntegrationName:     req.IntegrationName,
		DiscoveryConfigName: req.DiscoveryConfigName,
		IssueType:           issueType,
		SSMDocumentName:     req.DocumentName,
		InstallerScript:     req.InstallerScriptName(),
		InstanceName:        instanceName,
	}
}

func (si *SSMInstaller) emitInvalidInstanceEvents(ctx context.Context, req SSMRunRequest, instanceIDsState *instanceIDsSSMState) error {
	var errs []error
	for instanceID, instanceName := range instanceIDsState.missing {
		installationResult := invalidSSMInstanceInstallationResult(req, instanceID, instanceName,
			"EC2 Instance is not registered in SSM. Make sure that the instance has AmazonSSMManagedInstanceCore policy assigned.",
			usertasks.AutoDiscoverEC2IssueSSMInstanceNotRegistered,
		)
		if err := si.ReportSSMInstallationResultFunc(ctx, installationResult); err != nil {
			errs = append(errs, trace.Wrap(err))
		}
	}

	for instanceID, instanceName := range instanceIDsState.connectionLost {
		installationResult := invalidSSMInstanceInstallationResult(req, instanceID, instanceName,
			"SSM Agent in EC2 Instance is not connecting to SSM Service. Restart or reinstall the SSM service. See https://docs.aws.amazon.com/systems-manager/latest/userguide/ami-preinstalled-agent.html#verify-ssm-agent-status for more details.",
			usertasks.AutoDiscoverEC2IssueSSMInstanceConnectionLost,
		)
		if err := si.ReportSSMInstallationResultFunc(ctx, installationResult); err != nil {
			errs = append(errs, trace.Wrap(err))
		}
	}

	for instanceID, instanceName := range instanceIDsState.unsupportedOS {
		installationResult := invalidSSMInstanceInstallationResult(req, instanceID, instanceName,
			"EC2 instance is running an unsupported Operating System. Only Linux is supported.",
			usertasks.AutoDiscoverEC2IssueSSMInstanceUnsupportedOS,
		)
		if err := si.ReportSSMInstallationResultFunc(ctx, installationResult); err != nil {
			errs = append(errs, trace.Wrap(err))
		}
	}

	return errors.Join(errs...)
}

// instanceIDsSSMState contains a list of EC2 Instance IDs for a given state.
type instanceIDsSSMState struct {
	valid          map[string]string
	missing        map[string]string
	connectionLost map[string]string
	unsupportedOS  map[string]string
}

func instanceIDsFrom(m map[string]string) []string {
	return slices.Collect(maps.Keys(m))
}

// describeSSMAgentState returns the instanceIDsSSMState for all the instances.
func (si *SSMInstaller) describeSSMAgentState(ctx context.Context, req SSMRunRequest, allInstances map[string]string) (*instanceIDsSSMState, error) {
	ret := &instanceIDsSSMState{
		valid:          make(map[string]string),
		missing:        make(map[string]string),
		connectionLost: make(map[string]string),
		unsupportedOS:  make(map[string]string),
	}
	instanceIDs := instanceIDsFrom(allInstances)

	ssmInstancesInfo, err := req.SSM.DescribeInstanceInformationWithContext(ctx, &ssm.DescribeInstanceInformationInput{
		Filters: []*ssm.InstanceInformationStringFilter{
			{Key: aws.String(ssm.InstanceInformationFilterKeyInstanceIds), Values: aws.StringSlice(instanceIDs)},
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

	for instanceID, instanceName := range allInstances {
		instanceState, found := instanceStateByInstanceID[instanceID]
		if !found {
			ret.missing[instanceID] = instanceName
			continue
		}

		if aws.StringValue(instanceState.PingStatus) == ssm.PingStatusConnectionLost {
			ret.connectionLost[instanceID] = instanceName
			continue
		}

		if aws.StringValue(instanceState.PlatformType) != ssm.PlatformTypeLinux {
			ret.unsupportedOS[instanceID] = instanceName
			continue
		}

		ret.valid[instanceID] = instanceName
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

func (si *SSMInstaller) checkCommand(ctx context.Context, req SSMRunRequest, commandID, instanceID *string, instanceName string) error {
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

				return trace.Wrap(si.ReportSSMInstallationResultFunc(ctx, &SSMInstallationResult{
					SSMRunEvent:         invocationResultEvent,
					IntegrationName:     req.IntegrationName,
					DiscoveryConfigName: req.DiscoveryConfigName,
					IssueType:           usertasks.AutoDiscoverEC2IssueSSMScriptFailure,
					SSMDocumentName:     req.DocumentName,
					InstallerScript:     req.InstallerScriptName(),
					InstanceName:        instanceName,
				}))
			}

			return trace.Wrap(err)
		}

		// Emit an event for the first failed step or for the latest step.
		lastStep := i+1 == len(invocationSteps)
		if stepResultEvent.Metadata.Code != libevents.SSMRunSuccessCode || lastStep {
			return trace.Wrap(si.ReportSSMInstallationResultFunc(ctx, &SSMInstallationResult{
				SSMRunEvent:         stepResultEvent,
				IntegrationName:     req.IntegrationName,
				DiscoveryConfigName: req.DiscoveryConfigName,
				IssueType:           usertasks.AutoDiscoverEC2IssueSSMScriptFailure,
				SSMDocumentName:     req.DocumentName,
				InstallerScript:     req.InstallerScriptName(),
				InstanceName:        instanceName,
			}))
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
