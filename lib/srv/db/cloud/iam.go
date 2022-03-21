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
	"sync"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/utils"

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
	// TaskQueueSize is the size of the task queue.
	TaskQueueSize int
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
	if c.TaskQueueSize <= 0 {
		c.TaskQueueSize = defaultIAMTaskQueueSize
	}
	return nil
}

// iamTask defines an IAM task for the database.
type iamTask struct {
	// isSetup indicates the task is a setup task if true, a teardown task if
	// false.
	isSetup bool
	// database is the database to configure.
	database types.Database
}

// IAM is a service that manages IAM policies for cloud databases.
type IAM struct {
	ctx         context.Context
	cfg         IAMConfig
	log         logrus.FieldLogger
	awsIdentity aws.Identity
	mu          sync.RWMutex
	tasks       chan iamTask
}

// NewIAM returns a new IAM configurator service.
func NewIAM(ctx context.Context, config IAMConfig) (*IAM, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &IAM{
		ctx:   ctx,
		cfg:   config,
		log:   logrus.WithField(trace.Component, "iam"),
		tasks: make(chan iamTask, config.TaskQueueSize),
	}, nil
}

// Start starts the IAM configurator service.
func (c *IAM) Start() error {
	go func() {
		c.log.Debug("Started IAM configurator service.")
		defer c.log.Debug("Stopped IAM configurator service.")
		for {
			select {
			case <-c.ctx.Done():
				return

			case task := <-c.tasks:
				err := c.runTask(c.ctx, task)
				if err != nil {
					c.log.WithError(err).Errorf("Failed to auto-configure IAM for %v.", task.database)
				}
			}
		}
	}()
	return nil
}

// Setup sets up cloud IAM policies for the provided database.
func (c *IAM) Setup(ctx context.Context, database types.Database) error {
	if database.IsRDS() || database.IsRedshift() {
		return c.addTask(iamTask{
			isSetup:  true,
			database: database,
		})
	}
	return nil
}

// Teardown tears down cloud IAM policies for the provided database.
func (c *IAM) Teardown(ctx context.Context, database types.Database) error {
	if database.IsRDS() || database.IsRedshift() {
		return c.addTask(iamTask{
			isSetup:  false,
			database: database,
		})
	}
	return nil
}

// getAWSConfigurator returns configurator instance for the provided database.
func (c *IAM) getAWSConfigurator(ctx context.Context, database types.Database) (*awsClient, error) {
	identity, err := c.getAWSIdentity(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newAWS(ctx, awsConfig{
		clients:    c.cfg.Clients,
		policyName: databaseAccessInlinePolicyName,
		identity:   identity,
		database:   database,
	})
}

// getAWSIdentity returns this process' AWS identity.
func (c *IAM) getAWSIdentity(ctx context.Context) (aws.Identity, error) {
	c.mu.RLock()
	if c.awsIdentity != nil {
		defer c.mu.RUnlock()
		return c.awsIdentity, nil
	}
	c.mu.RUnlock()
	sts, err := c.cfg.Clients.GetAWSSTSClient("")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	awsIdentity, err := aws.GetIdentityWithClient(ctx, sts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.awsIdentity = awsIdentity
	return c.awsIdentity, nil
}

// runTask TODO
func (c *IAM) runTask(ctx context.Context, task iamTask) error {
	configurator, err := c.getAWSConfigurator(ctx, task.database)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(greedy52) ideally tasks can be bundled so the semaphore is acquired
	// once per group, and the IAM policy is only get/put once per group.
	lease, err := services.AcquireSemaphoreWithRetry(ctx, services.AcquireSemaphoreWithRetryConfig{
		Service: c.cfg.Semaphores,
		Request: types.AcquireSemaphoreRequest{
			SemaphoreKind: types.SemaphoreKindConnection,
			SemaphoreName: databaseAccessInlinePolicyName,
			MaxLeases:     1,
			Expires:       time.Now().Add(time.Minute),
		},
		Retry: utils.LinearConfig{
			Step:   10 * time.Second,
			Max:    time.Minute,
			Jitter: utils.NewHalfJitter(),
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		err := c.cfg.Semaphores.CancelSemaphoreLease(ctx, *lease)
		if err != nil {
			c.log.WithError(err).Errorf("Failed to cancel lease: %v.", lease)
		}
	}()

	if task.isSetup {
		return configurator.setupIAM(ctx)
	}
	return configurator.teardownIAM(ctx)
}

// addTask add a task for processing.
func (c *IAM) addTask(task iamTask) error {
	select {
	case c.tasks <- task:
		return nil

	default:
		return trace.LimitExceeded("failed to create IAM task for %v", task.database)
	}
}

// isIdle returns true if there is no tasks being processed.
func (c *IAM) isIdle() bool {
	return len(c.tasks) == 0
}

const (
	// databaseAccessInlinePolicyName is the inline policy name for database
	// access permissions for the IAM idenity.
	databaseAccessInlinePolicyName = "teleport-database-access"

	// defaultIAMTaskQueueSize is the default task queue size for IAM configurator.
	defaultIAMTaskQueueSize = 10000
)
