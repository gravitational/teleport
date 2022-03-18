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

	"github.com/gravitational/teleport/api/types"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/srv/db/common"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// awsConfig is the config for the client that configures IAM for AWS databases.
type awsConfig struct {
	// clients is an interface for creating AWS clients.
	clients common.CloudClients
	// database is the database instance to configure.
	database types.Database
	// awsPolicyClient is the IAM inline policy to configure.
	awsPolicyClient awslib.InlinePolicyClient
}

// Check validates the config.
func (c *awsConfig) Check() error {
	if c.clients == nil {
		return trace.BadParameter("missing parameter clients")
	}
	if c.database == nil {
		return trace.BadParameter("missing parameter database")
	}
	if c.awsPolicyClient == nil {
		return trace.BadParameter("missing parameter aws policy client")
	}
	return nil
}

// newAWS creates a new AWS IAM configurator.
func newAWS(ctx context.Context, config awsConfig) (*awsClient, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	rds, err := config.clients.GetAWSRDSClient(config.database.GetAWS().Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &awsClient{
		cfg: config,
		rds: rds,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "aws",
			"db":            config.database.GetName(),
		}),
	}, nil
}

type awsClient struct {
	cfg awsConfig
	rds rdsiface.RDSAPI
	log logrus.FieldLogger
}

// setupIAM configures IAM for RDS, Aurora or Redshift database.
func (r *awsClient) setupIAM(ctx context.Context) error {
	var errors []error
	if err := r.ensureIAMAuth(ctx); err != nil {
		if trace.IsAccessDenied(err) { // Permission errors are expected.
			r.log.Debugf("No permissions to enable IAM auth: %v.", err)
		} else {
			errors = append(errors, err)
		}
	}
	if err := r.ensureIAMPolicy(ctx); err != nil {
		if trace.IsAccessDenied(err) { // Permission errors are expected.
			r.log.Debugf("No permissions to ensure IAM policy: %v.", err)
		} else {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

// teardownIAM deconfigures IAM for RDS, Aurora or Redshift database.
func (r *awsClient) teardownIAM(ctx context.Context) error {
	var errors []error
	if err := r.deleteIAMPolicy(ctx); err != nil {
		if trace.IsAccessDenied(err) { // Permission errors are expected.
			r.log.Debugf("No permissions to delete IAM policy: %v.", err)
		} else {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

// ensureIAMAuth enables RDS instance IAM auth if it isn't enabled.
func (r *awsClient) ensureIAMAuth(ctx context.Context) error {
	if r.cfg.database.IsRedshift() {
		// Redshift IAM auth is always enabled.
		return nil
	}
	if r.cfg.database.GetAWS().RDS.IAMAuth {
		r.log.Debug("IAM auth already enabled.")
		return nil
	}
	if err := r.enableIAMAuth(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// enableIAMAuth turns on IAM auth setting on the RDS instance.
func (r *awsClient) enableIAMAuth(ctx context.Context) error {
	r.log.Debug("Enabling IAM auth.")
	var err error
	if r.cfg.database.GetAWS().RDS.ClusterID != "" {
		_, err = r.rds.ModifyDBClusterWithContext(ctx, &rds.ModifyDBClusterInput{
			DBClusterIdentifier:             aws.String(r.cfg.database.GetAWS().RDS.ClusterID),
			EnableIAMDatabaseAuthentication: aws.Bool(true),
			ApplyImmediately:                aws.Bool(true),
		})
		return common.ConvertError(err)
	}
	if r.cfg.database.GetAWS().RDS.InstanceID != "" {
		_, err = r.rds.ModifyDBInstanceWithContext(ctx, &rds.ModifyDBInstanceInput{
			DBInstanceIdentifier:            aws.String(r.cfg.database.GetAWS().RDS.InstanceID),
			EnableIAMDatabaseAuthentication: aws.Bool(true),
			ApplyImmediately:                aws.Bool(true),
		})
		return common.ConvertError(err)
	}
	return trace.BadParameter("no RDS cluster ID or instance ID for %v", r.cfg.database)
}

// ensureIAMPolicy adds database connect permissions to the agent's policy.
func (r *awsClient) ensureIAMPolicy(ctx context.Context) error {
	policy, err := r.cfg.awsPolicyClient.Get(ctx)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		policy = awslib.NewPolicyDocument()
	}

	action := r.cfg.database.GetIAMAction()
	var changed bool
	for _, resource := range r.cfg.database.GetIAMResources() {
		if policy.Ensure(awslib.EffectAllow, action, resource) {
			r.log.Debugf("Permission %q for %q is already part of policy.", action, resource)
		} else {
			r.log.Debugf("Adding permission %q for %q to policy.", action, resource)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	err = r.cfg.awsPolicyClient.Put(ctx, policy)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// deleteIAMPolicy deletes IAM access policy from the identity this agent is running as.
func (r *awsClient) deleteIAMPolicy(ctx context.Context) error {
	policy, err := r.cfg.awsPolicyClient.Get(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	action := r.cfg.database.GetIAMAction()
	for _, resource := range r.cfg.database.GetIAMResources() {
		policy.Delete(awslib.EffectAllow, action, resource)
	}
	// If policy is empty now, delete it as IAM policy can't be empty.
	if len(policy.Statements) == 0 {
		return r.cfg.awsPolicyClient.Delete(ctx)
	}
	return r.cfg.awsPolicyClient.Put(ctx, policy)
}
