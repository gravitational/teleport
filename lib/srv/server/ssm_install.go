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
	"errors"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/gravitational/teleport/api/types/events"
	apievents "github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
)

// SSMInstallerConfig represents configuration for an SSM install
// script executor.
type SSMInstallerConfig struct {
	// Emitter is an events emitter.
	Emitter apievents.Emitter
	// InstanceStates is a cache of known ec2 instances and their state.
	InstanceStates *InstanceFilterCache
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
	Instances []*ec2.Instance
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
	var ids []string
	for _, inst := range req.Instances {
		ids = append(ids, aws.StringValue(inst.InstanceId))
	}
	si.InstanceStates.SetInstances(req.AccountID, ids, InstanceStateNotStarted)

	params := make(map[string][]*string)
	for k, v := range req.Params {
		params[k] = []*string{aws.String(v)}
	}
	output, err := req.SSM.SendCommand(&ssm.SendCommandInput{
		DocumentName: aws.String(req.DocumentName),
		InstanceIds:  aws.StringSlice(ids),
		Parameters:   params,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	var g errgroup.Group
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

	cmdOut, err := req.SSM.GetCommandInvocation(&ssm.GetCommandInvocationInput{
		CommandId:  commandID,
		InstanceId: instanceID,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	status := aws.StringValue(cmdOut.Status)
	var code string
	var state InstanceInstallationState
	if status == ssm.CommandStatusSuccess {
		code = libevents.SSMRunSuccessCode
		state = InstanceStateCompleted
	} else {
		code = libevents.SSMRunFailCode
		state = InstanceStateError
	}

	event := events.SSMRun{
		Metadata: events.Metadata{
			Type: libevents.SSMRunEvent,
			Code: code,
		},
		CommandID:  aws.StringValue(commandID),
		InstanceID: aws.StringValue(instanceID),
		AccountID:  req.AccountID,
		Region:     req.Region,
		ExitCode:   aws.Int64Value(cmdOut.ResponseCode),
		Status:     aws.StringValue(cmdOut.Status),
	}

	si.InstanceStates.SetInstance(InstanceFilterKey{
		AccountID:  req.AccountID,
		InstanceID: aws.StringValue(instanceID),
	}, state)

	return trace.Wrap(si.Emitter.EmitAuditEvent(ctx, &event))
}

// InstanceFilterKey is used to key the instance filter cache
type InstanceFilterKey struct {
	AccountID, InstanceID string
}

// InstanceInstallationState represents the state that installing
// teleport is at for an instance.
type InstanceInstallationState int

const (
	InstanceStateUnknown InstanceInstallationState = iota
	// IsntanceStateNotStarted represents an instance that is known
	// but installation has not yet started.
	InstanceStateNotStarted
	// InstanceStateCompleted represents an instance that has had
	// teleport successfully installed
	InstanceStateCompleted
	// InstanceStateInstalling represents an instance where teleport
	// may still be being installed
	InstanceStateInstalling
	// IsntanceStateError represents an instance that failed to
	// execute the installation script
	InstanceStateError
)

// InstanceFilterCache keeps a cache of known EC2 nodes in the
// teleport cluster
type InstanceFilterCache struct {
	sync.RWMutex
	discoveredNodes map[InstanceFilterKey]InstanceInstallationState
}

// NewInstanceFilterCache initializes a new InstanceFilterCache with a set of existing servers.
func NewInstanceFilterCache() *InstanceFilterCache {
	cache := InstanceFilterCache{
		discoveredNodes: make(map[InstanceFilterKey]InstanceInstallationState),
		RWMutex:         sync.RWMutex{},
	}
	return &cache
}

// GetInstance retrieves an instance from the cache
func (ifc *InstanceFilterCache) GetInstance(cacheKey InstanceFilterKey) (InstanceInstallationState, bool) {
	ifc.RLock()
	defer ifc.RUnlock()
	state, ok := ifc.discoveredNodes[cacheKey]
	return state, ok
}

// SetInstance sets an instance and its state in the cache
func (ifc *InstanceFilterCache) SetInstance(cacheKey InstanceFilterKey, state InstanceInstallationState) {
	ifc.Lock()
	defer ifc.Unlock()
	ifc.discoveredNodes[cacheKey] = state
}

// SetInstances sets all the specified instances to the provided state
func (ifc *InstanceFilterCache) SetInstances(accountID string, instanceIDs []string, state InstanceInstallationState) {
	ifc.Lock()
	defer ifc.Unlock()
	for _, inst := range instanceIDs {
		ifc.discoveredNodes[InstanceFilterKey{AccountID: accountID, InstanceID: inst}] = state
	}
}
