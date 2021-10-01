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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/srv/db/common"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// IAMConfig is the IAM configurator config.
type IAMConfig struct {
	// Clients is an interface for retrieving cloud clients.
	Clients common.CloudClients
}

// Check validates the IAM configurator config.
func (c *IAMConfig) Check() error {
	if c.Clients == nil {
		c.Clients = common.NewCloudClients()
	}
	return nil
}

// IAM is a service that manages IAM policies for cloud databases.
type IAM struct {
	cfg         IAMConfig
	log         logrus.FieldLogger
	awsIdentity cloud.AWSIdentity
	mu          sync.RWMutex
}

// NewIAM returns a new IAM configurator service.
func NewIAM(ctx context.Context, config IAMConfig) (*IAM, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &IAM{
		cfg: config,
		log: logrus.WithField(trace.Component, "iam"),
	}, nil
}

// Setup sets up cloud IAM policies for the provided database.
func (c *IAM) Setup(ctx context.Context, database types.Database) error {
	if database.IsRDS() {
		rds, err := c.getRDSConfigurator(ctx, database)
		if err != nil {
			return trace.Wrap(err)
		}
		return rds.setupIAM(ctx)
	}
	return nil
}

// Teardown tears down cloud IAM policies for the provided database.
func (c *IAM) Teardown(ctx context.Context, database types.Database) error {
	if database.IsRDS() {
		rds, err := c.getRDSConfigurator(ctx, database)
		if err != nil {
			return trace.Wrap(err)
		}
		return rds.teardownIAM(ctx)
	}
	return nil
}

// getRDSConfigurator returns configurator instance for the provided database.
func (c *IAM) getRDSConfigurator(ctx context.Context, database types.Database) (*rdsClient, error) {
	identity, err := c.getAWSIdentity(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newRDS(ctx, rdsConfig{
		clients:  c.cfg.Clients,
		identity: identity,
		database: database,
	})
}

// getAWSIdentity returns this process' AWS identity.
func (c *IAM) getAWSIdentity(ctx context.Context) (cloud.AWSIdentity, error) {
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
	awsIdentity, err := cloud.GetAWSIdentityWithClient(ctx, sts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.awsIdentity = awsIdentity
	return c.awsIdentity, nil
}
