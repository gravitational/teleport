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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"

	"github.com/gravitational/teleport/api/types"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/limiter"
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
	// DELETE IN 11.0.
	HostID string
	// TaskQueueSize is the size of the task queue.
	TaskQueueSize int
	// SetupRateLimiter limits the rate of setup calls per database.
	SetupRateLimiter *limiter.RateLimiter
}

// Check validates the IAM configurator config.
func (c *IAMConfig) Check() (err error) {
	if c.Semaphores == nil {
		return trace.BadParameter("missing Semaphores")
	}
	if c.Clients == nil {
		c.Clients = common.NewCloudClients()
	}
	if c.HostID == "" {
		return trace.BadParameter("missing HostID")
	}
	if c.TaskQueueSize <= 0 {
		c.TaskQueueSize = defaultIAMTaskQueueSize
	}
	if c.SetupRateLimiter == nil {
		c.SetupRateLimiter, err = limiter.NewRateLimiter(limiter.Config{
			Rates: []limiter.Rate{{
				Period:  defaultSetupRatePeriod,
				Average: 1,
				Burst:   1,
			}},
		})
		if err != nil {
			return trace.Wrap(err)
		}
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
	closeCtx    context.Context
	cfg         IAMConfig
	log         logrus.FieldLogger
	awsIdentity awslib.Identity
	mu          sync.RWMutex
	tasks       chan iamTask
}

// NewIAM returns a new IAM configurator service.
func NewIAM(ctx context.Context, config IAMConfig) (*IAM, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &IAM{
		closeCtx: ctx,
		cfg:      config,
		log:      logrus.WithField(trace.Component, "iam"),
		tasks:    make(chan iamTask, config.TaskQueueSize),
	}, nil
}

// Start starts the IAM configurator service.
func (c *IAM) Start() {
	// DELETE IN 11.0.
	c.migrateInlinePolicy(c.closeCtx)

	c.log.Info("Started IAM configurator service.")
	defer c.log.Info("Stopped IAM configurator service.")
	for {
		select {
		case <-c.closeCtx.Done():
			return

		case task := <-c.tasks:
			err := c.processTask(c.closeCtx, task)
			if err != nil {
				c.log.WithError(err).Errorf("Failed to auto-configure IAM for %v.", task.database)
			}
		}
	}
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
func (c *IAM) getAWSIdentity(ctx context.Context) (awslib.Identity, error) {
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
	awsIdentity, err := awslib.GetIdentityWithClient(ctx, sts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.awsIdentity = awsIdentity
	return c.awsIdentity, nil
}

// processTask runs an IAM task.
func (c *IAM) processTask(ctx context.Context, task iamTask) error {
	configurator, err := c.getAWSConfigurator(ctx, task.database)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(greedy52) ideally tasks can be bundled so the semaphore is acquired
	// once per group, and the IAM policy is only get/put once per group.
	lease, err := services.AcquireSemaphoreWithRetry(ctx, services.AcquireSemaphoreWithRetryConfig{
		Service: c.cfg.Semaphores,
		Request: types.AcquireSemaphoreRequest{
			SemaphoreKind: databaseAccessInlinePolicyName,
			SemaphoreName: configurator.cfg.identity.GetName(),
			MaxLeases:     1,
			Expires:       time.Now().Add(time.Minute),
		},
		Retry: utils.LinearConfig{
			Step:   10 * time.Second,
			Max:    2 * time.Minute,
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
	if task.isSetup {
		token := task.database.GetName() + task.database.GetURI()
		if err := c.cfg.SetupRateLimiter.RegisterRequest(token, nil); err != nil {
			return trace.LimitExceeded(err.Error())
		}
	}

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

// migrateInlinePolicy removes old inline policies "teleport-<host-id>" for
// the caller identity. DELETE IN 11.0.
func (c *IAM) migrateInlinePolicy(ctx context.Context) {
	oldPolicyName := "teleport-" + c.cfg.HostID
	identity, err := c.getAWSIdentity(ctx)
	if err != nil {
		c.log.WithError(err).Debugf("Failed to get AWS identity.")
		return
	}

	iamClient, err := c.cfg.Clients.GetAWSIAMClient("")
	if err != nil {
		c.log.WithError(err).Debugf("Failed to get IAM client.")
		return
	}

	switch identity.(type) {
	case awslib.Role:
		_, err = iamClient.DeleteRolePolicyWithContext(ctx, &iam.DeleteRolePolicyInput{
			PolicyName: aws.String(oldPolicyName),
			RoleName:   aws.String(identity.GetName()),
		})
	case awslib.User:
		_, err = iamClient.DeleteUserPolicyWithContext(ctx, &iam.DeleteUserPolicyInput{
			PolicyName: aws.String(oldPolicyName),
			UserName:   aws.String(identity.GetName()),
		})
	}

	if err != nil && !trace.IsNotFound(common.ConvertError(err)) {
		c.log.WithError(err).Errorf("Failed to delete inline policy %v for %v. It is recommended to remove this policy as it is no longer required.", oldPolicyName, identity)
	}
}

const (
	// databaseAccessInlinePolicyName is the inline policy name for database
	// access permissions for the IAM idenity.
	databaseAccessInlinePolicyName = "teleport-managed-database-access"

	// defaultIAMTaskQueueSize is the default task queue size for IAM configurator.
	defaultIAMTaskQueueSize = 10000

	// defaultSetupRatePeriod is the default rate period per database IAM setup.
	defaultSetupRatePeriod = 10 * time.Minute
)
