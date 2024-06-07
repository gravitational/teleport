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
	"encoding/json"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	dbiam "github.com/gravitational/teleport/lib/srv/db/common/iam"
)

// awsConfig is the config for the client that configures IAM for AWS databases.
type awsConfig struct {
	// clients is an interface for creating AWS clients.
	clients cloud.Clients
	// identity is AWS identity this database agent is running as.
	identity awslib.Identity
	// database is the database instance to configure.
	database types.Database
	// policyName is the name of the inline policy for the identity.
	policyName string
}

// Check validates the config.
func (c *awsConfig) Check() error {
	if c.clients == nil {
		return trace.BadParameter("missing parameter clients")
	}
	if c.identity == nil {
		return trace.BadParameter("missing parameter identity")
	}
	if c.database == nil {
		return trace.BadParameter("missing parameter database")
	}
	if c.policyName == "" {
		return trace.BadParameter("missing parameter policy name")
	}
	return nil
}

// newAWS creates a new AWS IAM configurator.
func newAWS(ctx context.Context, config awsConfig) (*awsClient, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	logger := logrus.WithFields(logrus.Fields{
		teleport.ComponentKey: "aws",
		"db":                  config.database.GetName(),
	})
	dbConfigurator, err := getDBConfigurator(ctx, logger, config.clients, config.database)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	meta := config.database.GetAWS()
	iam, err := config.clients.GetAWSIAMClient(ctx, meta.Region,
		cloud.WithAssumeRoleFromAWSMeta(meta),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &awsClient{
		cfg:            config,
		dbConfigurator: dbConfigurator,
		iam:            iam,
		log:            logger,
	}, nil
}

type dbIAMAuthConfigurator interface {
	// ensureIAMAuth enables DB IAM auth if it isn't already enabled.
	ensureIAMAuth(context.Context, types.Database) error
}

// getDBConfigurator returns a database IAM Auth configurator.
func getDBConfigurator(ctx context.Context, log logrus.FieldLogger, clients cloud.Clients, db types.Database) (dbIAMAuthConfigurator, error) {
	if db.IsRDS() {
		// Only setting for RDS instances and Aurora clusters.
		return &rdsDBConfigurator{clients: clients, log: log}, nil
	}
	// IAM Auth for Redshift, ElastiCache, and RDS Proxy is always enabled.
	return &nopDBConfigurator{}, nil
}

type awsClient struct {
	cfg            awsConfig
	dbConfigurator dbIAMAuthConfigurator
	iam            iamiface.IAMAPI
	log            logrus.FieldLogger
}

// setupIAMAuth ensures the IAM Authentication is enbaled for RDS, Aurora, ElastiCache or Redshift database.
func (r *awsClient) setupIAMAuth(ctx context.Context) error {
	if err := r.dbConfigurator.ensureIAMAuth(ctx, r.cfg.database); err != nil {
		if trace.IsAccessDenied(err) { // Permission errors are expected.
			r.log.Debugf("No permissions to enable IAM auth: %v.", err)
			return nil
		}
		return trace.Wrap(err)
	}

	return nil
}

// setupIAMAuth ensures the IAM Policy is set up for RDS, Aurora, ElastiCache or Redshift database.
// It returns whether the IAM Policy was properly set up.
// Eg for RDS: adds policy to allow the `rds-db:connect` action for the Database.
func (r *awsClient) setupIAMPolicy(ctx context.Context) (bool, error) {
	if err := r.ensureIAMPolicy(ctx); err != nil {
		if trace.IsAccessDenied(err) { // Permission errors are expected.
			r.log.Debugf("No permissions to ensure IAM policy: %v.", err)
			return false, nil
		}

		return false, trace.Wrap(err)
	}

	return true, nil
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

// ensureIAMPolicy adds database connect permissions to the agent's policy.
func (r *awsClient) ensureIAMPolicy(ctx context.Context) error {
	dbIAM, placeholders, err := dbiam.GetAWSPolicyDocument(r.cfg.database)
	if err != nil {
		return trace.Wrap(err)
	}

	policy, err := r.getIAMPolicy(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	var changed bool
	dbIAM.ForEach(func(effect, action, resource string) {
		if policy.Ensure(effect, action, resource) {
			r.log.Debugf("Permission %q for %q is already part of policy.", action, resource)
		} else {
			r.log.Debugf("Adding permission %q for %q to policy.", action, resource)
			changed = true
		}
	})
	if !changed {
		return nil
	}
	err = r.updateIAMPolicy(ctx, policy)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(placeholders) > 0 {
		r.log.Warnf("Please make sure the database agent has the IAM permissions to fetch cloud metadata, or make sure these values are set in the static config. Placeholders %q are found when configuring the IAM policy for database %v.",
			placeholders, r.cfg.database.GetName())
	}
	return nil
}

// deleteIAMPolicy deletes IAM access policy from the identity this agent is running as.
func (r *awsClient) deleteIAMPolicy(ctx context.Context) error {
	dbIAM, _, err := dbiam.GetAWSPolicyDocument(r.cfg.database)
	if err != nil {
		return trace.Wrap(err)
	}

	policy, err := r.getIAMPolicy(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	dbIAM.ForEach(func(effect, action, resource string) {
		policy.Delete(effect, action, resource)
	})
	// If policy is empty now, delete it as IAM policy can't be empty.
	if len(policy.Statements) == 0 {
		return r.detachIAMPolicy(ctx)
	}
	return r.updateIAMPolicy(ctx, policy)
}

// getIAMPolicy fetches and returns this agent's parsed IAM policy document.
func (r *awsClient) getIAMPolicy(ctx context.Context) (*awslib.PolicyDocument, error) {
	var policyDocument string
	switch r.cfg.identity.(type) {
	case awslib.Role:
		out, err := r.iam.GetRolePolicyWithContext(ctx, &iam.GetRolePolicyInput{
			PolicyName: aws.String(r.cfg.policyName),
			RoleName:   aws.String(r.cfg.identity.GetName()),
		})
		if err != nil {
			if trace.IsNotFound(awslib.ConvertIAMError(err)) {
				return awslib.NewPolicyDocument(), nil
			}
			return nil, awslib.ConvertIAMError(err)
		}
		policyDocument = aws.StringValue(out.PolicyDocument)
	case awslib.User:
		out, err := r.iam.GetUserPolicyWithContext(ctx, &iam.GetUserPolicyInput{
			PolicyName: aws.String(r.cfg.policyName),
			UserName:   aws.String(r.cfg.identity.GetName()),
		})
		if err != nil {
			if trace.IsNotFound(awslib.ConvertIAMError(err)) {
				return awslib.NewPolicyDocument(), nil
			}
			return nil, awslib.ConvertIAMError(err)
		}
		policyDocument = aws.StringValue(out.PolicyDocument)
	default:
		return nil, trace.BadParameter("can only fetch policies for roles or users, got %v", r.cfg.identity)
	}
	return awslib.ParsePolicyDocument(policyDocument)
}

// updateIAMPolicy attaches IAM access policy to the identity this agent is running as.
func (r *awsClient) updateIAMPolicy(ctx context.Context, policy *awslib.PolicyDocument) error {
	r.log.Debugf("Updating IAM policy for %v.", r.cfg.identity)
	document, err := json.Marshal(policy)
	if err != nil {
		return trace.Wrap(err)
	}
	switch r.cfg.identity.(type) {
	case awslib.Role:
		_, err = r.iam.PutRolePolicyWithContext(ctx, &iam.PutRolePolicyInput{
			PolicyName:     aws.String(r.cfg.policyName),
			PolicyDocument: aws.String(string(document)),
			RoleName:       aws.String(r.cfg.identity.GetName()),
		})
	case awslib.User:
		_, err = r.iam.PutUserPolicyWithContext(ctx, &iam.PutUserPolicyInput{
			PolicyName:     aws.String(r.cfg.policyName),
			PolicyDocument: aws.String(string(document)),
			UserName:       aws.String(r.cfg.identity.GetName()),
		})
	default:
		return trace.BadParameter("can only update policies for roles or users, got %v", r.cfg.identity)
	}
	return awslib.ConvertIAMError(err)
}

// detachIAMPolicy detaches IAM access policy from the identity this agent is running as.
func (r *awsClient) detachIAMPolicy(ctx context.Context) error {
	r.log.Debugf("Detaching IAM policy from %v.", r.cfg.identity)
	var err error
	switch r.cfg.identity.(type) {
	case awslib.Role:
		_, err = r.iam.DeleteRolePolicyWithContext(ctx, &iam.DeleteRolePolicyInput{
			PolicyName: aws.String(r.cfg.policyName),
			RoleName:   aws.String(r.cfg.identity.GetName()),
		})
	case awslib.User:
		_, err = r.iam.DeleteUserPolicyWithContext(ctx, &iam.DeleteUserPolicyInput{
			PolicyName: aws.String(r.cfg.policyName),
			UserName:   aws.String(r.cfg.identity.GetName()),
		})
	default:
		return trace.BadParameter("can only detach policies from roles or users, got %v", r.cfg.identity)
	}
	return awslib.ConvertIAMError(err)
}

type rdsDBConfigurator struct {
	clients cloud.Clients
	log     logrus.FieldLogger
}

// ensureIAMAuth enables RDS instance IAM auth if it isn't already enabled.
func (r *rdsDBConfigurator) ensureIAMAuth(ctx context.Context, db types.Database) error {
	if db.GetAWS().RDS.IAMAuth {
		r.log.Debug("IAM auth already enabled.")
		return nil
	}
	if err := r.enableIAMAuth(ctx, db); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// enableIAMAuth turns on IAM auth setting on the RDS instance.
func (r *rdsDBConfigurator) enableIAMAuth(ctx context.Context, db types.Database) error {
	r.log.Debug("Enabling IAM auth for RDS.")
	meta := db.GetAWS()
	rdsClt, err := r.clients.GetAWSRDSClient(ctx, meta.Region,
		cloud.WithAssumeRoleFromAWSMeta(meta),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	if meta.RDS.ClusterID != "" {
		_, err = rdsClt.ModifyDBClusterWithContext(ctx, &rds.ModifyDBClusterInput{
			DBClusterIdentifier:             aws.String(meta.RDS.ClusterID),
			EnableIAMDatabaseAuthentication: aws.Bool(true),
			ApplyImmediately:                aws.Bool(true),
		})
		return awslib.ConvertIAMError(err)
	}
	if meta.RDS.InstanceID != "" {
		_, err = rdsClt.ModifyDBInstanceWithContext(ctx, &rds.ModifyDBInstanceInput{
			DBInstanceIdentifier:            aws.String(meta.RDS.InstanceID),
			EnableIAMDatabaseAuthentication: aws.Bool(true),
			ApplyImmediately:                aws.Bool(true),
		})
		return awslib.ConvertIAMError(err)
	}
	return trace.BadParameter("no RDS cluster ID or instance ID for %v", db)
}

type nopDBConfigurator struct{}

// ensureIAMAuth is a no-op.
func (c *nopDBConfigurator) ensureIAMAuth(context.Context, types.Database) error {
	return nil
}
