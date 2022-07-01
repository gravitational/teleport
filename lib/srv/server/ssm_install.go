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
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/gravitational/teleport/api/types/events"
	libevent "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/trace"
)

type SSMInstaller struct {
	instances    []*string
	SSM          ssmiface.SSMAPI
	recheckTimer *time.Ticker
	params       map[string][]*string
	emitter      events.Emitter
	ctx          context.Context
	cancelfn     context.CancelFunc
	region       string
	accountID    string
}

// SSMInstallerConfig represents configuration for an SSM install
// script executor.
type SSMInstallerConfig struct {
	// SSM is an SSM API client.
	SSM ssmiface.SSMAPI
	// Instances is the list of instances that will have the SSM
	// document executed on them.
	Instances []*ec2.Instance
	// Params is a list of parameters to include when executing the
	// SSM document.
	Params map[string]string
	// Emitter is an events emitter.
	Emitter events.Emitter
	Ctx     context.Context
	// Region is the region instances are present in, used in audit
	// events.
	Region string
	// AccountID is the AWS account being used to execute the SSM document.
	AccountID string
}

func NewSSMInstaller(cfg SSMInstallerConfig) *SSMInstaller {
	var ids []*string

	for _, inst := range cfg.Instances {
		ids = append(ids, inst.InstanceId)
	}
	ssmParams := make(map[string][]*string)

	for key, val := range cfg.Params {
		ssmParams[key] = []*string{aws.String(val)}
	}

	return &SSMInstaller{
		instances:    ids,
		SSM:          cfg.SSM,
		recheckTimer: time.NewTicker(time.Second * 30),
		params:       ssmParams,
		emitter:      cfg.Emitter,
		ctx:          cfg.Ctx,
		region:       cfg.Region,
		accountID:    cfg.AccountID,
	}
}

var ErrCommandInProgress = errors.New("command in progress")

func (i *SSMInstaller) checkCommands(commandID *string) error {
	for _, inst := range i.instances {
		cmdOut, err := i.SSM.GetCommandInvocation(&ssm.GetCommandInvocationInput{
			CommandId:  commandID,
			InstanceId: inst,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		status := aws.StringValue(cmdOut.Status)
		if status == ssm.CommandStatusInProgress {
			return trace.Wrap(ErrCommandInProgress)
		}

		var code string
		if status == ssm.CommandStatusFailed {
			code = libevent.SSMRunFailCode
		} else {
			code = libevent.SSMRunSuccessCode
		}

		event := events.SSMRun{
			Metadata: events.Metadata{
				Type: libevent.SSMRunEvent,
				Code: code,
			},
			CommandID:  aws.StringValue(commandID),
			InstanceID: aws.StringValue(inst),
			AccountID:  i.accountID,
			Region:     i.region,
			ExitCode:   aws.Int64Value(cmdOut.ResponseCode),
			Status:     status,
		}

		if err := i.emitter.EmitAuditEvent(i.ctx, &event); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Run executes the SSM document and then blocks until the command has completed.
func (i *SSMInstaller) Run(document string) error {
	output, err := i.SSM.SendCommand(&ssm.SendCommandInput{
		DocumentName: aws.String(document),
		InstanceIds:  i.instances,
		Parameters:   i.params,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	commandID := output.Command.CommandId
	for {
		select {
		case <-i.recheckTimer.C:
		case <-i.ctx.Done():
			return nil
		}
		err := i.checkCommands(commandID)
		if err != nil {
			if errors.Is(err, ErrCommandInProgress) {
				continue
			}
			return err
		}
		return nil
	}
}

// Stop cancels the command execution completion check loop.
func (i *SSMInstaller) Stop() {
	i.cancelfn()
}
