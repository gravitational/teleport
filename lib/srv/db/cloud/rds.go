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
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/srv/db/common"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// rdsConfig is the config for the client that auto-configures RDS databases.
type rdsConfig struct {
	// clients is an interface for creating AWS clients.
	clients common.CloudClients
	// identity is AWS identity this database agent is running as.
	identity cloud.AWSIdentity
	// database is the database instance to configure.
	database types.Database
}

// Check validates the config.
func (c *rdsConfig) Check() error {
	if c.clients == nil {
		return trace.BadParameter("missing parameter clients")
	}
	if c.identity == nil {
		return trace.BadParameter("missing parameter identity")
	}
	if c.database == nil {
		return trace.BadParameter("missing parameter database")
	}
	return nil
}

// newRDS creates a new RDS configurator.
func newRDS(ctx context.Context, config rdsConfig) (*rdsClient, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	rds, err := config.clients.GetAWSRDSClient(config.database.GetAWS().Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	iam, err := config.clients.GetAWSIAMClient(config.database.GetAWS().Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &rdsClient{
		cfg: config,
		rds: rds,
		iam: iam,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "rds",
			"db":            config.database.GetName(),
		}),
	}, nil
}

type rdsClient struct {
	cfg rdsConfig
	rds rdsiface.RDSAPI
	iam iamiface.IAMAPI
	log logrus.FieldLogger
}

// setupIAM configures IAM for RDS or Aurora database.
func (r *rdsClient) setupIAM(ctx context.Context) error {
	var errors []error
	if err := r.ensureIAMAuth(ctx); err != nil {
		if trace.IsAccessDenied(err) { // Permission errors are expected.
			r.log.Debugf("No permissions to enable IAM auth: %v.", err)
		} else {
			errors = append(errors, err)
		}
	}
	if err := r.attachIAMPolicy(ctx); err != nil {
		if trace.IsAccessDenied(err) { // Permission errors are expected.
			r.log.Debugf("No permissions to attach IAM policy: %v.", err)
		} else {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

// teardownIAM deconfigures IAM for RDS or Aurora database.
func (r *rdsClient) teardownIAM(ctx context.Context) error {
	var errors []error
	if err := r.detachIAMPolicy(ctx); err != nil {
		if trace.IsAccessDenied(err) { // Permission errors are expected.
			r.log.Debugf("No permissions to detach IAM policy: %v.", err)
		} else {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

// ensureIAMAuth makes enables RDS instance IAM auth if it isn't enabled.
func (r *rdsClient) ensureIAMAuth(ctx context.Context) error {
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
func (r *rdsClient) enableIAMAuth(ctx context.Context) error {
	r.log.Debug("Enabling IAM auth.")
	var err error
	if r.cfg.database.GetAWS().RDS.ClusterID != "" {
		_, err = r.rds.ModifyDBClusterWithContext(ctx, &rds.ModifyDBClusterInput{
			DBClusterIdentifier:             aws.String(r.cfg.database.GetAWS().RDS.ClusterID),
			EnableIAMDatabaseAuthentication: aws.Bool(true),
			ApplyImmediately:                aws.Bool(true),
		})
	} else {
		_, err = r.rds.ModifyDBInstanceWithContext(ctx, &rds.ModifyDBInstanceInput{
			DBInstanceIdentifier:            aws.String(r.cfg.database.GetAWS().RDS.InstanceID),
			EnableIAMDatabaseAuthentication: aws.Bool(true),
			ApplyImmediately:                aws.Bool(true),
		})
	}
	return common.ConvertError(err)
}

// attachIAMPolicy attaches IAM access policy to the identity this agent is running as.
func (r *rdsClient) attachIAMPolicy(ctx context.Context) error {
	r.log.Debugf("Attaching IAM policy to %v.", r.cfg.identity)
	var err error
	switch r.cfg.identity.(type) {
	case cloud.AWSRole:
		_, err = r.iam.PutRolePolicyWithContext(ctx, &iam.PutRolePolicyInput{
			PolicyName:     aws.String(r.cfg.database.GetName()),
			PolicyDocument: aws.String(r.cfg.database.GetRDSPolicy()),
			RoleName:       aws.String(r.cfg.identity.GetName()),
		})
	case cloud.AWSUser:
		_, err = r.iam.PutUserPolicyWithContext(ctx, &iam.PutUserPolicyInput{
			PolicyName:     aws.String(r.cfg.database.GetName()),
			PolicyDocument: aws.String(r.cfg.database.GetRDSPolicy()),
			UserName:       aws.String(r.cfg.identity.GetName()),
		})
	default:
		return trace.BadParameter("can only attach policies to roles or users, got %v", r.cfg.identity)
	}
	return common.ConvertError(err)
}

// detachIAMPolicy detaches IAM access policy from the identity this agent is running as.
func (r *rdsClient) detachIAMPolicy(ctx context.Context) error {
	r.log.Debugf("Detaching IAM policy from %v.", r.cfg.identity)
	var err error
	switch r.cfg.identity.(type) {
	case cloud.AWSRole:
		_, err = r.iam.DeleteRolePolicyWithContext(ctx, &iam.DeleteRolePolicyInput{
			PolicyName: aws.String(r.cfg.database.GetName()),
			RoleName:   aws.String(r.cfg.identity.GetName()),
		})
	case cloud.AWSUser:
		_, err = r.iam.DeleteUserPolicyWithContext(ctx, &iam.DeleteUserPolicyInput{
			PolicyName: aws.String(r.cfg.database.GetName()),
			UserName:   aws.String(r.cfg.identity.GetName()),
		})
	default:
		return trace.BadParameter("can only detach policies from roles or users, got %v", r.cfg.identity)
	}
	return common.ConvertError(err)
}
