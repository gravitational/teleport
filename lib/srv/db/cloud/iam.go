/*
Copyright 2021 Gravitational, Inc.

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

package cloud

import (
	"context"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// IAMConfig is the IAM configurator config.
type IAMConfig struct {
	// Semaphores is the client to acquire semaphores.
	Semaphores types.Semaphores
	// Clients is an interface for retrieving cloud clients.
	Clients common.CloudClients
	// HostID is the host identified where this agent is running.
	HostID string
	// TaskFlushInterval is the interval duration how often tasks are flushed.
	TaskFlushInterval time.Duration
	// TaskMaxRetries is the maximum number a IAM task will be retried.
	TaskMaxRetries int
}

// Check validates the IAM configurator config.
func (c *IAMConfig) Check() error {
	if c.Clients == nil {
		c.Clients = common.NewCloudClients()
	}
	if c.Semaphores == nil {
		return trace.BadParameter("missing Semaphores")
	}
	if c.HostID == "" {
		return trace.BadParameter("missing HostID")
	}
	if c.TaskFlushInterval == 0 {
		c.TaskFlushInterval = defaultTaskFlushInterval
	}
	if c.TaskMaxRetries == 0 {
		c.TaskMaxRetries = defaultTaskMaxRetries
	}
	return nil
}

// IAM is a service that manages IAM policies for cloud databases.
type IAM struct {
	ctx             context.Context
	cfg             IAMConfig
	log             logrus.FieldLogger
	awsPolicyClient aws.InlinePolicyClient
	taskQueue       iamTaskQueue
}

// NewIAM returns a new IAM configurator service.
func NewIAM(ctx context.Context, config IAMConfig) (*IAM, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &IAM{
		ctx: ctx,
		cfg: config,
		log: logrus.WithField(trace.Component, "iam"),
	}, nil
}

// Start starts the IAM configurator service.
func (c *IAM) Start() error {
	go c.runPeriodicOperations()
	return nil
}

// Setup sets up cloud IAM policies for the provided database.
func (c *IAM) Setup(ctx context.Context, database types.Database) error {
	if database.IsRDS() || database.IsRedshift() {
		c.taskQueue.addTask(iamTask{
			isSetup:  true,
			database: database,
		})
	}
	return nil
}

// Teardown tears down cloud IAM policies for the provided database.
func (c *IAM) Teardown(ctx context.Context, database types.Database) error {
	if database.IsRDS() || database.IsRedshift() {
		c.taskQueue.addTask(iamTask{
			isSetup:  false,
			database: database,
		})
	}
	return nil
}

// getAWSPolicyClient returns the policy client for configuring the inline policy.
func (c *IAM) getAWSPolicyClient() (aws.InlinePolicyClient, error) {
	if c.awsPolicyClient != nil {
		return c.awsPolicyClient, nil
	}

	policyName := "teleport-" + c.cfg.HostID
	awsPolicyClient, err := createAWSPolicyClient(c.ctx, policyName, c.cfg.Clients)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.awsPolicyClient = awsPolicyClient
	return c.awsPolicyClient, nil
}

// runPeriodicOperations runs a periodic interval to flush IAM tasks.
func (c *IAM) runPeriodicOperations() {
	tick := interval.New(interval.Config{
		Duration: c.cfg.TaskFlushInterval,
		Jitter:   utils.NewHalfJitter(),
	})
	defer tick.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return

		case <-tick.Next():
			err := c.flush()
			if err != nil {
				c.log.Error(err)
			}
		}
	}
}

// flush flushes IAM task queue.
func (c *IAM) flush() error {
	remainingTasks := c.taskQueue.take()
	if len(remainingTasks) == 0 {
		return nil
	}

	defer func() {
		c.taskQueue.addTasksForRetry(remainingTasks, c.cfg.TaskMaxRetries)
	}()

	awsPolicyClient, err := c.getAWSPolicyClient()
	if err != nil {
		return trace.Wrap(err)
	}

	expiresDuration := time.Duration(len(remainingTasks)) * defaultSemaphoreExpiresPerTask
	request := types.AcquireSemaphoreRequest{
		SemaphoreKind: types.SemaphoreKindConnection,
		SemaphoreName: policyName,
		MaxLeases:     1,
		Expires:       time.Now().Add(expiresDuration),
	}
	lease, err := c.cfg.Semaphores.AcquireSemaphore(c.ctx, request)
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		err := c.cfg.Semaphores.CancelSemaphoreLease(c.ctx, *lease)
		if err != nil {
			c.log.WithError(err).Errorf("Failed to cancel lease: %v.", lease)
		}
	}()

	// TODO(greedy52) optimize the logic to make Get/Put policy API calls only
	// once for all tasks, instead of doing it once per database.
	var errors []error
	failedTasks := []iamTask{}
	for _, task := range remainingTasks {
		err := task.run(c.ctx, c.cfg.Clients, awsPolicyClient)
		if err != nil {
			errors = append(errors, trace.Retry(err, "failed to configure IAM for %v (try #%d): %v", task.database.GetName(), task.retryCount+1, err))
			failedTasks = append(failedTasks, task)
		}
	}

	// Set failed tasks for retry.
	remainingTasks = failedTasks
	return trace.NewAggregate(errors...)
}

// createAWSPolicyClient returns the policy client.
func createAWSPolicyClient(ctx context.Context, policyName string, clients common.CloudClients) (aws.InlinePolicyClient, error) {
	sts, err := clients.GetAWSSTSClient("")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	iam, err := clients.GetAWSIAMClient("")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	awsIdentity, err := aws.GetIdentityWithClient(ctx, sts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	awsPolicyClient, err := aws.NewInlinePolicyClientForIdentity(policyName, iam, awsIdentity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return awsPolicyClient, nil
}

const (
	// policyName is the inline policy name used for the IAM idenity.
	policyName = "teleport-database-access"

	// defaultTaskFlushInterval is the default flush interval.
	defaultTaskFlushInterval = 20 * time.Second

	// defaultTaskMaxRetries is the default task max retries.
	defaultTaskMaxRetries = 3

	// defaultSemaphoreExpiresPerTask is the default per task duration used to
	// calculate expiration time for the semaphore.
	defaultSemaphoreExpiresPerTask = 10 * time.Second
)
