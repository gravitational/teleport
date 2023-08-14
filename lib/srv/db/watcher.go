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

package db

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	clients "github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/services"
	discovery "github.com/gravitational/teleport/lib/srv/discovery/common"
	dbfetchers "github.com/gravitational/teleport/lib/srv/discovery/fetchers/db"
	"github.com/gravitational/teleport/lib/utils"
)

// startReconciler starts reconciler that registers/unregisters proxied
// databases according to the up-to-date list of database resources and
// databases imported from the cloud.
func (s *Server) startReconciler(ctx context.Context) error {
	reconciler, err := services.NewReconciler(services.ReconcilerConfig{
		Matcher:             s.matcher,
		GetCurrentResources: s.getResources,
		GetNewResources:     s.monitoredDatabases.get,
		OnCreate:            s.onCreate,
		OnUpdate:            s.onUpdate,
		OnDelete:            s.onDelete,
		Log:                 s.log,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		for {
			select {
			case <-s.reconcileCh:
				if err := reconciler.Reconcile(ctx); err != nil {
					s.log.WithError(err).Error("Failed to reconcile.")
				} else if s.cfg.OnReconcile != nil {
					s.cfg.OnReconcile(s.getProxiedDatabases())
				}
			case <-ctx.Done():
				s.log.Debug("Reconciler done.")
				return
			}
		}
	}()
	return nil
}

// startResourceWatcher starts watching changes to database resources and
// registers/unregisters the proxied databases accordingly.
func (s *Server) startResourceWatcher(ctx context.Context) (*services.DatabaseWatcher, error) {
	if len(s.cfg.ResourceMatchers) == 0 {
		s.log.Debug("Not starting database resource watcher.")
		return nil, nil
	}
	s.log.Debug("Starting database resource watcher.")
	watcher, err := services.NewDatabaseWatcher(ctx, services.DatabaseWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentDatabase,
			Log:       s.log,
			Client:    s.cfg.AccessPoint,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go func() {
		defer s.log.Debug("Database resource watcher done.")
		defer watcher.Close()
		for {
			select {
			case databases := <-watcher.DatabasesC:
				// Overwrite database specs like AssumeRoleARN before reconcile.
				applyResourceMatchersToDatabases(databases, s.cfg.ResourceMatchers)
				s.monitoredDatabases.setResources(databases)
				select {
				case s.reconcileCh <- struct{}{}:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return watcher, nil
}

// startCloudWatcher starts fetching cloud databases according to the
// selectors and register/unregister them appropriately.
func (s *Server) startCloudWatcher(ctx context.Context) error {
	awsFetchers, err := dbfetchers.MakeAWSFetchers(ctx, s.cfg.CloudClients, s.cfg.AWSMatchers)
	if err != nil {
		return trace.Wrap(err)
	}
	azureFetchers, err := dbfetchers.MakeAzureFetchers(s.cfg.CloudClients, s.cfg.AzureMatchers)
	if err != nil {
		return trace.Wrap(err)
	}

	watcher, err := discovery.NewWatcher(ctx, discovery.WatcherConfig{
		Fetchers: append(awsFetchers, azureFetchers...),
		Log:      logrus.WithField(trace.Component, "watcher:cloud"),
	})
	if err != nil {
		if trace.IsNotFound(err) {
			s.log.Debugf("Not starting cloud database watcher: %v.", err)
			return nil
		}
		return trace.Wrap(err)
	}
	go watcher.Start()
	go func() {
		defer s.log.Debug("Cloud database watcher done.")
		for {
			select {
			case resources := <-watcher.ResourcesC():
				databases, err := resources.AsDatabases()
				if err == nil {
					s.monitoredDatabases.setCloud(databases)
				} else {
					s.log.WithError(err).Warnf("Failed to convert resources to databases.")
				}
				select {
				case s.reconcileCh <- struct{}{}:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

// getResources returns proxied databases as resources.
func (s *Server) getResources() types.ResourcesWithLabelsMap {
	return s.getProxiedDatabases().AsResources().ToMap()
}

// onCreate is called by reconciler when a new database is created.
func (s *Server) onCreate(ctx context.Context, resource types.ResourceWithLabels) error {
	database, ok := resource.(types.Database)
	if !ok {
		return trace.BadParameter("expected types.Database, got %T", resource)
	}

	if s.monitoredDatabases.isDiscoveryResource(database) {
		s.cfg.discoveryResourceChecker.check(ctx, database)
	}
	return s.registerDatabase(ctx, database)
}

// onUpdate is called by reconciler when an already proxied database is updated.
func (s *Server) onUpdate(ctx context.Context, resource types.ResourceWithLabels) error {
	database, ok := resource.(types.Database)
	if !ok {
		return trace.BadParameter("expected types.Database, got %T", resource)
	}
	return s.updateDatabase(ctx, database)
}

// onDelete is called by reconciler when a proxied database is deleted.
func (s *Server) onDelete(ctx context.Context, resource types.ResourceWithLabels) error {
	database, ok := resource.(types.Database)
	if !ok {
		return trace.BadParameter("expected types.Database, got %T", resource)
	}
	return s.unregisterDatabase(ctx, database)
}

// matcher is used by reconciler to check if database matches selectors.
func (s *Server) matcher(resource types.ResourceWithLabels) bool {
	database, ok := resource.(types.Database)
	if !ok {
		return false
	}

	// In the case of databases discovered by this database server, matchers
	// should be skipped.
	if s.monitoredDatabases.isCloud(database) {
		return true // Cloud fetchers return only matching databases.
	}

	// Database resources created via CLI, API, or discovery service are
	// filtered by resource matchers.
	return services.MatchResourceLabels(s.cfg.ResourceMatchers, database)
}

// discoveryResourceChecker defines an interface for checking database
// resources created by the discovery service.
type discoveryResourceChecker interface {
	// check performs required checks on provided database resource before it
	// gets registered.
	check(ctx context.Context, database types.Database)
}

// cloudCredentialsChecker is a discoveryResourceChecker for validating cloud
// credentials against the incoming discovery resources.
type cloudCredentialsChecker struct {
	cloudClients     clients.Clients
	resourceMatchers []services.ResourceMatcher
	log              *logrus.Entry
	cache            *utils.FnCache
}

func newCloudCrednentialsChecker(ctx context.Context, cloudClients clients.Clients, resourceMatchers []services.ResourceMatcher) (discoveryResourceChecker, error) {
	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:     10 * time.Minute,
		Context: ctx,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &cloudCredentialsChecker{
		cloudClients:     cloudClients,
		resourceMatchers: resourceMatchers,
		log:              logrus.WithField(trace.Component, teleport.ComponentDatabase),
		cache:            cache,
	}, nil
}

// check performs some quick checks to see whether this database agent can handle
// the incoming database (likely created by discovery service), and logs a
// warning with suggestions for this situation.
func (c *cloudCredentialsChecker) check(ctx context.Context, database types.Database) {
	if database.Origin() != types.OriginCloud {
		return
	}

	switch {
	case database.IsAWSHosted():
		c.checkAWS(ctx, database)
	case database.IsAzure():
		c.checkAzure(ctx, database)
	default:
		c.log.Debugf("Database %q has unknown cloud type %q.", database.GetName(), database.GetType())
	}
}

func (c *cloudCredentialsChecker) checkAWS(ctx context.Context, database types.Database) {
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
func (c *cloudCredentialsChecker) getAWSIdentity(ctx context.Context, meta *types.AWS) (aws.Identity, error) {
	if meta.AssumeRoleARN != "" {
		// If the database has an assume role ARN, use that instead of
		// agent identity. This avoids an unnecessary sts call too.
		return aws.IdentityFromArn(meta.AssumeRoleARN)
	}

	identity, err := utils.FnCacheGet(ctx, c.cache, types.CloudAWS, func(ctx context.Context) (aws.Identity, error) {
		client, err := c.cloudClients.GetAWSSTSClient(ctx, "")
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return aws.GetIdentityWithClient(ctx, client)
	})
	return identity, trace.Wrap(err)
}

func (c *cloudCredentialsChecker) checkAzure(ctx context.Context, database types.Database) {
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

func (c *cloudCredentialsChecker) warn(err error, database types.Database, msg string) {
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

func (c *cloudCredentialsChecker) isWildcardMatcher() bool {
	if len(c.resourceMatchers) != 1 {
		return false
	}

	wildcardLabels := c.resourceMatchers[0].Labels[types.Wildcard]
	return len(wildcardLabels) == 1 && wildcardLabels[0] == types.Wildcard
}

func applyResourceMatchersToDatabases(databases types.Databases, resourceMatchers []services.ResourceMatcher) {
	for _, database := range databases {
		applyResourceMatcherToDatabase(database, resourceMatchers)
	}
}

func applyResourceMatcherToDatabase(database types.Database, resourceMatchers []services.ResourceMatcher) {
	for _, matcher := range resourceMatchers {
		if len(matcher.Labels) == 0 || matcher.AWS.AssumeRoleARN == "" {
			continue
		}
		if match, _, _ := services.MatchLabels(matcher.Labels, database.GetAllLabels()); !match {
			continue
		}

		database.SetAWSAssumeRole(matcher.AWS.AssumeRoleARN)
		database.SetAWSExternalID(matcher.AWS.ExternalID)
	}
}
