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
	"fmt"
	"slices"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// credentialsChecker performs some quick checks to see whether this database
// agent can handle the incoming database wrt to the agent's credentials.
//
// Note that this checker warns the user with suggestions on how to configure
// the credentials correctly instead of returning errors.
type credentialsChecker struct {
	cloudClients     cloud.Clients
	resourceMatchers []services.ResourceMatcher
	log              logrus.FieldLogger
	cache            *utils.FnCache
}

func newCrednentialsChecker(cfg DiscoveryResourceCheckerConfig) (*credentialsChecker, error) {
	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:     10 * time.Minute,
		Context: cfg.Context,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &credentialsChecker{
		cloudClients:     cfg.Clients,
		resourceMatchers: cfg.ResourceMatchers,
		log:              cfg.Log,
		cache:            cache,
	}, nil
}

// Check performs some quick checks to see whether this database agent can
// handle the incoming database wrt to the agent's credentials.
func (c *credentialsChecker) Check(ctx context.Context, database types.Database) error {
	switch {
	case database.IsAWSHosted():
		c.checkAWS(ctx, database)
	case database.IsAzure():
		c.checkAzure(ctx, database)
	default:
		c.log.Debugf("Database %q has unknown cloud type %q.", database.GetName(), database.GetType())
	}
	return nil
}

func (c *credentialsChecker) checkAWS(ctx context.Context, database types.Database) {
	meta := database.GetAWS()
	identity, err := c.getAWSIdentity(ctx, &meta)
	if err != nil {
		c.warn(err, database, "Failed to get AWS identity when checking a database created by the discovery service.")
		return
	}

	if meta.AccountID != "" && meta.AccountID != identity.GetAccountID() {
		c.warn(nil, database, fmt.Sprintf("The database agent's identity and discovered database %q have different AWS account IDs (%s vs %s).",
			database.GetName(),
			identity.GetAccountID(),
			meta.AccountID,
		))
		return
	}
}

// getAWSIdentity returns the identity used to access the given database,
// that is either the agent's identity or the database's configured assume-role.
func (c *credentialsChecker) getAWSIdentity(ctx context.Context, meta *types.AWS) (aws.Identity, error) {
	if meta.AssumeRoleARN != "" {
		// If the database has an assume role ARN, use that instead of
		// agent identity. This avoids an unnecessary sts call too.
		return aws.IdentityFromArn(meta.AssumeRoleARN)
	}

	identity, err := utils.FnCacheGet(ctx, c.cache, types.CloudAWS, func(ctx context.Context) (aws.Identity, error) {
		client, err := c.cloudClients.GetAWSSTSClient(ctx, "", cloud.WithAmbientCredentials())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return aws.GetIdentityWithClient(ctx, client)
	})
	return identity, trace.Wrap(err)
}

func (c *credentialsChecker) checkAzure(ctx context.Context, database types.Database) {
	allSubIDs, err := utils.FnCacheGet(ctx, c.cache, types.CloudAzure, func(ctx context.Context) ([]string, error) {
		client, err := c.cloudClients.GetAzureSubscriptionClient()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return client.ListSubscriptionIDs(ctx)
	})
	if err != nil {
		c.warn(err, database, "Failed to get Azure subscription IDs when checking a database created by the discovery service.")
		return
	}

	rid, err := arm.ParseResourceID(database.GetAzure().ResourceID)
	if err != nil {
		c.log.Warnf("Failed to parse resource ID of database %q: %v.", database.GetName(), err)
		return
	}

	if !slices.Contains(allSubIDs, rid.SubscriptionID) {
		c.warn(nil, database, fmt.Sprintf("The discovered database %q is in a subscription (ID: %s) that the database agent does not have access to.",
			database.GetName(),
			rid.SubscriptionID,
		))
		return
	}
}

func (c *credentialsChecker) warn(err error, database types.Database, msg string) {
	log := c.log.WithField("database", database)
	if err != nil {
		log = log.WithField("error", err.Error())
	}

	logLevel := logrus.InfoLevel
	if c.isWildcardMatcher() {
		logLevel = logrus.WarnLevel
	}
	log.Logf(logLevel, "%s You can update \"db_service.resources\" section of this agent's config file to filter out unwanted resources (see https://goteleport.com/docs/database-access/reference/configuration/ for more details). If this database is intended to be handled by this agent, please verify that valid cloud credentials are configured for the agent.", msg)
}

func (c *credentialsChecker) isWildcardMatcher() bool {
	if len(c.resourceMatchers) != 1 {
		return false
	}

	wildcardLabels := c.resourceMatchers[0].Labels[types.Wildcard]
	return len(wildcardLabels) == 1 && wildcardLabels[0] == types.Wildcard
}
