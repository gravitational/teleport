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
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	dbiam "github.com/gravitational/teleport/lib/srv/db/common/iam"
)

// awsConfig is the config for the client that configures IAM for AWS databases.
type awsConfig struct {
	// awsConfigProvider provides [aws.Config] for AWS SDK service clients.
	awsConfigProvider awsconfig.Provider
	// identity is AWS identity this database agent is running as.
	identity awslib.Identity
	// database is the database instance to configure.
	database types.Database
	// policyName is the name of the inline policy for the identity.
	policyName string
	// awsClients is an internal-only AWS SDK client provider that is
	// only set in tests.
	awsClients awsClientProvider
}

// Check validates the config.
func (c *awsConfig) Check() error {
	if c.identity == nil {
		return trace.BadParameter("missing parameter identity")
	}
	if c.database == nil {
		return trace.BadParameter("missing parameter database")
	}
	if c.policyName == "" {
		return trace.BadParameter("missing parameter policy name")
	}
	if c.awsConfigProvider == nil {
		return trace.BadParameter("missing parameter awsConfigProvider")
	}
	if c.awsClients == nil {
		return trace.BadParameter("missing parameter awsClients")
	}
	return nil
}

// newAWS creates a new AWS IAM configurator.
func newAWS(ctx context.Context, config awsConfig) (*awsClient, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	logger := slog.With(
		teleport.ComponentKey, "aws",
		"db", config.database.GetName(),
	)
	dbConfigurator, err := getDBConfigurator(logger, config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	meta := config.database.GetAWS()
	awsCfg, err := config.awsConfigProvider.GetConfig(ctx, meta.Region,
		awsconfig.WithAssumeRole(meta.AssumeRoleARN, meta.ExternalID),
		awsconfig.WithAmbientCredentials(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	iamClt := config.awsClients.getIAMClient(awsCfg)
	return &awsClient{
		cfg:            config,
		dbConfigurator: dbConfigurator,
		iam:            iamClt,
		logger:         logger,
	}, nil
}

type dbIAMAuthConfigurator interface {
	// ensureIAMAuth enables DB IAM auth if it isn't already enabled.
	ensureIAMAuth(context.Context, types.Database) error
}

// getDBConfigurator returns a database IAM Auth configurator.
func getDBConfigurator(logger *slog.Logger, cfg awsConfig) (dbIAMAuthConfigurator, error) {
	if cfg.database.IsRDS() {
		// Only setting for RDS instances and Aurora clusters.
		return &rdsDBConfigurator{
			awsConfigProvider: cfg.awsConfigProvider,
			logger:            logger,
			awsClients:        cfg.awsClients,
		}, nil
	}
	// IAM Auth for Redshift, ElastiCache, and RDS Proxy is always enabled.
	return &nopDBConfigurator{}, nil
}

type awsClient struct {
	cfg            awsConfig
	dbConfigurator dbIAMAuthConfigurator
	iam            iamClient
	logger         *slog.Logger
}

// setupIAMAuth ensures the IAM Authentication is enbaled for RDS, Aurora, ElastiCache or Redshift database.
func (r *awsClient) setupIAMAuth(ctx context.Context) error {
	if err := r.dbConfigurator.ensureIAMAuth(ctx, r.cfg.database); err != nil {
		if trace.IsAccessDenied(err) { // Permission errors are expected.
			r.logger.DebugContext(ctx, "No permissions to enable IAM auth", "error", err)
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
			r.logger.DebugContext(ctx, "No permissions to ensure IAM policy", "error", err)
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
			r.logger.DebugContext(ctx, "No permissions to delete IAM policy", "error", err)
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
	dbIAM.ForEach(func(effect, action, resource string, conditions awslib.Conditions) {
		if policy.EnsureResourceAction(effect, action, resource, conditions) {
			r.logger.DebugContext(ctx, "Adding database permission to policy",
				"action", action,
				"resource", resource,
			)
			changed = true
		} else {
			r.logger.DebugContext(ctx, "Permission is already part of policy", "action", action, "resource", resource)
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
		r.logger.WarnContext(ctx, "Please make sure the database agent has the IAM permissions to fetch cloud metadata, or make sure these values are set in the static config. Placeholders were found when configuring the IAM policy for database.",
			"placeholders", placeholders,
			"database", r.cfg.database.GetName(),
		)
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
	dbIAM.ForEach(func(effect, action, resource string, conditions awslib.Conditions) {
		policy.DeleteResourceAction(effect, action, resource, conditions)
	})
	// If policy is empty now, delete it as IAM policy can't be empty.
	if policy.IsEmpty() {
		return r.detachIAMPolicy(ctx)
	}
	return r.updateIAMPolicy(ctx, policy)
}

// getIAMPolicy fetches and returns this agent's parsed IAM policy document.
func (r *awsClient) getIAMPolicy(ctx context.Context) (*awslib.PolicyDocument, error) {
	var policyDocument string
	switch r.cfg.identity.(type) {
	case awslib.Role:
		out, err := r.iam.GetRolePolicy(ctx, &iam.GetRolePolicyInput{
			PolicyName: aws.String(r.cfg.policyName),
			RoleName:   aws.String(r.cfg.identity.GetName()),
		})
		if err != nil {
			if trace.IsNotFound(awslib.ConvertIAMError(err)) {
				return awslib.NewPolicyDocument(), nil
			}
			return nil, awslib.ConvertIAMError(err)
		}
		policyDocument = aws.ToString(out.PolicyDocument)
	case awslib.User:
		out, err := r.iam.GetUserPolicy(ctx, &iam.GetUserPolicyInput{
			PolicyName: aws.String(r.cfg.policyName),
			UserName:   aws.String(r.cfg.identity.GetName()),
		})
		if err != nil {
			if trace.IsNotFound(awslib.ConvertIAMError(err)) {
				return awslib.NewPolicyDocument(), nil
			}
			return nil, awslib.ConvertIAMError(err)
		}
		policyDocument = aws.ToString(out.PolicyDocument)
	default:
		return nil, trace.BadParameter("can only fetch policies for roles or users, got %v", r.cfg.identity)
	}
	return awslib.ParsePolicyDocument(policyDocument)
}

// updateIAMPolicy attaches IAM access policy to the identity this agent is running as.
func (r *awsClient) updateIAMPolicy(ctx context.Context, policy *awslib.PolicyDocument) error {
	r.logger.DebugContext(ctx, "Updating IAM policy", "identity", r.cfg.identity)
	document, err := json.Marshal(policy)
	if err != nil {
		return trace.Wrap(err)
	}
	switch r.cfg.identity.(type) {
	case awslib.Role:
		_, err = r.iam.PutRolePolicy(ctx, &iam.PutRolePolicyInput{
			PolicyName:     aws.String(r.cfg.policyName),
			PolicyDocument: aws.String(string(document)),
			RoleName:       aws.String(r.cfg.identity.GetName()),
		})
	case awslib.User:
		_, err = r.iam.PutUserPolicy(ctx, &iam.PutUserPolicyInput{
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
	r.logger.DebugContext(ctx, "Detaching IAM policy", "identity", r.cfg.identity)
	var err error
	switch r.cfg.identity.(type) {
	case awslib.Role:
		_, err = r.iam.DeleteRolePolicy(ctx, &iam.DeleteRolePolicyInput{
			PolicyName: aws.String(r.cfg.policyName),
			RoleName:   aws.String(r.cfg.identity.GetName()),
		})
	case awslib.User:
		_, err = r.iam.DeleteUserPolicy(ctx, &iam.DeleteUserPolicyInput{
			PolicyName: aws.String(r.cfg.policyName),
			UserName:   aws.String(r.cfg.identity.GetName()),
		})
	default:
		return trace.BadParameter("can only detach policies from roles or users, got %v", r.cfg.identity)
	}
	return awslib.ConvertIAMError(err)
}

type rdsDBConfigurator struct {
	awsConfigProvider awsconfig.Provider
	logger            *slog.Logger
	awsClients        awsClientProvider
}

// ensureIAMAuth enables RDS instance IAM auth if it isn't already enabled.
func (r *rdsDBConfigurator) ensureIAMAuth(ctx context.Context, db types.Database) error {
	if db.GetAWS().RDS.IAMAuth {
		r.logger.DebugContext(ctx, "IAM auth already enabled")
		return nil
	}
	if err := r.enableIAMAuth(ctx, db); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// enableIAMAuth turns on IAM auth setting on the RDS instance.
func (r *rdsDBConfigurator) enableIAMAuth(ctx context.Context, db types.Database) error {
	r.logger.DebugContext(ctx, "Enabling IAM auth for RDS")
	meta := db.GetAWS()
	if meta.RDS.ClusterID == "" && meta.RDS.InstanceID == "" {
		return trace.BadParameter("no RDS cluster ID or instance ID for %v", db)
	}
	awsCfg, err := r.awsConfigProvider.GetConfig(ctx, meta.Region,
		awsconfig.WithAssumeRole(meta.AssumeRoleARN, meta.ExternalID),
		awsconfig.WithAmbientCredentials(),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	clt := r.awsClients.getRDSClient(awsCfg)
	if meta.RDS.ClusterID != "" {
		_, err = clt.ModifyDBCluster(ctx, &rds.ModifyDBClusterInput{
			DBClusterIdentifier:             aws.String(meta.RDS.ClusterID),
			EnableIAMDatabaseAuthentication: aws.Bool(true),
			ApplyImmediately:                aws.Bool(true),
		})
		return awslib.ConvertRequestFailureError(err)
	}
	if meta.RDS.InstanceID != "" {
		_, err = clt.ModifyDBInstance(ctx, &rds.ModifyDBInstanceInput{
			DBInstanceIdentifier:            aws.String(meta.RDS.InstanceID),
			EnableIAMDatabaseAuthentication: aws.Bool(true),
			ApplyImmediately:                aws.Bool(true),
		})
		return awslib.ConvertRequestFailureError(err)
	}
	return nil
}

type nopDBConfigurator struct{}

// ensureIAMAuth is a no-op.
func (c *nopDBConfigurator) ensureIAMAuth(context.Context, types.Database) error {
	return nil
}
