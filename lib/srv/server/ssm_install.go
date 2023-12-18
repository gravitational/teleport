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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	apievents "github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
)

// SSMInstallerConfig represents configuration for an SSM install
// script executor.
type SSMInstallerConfig struct {
	// Emitter is an events emitter.
	Emitter apievents.Emitter
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
}

// NewSSMInstaller returns a new instance of the SSM installer that installs Teleport on EC2 instances.
func NewSSMInstaller(cfg SSMInstallerConfig) *SSMInstaller {
	return &SSMInstaller{
		SSMInstallerConfig: cfg,
	}
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

	output, err := req.SSM.SendCommandWithContext(ctx, &ssm.SendCommandInput{
		DocumentName: aws.String(req.DocumentName),
		InstanceIds:  aws.StringSlice(ids),
		Parameters:   params,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(10)
	for _, inst := range ids {
		inst := inst
		g.Go(func() error {
			return trace.Wrap(si.checkCommand(ctx, req, output.Command.CommandId, &inst))
		})
	}
	return trace.Wrap(g.Wait())
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

	cmdOut, err := req.SSM.GetCommandInvocationWithContext(ctx, &ssm.GetCommandInvocationInput{
		CommandId:  commandID,
		InstanceId: instanceID,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	status := aws.StringValue(cmdOut.Status)

	code := libevents.SSMRunFailCode
	if status == ssm.CommandStatusSuccess {
		code = libevents.SSMRunSuccessCode
	}

	exitCode := aws.Int64Value(cmdOut.ResponseCode)
	if exitCode == 0 && code == libevents.SSMRunFailCode {
		exitCode = -1
	}
	event := apievents.SSMRun{
		Metadata: apievents.Metadata{
			Type: libevents.SSMRunEvent,
			Code: code,
		},
		CommandID:  aws.StringValue(commandID),
		InstanceID: aws.StringValue(instanceID),
		AccountID:  req.AccountID,
		Region:     req.Region,
		ExitCode:   exitCode,
		Status:     aws.StringValue(cmdOut.Status),
	}

	return trace.Wrap(si.Emitter.EmitAuditEvent(ctx, &event))
}
