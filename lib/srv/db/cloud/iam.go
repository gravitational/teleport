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

package cloud

import (
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cloud"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common/iam"
)

// IAMConfig is the IAM configurator config.
type IAMConfig struct {
	// Clock is used to control time.
	Clock clockwork.Clock
	// AccessPoint is a caching client connected to the Auth Server.
	AccessPoint authclient.DatabaseAccessPoint
	// Clients is an interface for retrieving cloud clients.
	Clients cloud.Clients
	// HostID is the host identified where this agent is running.
	// DELETE IN 11.0.
	HostID string
	// onProcessedTask is called after a task is processed.
	onProcessedTask func(processedTask iamTask, processError error)
}

// Check validates the IAM configurator config.
func (c *IAMConfig) Check() error {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("missing AccessPoint")
	}
	if c.Clients == nil {
		cloudClients, err := cloud.NewClients()
		if err != nil {
			return trace.Wrap(err)
		}
		c.Clients = cloudClients
	}
	if c.HostID == "" {
		return trace.BadParameter("missing HostID")
	}
	return nil
}

// iamTask defines a background task to either setup or teardown IAM policies
// for cloud databases.
type iamTask struct {
	// isSetup indicates the task is a setup task if true, a teardown task if
	// false.
	isSetup bool
	// database is the database to configure.
	database types.Database
}

// IAM is a service that manages IAM policies for cloud databases.
//
// A semaphore lock has to be acquired by the this service before making
// changes to the IAM inline policy as database agents may share the same the
// same policy. These tasks are processed in a background goroutine to avoid
// blocking callers when acquiring the locks with retries.
type IAM struct {
	cfg IAMConfig
	log logrus.FieldLogger
	// agentIdentity is the db agent's identity, as determined by
	// shared config credential chain used to call AWS STS GetCallerIdentity.
	// Use getAWSIdentity to get the correct identity for a database,
	// which may have assume_role_arn set.
	agentIdentity awslib.Identity
	mu            sync.RWMutex
	tasks         chan iamTask

	// iamPolicyStatus indicates whether the required IAM Policy to access the database was created.
	iamPolicyStatus sync.Map
}

// NewIAM returns a new IAM configurator service.
func NewIAM(ctx context.Context, config IAMConfig) (*IAM, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &IAM{
		cfg:             config,
		log:             logrus.WithField(teleport.ComponentKey, "iam"),
		tasks:           make(chan iamTask, defaultIAMTaskQueueSize),
		iamPolicyStatus: sync.Map{},
	}, nil
}

// Start starts the IAM configurator service.
func (c *IAM) Start(ctx context.Context) error {
	go func() {
		c.log.Info("Started IAM configurator service.")
		defer c.log.Info("Stopped IAM configurator service.")
		for {
			select {
			case <-ctx.Done():
				return

			case task := <-c.tasks:
				err := c.processTask(ctx, task)
				if err != nil {
					c.log.WithError(err).Errorf("Failed to auto-configure IAM for %v.", task.database)
				}
				if c.cfg.onProcessedTask != nil {
					c.cfg.onProcessedTask(task, err)
				}
			}
		}
	}()
	return nil
}

// Setup sets up cloud IAM policies for the provided database.
func (c *IAM) Setup(ctx context.Context, database types.Database) error {
	if c.isSetupRequiredForDatabase(database) {
		c.iamPolicyStatus.Store(database.GetName(), types.IAMPolicyStatus_IAM_POLICY_STATUS_PENDING)
		return c.addTask(iamTask{
			isSetup:  true,
			database: database,
		})
	}
	return nil
}

// Teardown tears down cloud IAM policies for the provided database.
func (c *IAM) Teardown(ctx context.Context, database types.Database) error {
	if c.isSetupRequiredForDatabase(database) {
		return c.addTask(iamTask{
			isSetup:  false,
			database: database,
		})
	}
	return nil
}

// UpdateIAMStatus updates the IAMPolicyExists for the Database.
func (c *IAM) UpdateIAMStatus(database types.Database) error {
	if c.isSetupRequiredForDatabase(database) {
		awsStatus := database.GetAWS()

		iamPolicyStatus, ok := c.iamPolicyStatus.Load(database.GetName())
		if !ok {
			// If there was no key found it was a result of un-registering database
			// (and policy) as a result of deletion or failing to re-register from
			// updating database.
			awsStatus.IAMPolicyStatus = types.IAMPolicyStatus_IAM_POLICY_STATUS_UNSPECIFIED
			database.SetStatusAWS(awsStatus)
			return nil
		}

		awsStatus.IAMPolicyStatus = iamPolicyStatus.(types.IAMPolicyStatus)
		database.SetStatusAWS(awsStatus)
	}
	return nil
}

// isSetupRequiredForDatabase returns true if database type is supported.
func (c *IAM) isSetupRequiredForDatabase(database types.Database) bool {
	switch database.GetType() {
	case types.DatabaseTypeRDS,
		types.DatabaseTypeRDSProxy,
		types.DatabaseTypeRedshift:
		return true
	case types.DatabaseTypeElastiCache:
		ok, err := iam.CheckElastiCacheSupportsIAMAuth(database)
		if err != nil {
			c.log.WithError(err).Debugf("Assuming database %s supports IAM auth.",
				database.GetName())
			return true
		}
		return ok
	case types.DatabaseTypeMemoryDB:
		ok, err := iam.CheckMemoryDBSupportsIAMAuth(database)
		if err != nil {
			c.log.WithError(err).Debugf("Assuming database %s supports IAM auth.",
				database.GetName())
			return true
		}
		return ok
	default:
		return false
	}
}

// getAWSConfigurator returns configurator instance for the provided database.
func (c *IAM) getAWSConfigurator(ctx context.Context, database types.Database) (*awsClient, error) {
	identity, err := c.getAWSIdentity(ctx, database)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	policyName, err := c.getPolicyName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newAWS(ctx, awsConfig{
		clients:    c.cfg.Clients,
		policyName: policyName,
		identity:   identity,
		database:   database,
	})
}

// getAWSIdentity returns the identity used to access the given database,
// that is either the agent's identity or the database's configured assume-role.
func (c *IAM) getAWSIdentity(ctx context.Context, database types.Database) (awslib.Identity, error) {
	meta := database.GetAWS()
	if meta.AssumeRoleARN != "" {
		// If the database has an assume role ARN, use that instead of
		// agent identity. This avoids an unnecessary sts call too.
		return awslib.IdentityFromArn(meta.AssumeRoleARN)
	}

	c.mu.RLock()
	if c.agentIdentity != nil {
		defer c.mu.RUnlock()
		return c.agentIdentity, nil
	}
	c.mu.RUnlock()
	sts, err := c.cfg.Clients.GetAWSSTSClient(ctx, meta.Region, cloud.WithAmbientCredentials())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	awsIdentity, err := awslib.GetIdentityWithClient(ctx, sts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.agentIdentity = awsIdentity
	return c.agentIdentity, nil
}

// getPolicyName returns the inline policy name.
func (c *IAM) getPolicyName() (string, error) {
	clusterName, err := c.cfg.AccessPoint.GetClusterName()
	if err != nil {
		return "", trace.Wrap(err)
	}

	prefix := clusterName.GetClusterName()

	// If the length of the policy name is over the limit, trim the cluster
	// name from right and keep the policyNameSuffix intact.
	maxPrefixLength := maxPolicyNameLength - len(policyNameSuffix)
	if len(prefix) > maxPrefixLength {
		prefix = prefix[:maxPrefixLength]
	}

	return prefix + policyNameSuffix, nil
}

// processTask runs an IAM task.
func (c *IAM) processTask(ctx context.Context, task iamTask) error {
	configurator, err := c.getAWSConfigurator(ctx, task.database)
	if err != nil {
		c.iamPolicyStatus.Store(task.database.GetName(), types.IAMPolicyStatus_IAM_POLICY_STATUS_FAILED)
		if trace.Unwrap(err) == credentials.ErrNoValidProvidersFoundInChain {
			c.log.Warnf("No AWS credentials provider. Skipping IAM task for database %v.", task.database.GetName())
			return nil
		}
		return trace.Wrap(err)
	}

	// Acquire a semaphore before making changes to the shared IAM policy.
	//
	// TODO(greedy52) ideally tasks can be bundled so the semaphore is acquired
	// once per group, and the IAM policy is only get/put once per group.
	lease, err := services.AcquireSemaphoreWithRetry(ctx, services.AcquireSemaphoreWithRetryConfig{
		Service: c.cfg.AccessPoint,
		Request: types.AcquireSemaphoreRequest{
			SemaphoreKind: configurator.cfg.policyName,
			SemaphoreName: configurator.cfg.identity.GetName(),
			MaxLeases:     1,
			Holder:        c.cfg.HostID,

			// If the semaphore fails to release for some reason, it will expire in a
			// minute on its own.
			Expires: c.cfg.Clock.Now().Add(time.Minute),
		},

		// Retry with some jitters up to twice of the semaphore expire time.
		Retry: retryutils.LinearConfig{
			Step:   10 * time.Second,
			Max:    2 * time.Minute,
			Jitter: retryutils.NewHalfJitter(),
		},
	})
	if err != nil {
		c.iamPolicyStatus.Store(task.database.GetName(), types.IAMPolicyStatus_IAM_POLICY_STATUS_FAILED)
		return trace.Wrap(err)
	}

	defer func() {
		err := c.cfg.AccessPoint.CancelSemaphoreLease(ctx, *lease)
		if err != nil {
			c.log.WithError(err).Errorf("Failed to cancel lease: %v.", lease)
		}
	}()

	if task.isSetup {
		iamAuthErr := configurator.setupIAMAuth(ctx)
		iamPolicySetup, iamPolicyErr := configurator.setupIAMPolicy(ctx)
		statusEnum := types.IAMPolicyStatus_IAM_POLICY_STATUS_FAILED
		if iamPolicySetup {
			statusEnum = types.IAMPolicyStatus_IAM_POLICY_STATUS_SUCCESS
		}
		c.iamPolicyStatus.Store(task.database.GetName(), statusEnum)

		return trace.NewAggregate(iamAuthErr, iamPolicyErr)
	}

	c.iamPolicyStatus.Delete(task.database.GetName())
	return configurator.teardownIAM(ctx)
}

// addTask adds a task for processing.
func (c *IAM) addTask(task iamTask) error {
	select {
	case c.tasks <- task:
		return nil

	default:
		return trace.LimitExceeded("failed to create IAM task for %v", task.database.GetName())
	}
}

const (
	// maxPolicyNameLength is the maximum number of characters for IAM policy
	// name.
	maxPolicyNameLength = 128

	// policyNameSuffix is the suffix for inline policy names.
	policyNameSuffix = "-teleport-database-access"

	// defaultIAMTaskQueueSize is the default task queue size for IAM configurator.
	defaultIAMTaskQueueSize = 10000
)
