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
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common/iam"
)

// IAMConfig is the IAM configurator config.
type IAMConfig struct {
	// Clock is used to control time.
	Clock clockwork.Clock
	// AccessPoint is a caching client connected to the Auth Server.
	AccessPoint authclient.DatabaseAccessPoint
	// AWSConfigProvider provides [aws.Config] for AWS SDK service clients.
	AWSConfigProvider awsconfig.Provider
	// HostID is the host identified where this agent is running.
	// DELETE IN 11.0.
	HostID string
	// onProcessedTask is called after a task is processed.
	onProcessedTask func(processedTask iamTask, processError error)
	// awsClients is an SDK client provider.
	awsClients awsClientProvider
}

// Check validates the IAM configurator config.
func (c *IAMConfig) Check() error {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("missing AccessPoint")
	}
	if c.AWSConfigProvider == nil {
		return trace.BadParameter("missing AWSConfigProvider")
	}
	if c.HostID == "" {
		return trace.BadParameter("missing HostID")
	}
	if c.awsClients == nil {
		c.awsClients = defaultAWSClients{}
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
	cfg    IAMConfig
	logger *slog.Logger
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
		logger:          slog.With(teleport.ComponentKey, "iam"),
		tasks:           make(chan iamTask, defaultIAMTaskQueueSize),
		iamPolicyStatus: sync.Map{},
	}, nil
}

// Start starts the IAM configurator service.
func (c *IAM) Start(ctx context.Context) error {
	go func() {
		c.logger.InfoContext(ctx, "Started IAM configurator service")
		defer c.logger.InfoContext(ctx, "Stopped IAM configurator service")
		for {
			select {
			case <-ctx.Done():
				return

			case task := <-c.tasks:
				err := c.processTask(ctx, task)
				if err != nil {
					c.logger.ErrorContext(ctx, "Failed to auto-configure IAM for database",
						"error", err,
						"database", task.database,
					)
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
	if c.isSetupRequiredForDatabase(ctx, database) {
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
	if c.isSetupRequiredForDatabase(ctx, database) {
		return c.addTask(iamTask{
			isSetup:  false,
			database: database,
		})
	}
	return nil
}

// UpdateIAMStatus updates the IAMPolicyExists for the Database.
func (c *IAM) UpdateIAMStatus(ctx context.Context, database types.Database) error {
	if c.isSetupRequiredForDatabase(ctx, database) {
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
func (c *IAM) isSetupRequiredForDatabase(ctx context.Context, database types.Database) bool {
	switch database.GetType() {
	case types.DatabaseTypeRDS,
		types.DatabaseTypeRDSProxy,
		types.DatabaseTypeRedshift:
		return true
	case types.DatabaseTypeElastiCache:
		ok, err := iam.CheckElastiCacheSupportsIAMAuth(database)
		if err != nil {
			c.logger.DebugContext(ctx, "Assuming database supports IAM auth",
				"error", err,
				"database", database.GetName(),
			)
			return true
		}
		return ok
	case types.DatabaseTypeMemoryDB:
		ok, err := iam.CheckMemoryDBSupportsIAMAuth(database)
		if err != nil {
			c.logger.DebugContext(ctx, "Assuming database supports IAM auth",
				"error", err,
				"database", database.GetName(),
			)
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
		awsConfigProvider: c.cfg.AWSConfigProvider,
		database:          database,
		identity:          identity,
		policyName:        policyName,
		awsClients:        c.cfg.awsClients,
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

	awsCfg, err := c.cfg.AWSConfigProvider.GetConfig(ctx, meta.Region, awsconfig.WithAmbientCredentials())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt := c.cfg.awsClients.getSTSClient(awsCfg)
	_, err = awsCfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed to retrieve credentials")
	}
	awsIdentity, err := awslib.GetIdentityWithClient(ctx, clt)
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
	clusterName, err := c.cfg.AccessPoint.GetClusterName(context.TODO())
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
		if strings.Contains(err.Error(), "failed to retrieve credentials") {
			c.logger.WarnContext(ctx, "Failed to load AWS IAM configurator, skipping IAM task for database",
				"database", task.database.GetName(),
				"error", err,
			)
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
			Jitter: retryutils.HalfJitter,
		},
	})
	if err != nil {
		c.iamPolicyStatus.Store(task.database.GetName(), types.IAMPolicyStatus_IAM_POLICY_STATUS_FAILED)
		return trace.Wrap(err)
	}

	defer func() {
		err := c.cfg.AccessPoint.CancelSemaphoreLease(ctx, *lease)
		if err != nil {
			c.logger.ErrorContext(ctx, "Failed to cancel lease",
				"error", err,
				"lease", lease.LeaseID,
			)
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
