/*
Copyright 2022 Gravitational, Inc.

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
package users

import (
	"context"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

// Config is the config for users service.
type Config struct {
	// Clients is an interface for retrieving cloud clients.
	Clients common.CloudClients
	// Clock is used to control time.
	Clock clockwork.Clock
	// Interval is the interval between user updates. Interval is also used as
	// the minimum password expiration duration.
	Interval time.Duration
	// Log is the logrus field logger.
	Log logrus.FieldLogger
}

// CheckAndSetDefaults validates the config and set defaults.
func (c *Config) CheckAndSetDefaults() (err error) {
	if c.Clients == nil {
		c.Clients = common.NewCloudClients()
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.Interval == 0 {
		// A AWS Secrets Manager secret can have at most 100 versions per day
		// (about one new version per 15 minutes).
		//
		// https://docs.aws.amazon.com/secretsmanager/latest/userguide/reference_limits.html
		//
		// Note that currently all database types are sharing the same interval
		// for password rotations.
		c.Interval = 15 * time.Minute
	}
	if c.Log == nil {
		c.Log = logrus.WithField(trace.Component, "cloudusers")
	}
	return nil
}

// Users manages database users for cloud databases.
type Users struct {
	cfg               Config
	fetchersByType    map[string]Fetcher
	usersByID         map[string]User
	lookup            *lookupMap
	setupDatabaseChan chan types.Database
}

// Fetcher fetches database users for a particular database type.
type Fetcher interface {
	// GetType returns the database type of the fetcher.
	GetType() string
	// FetchDatabaseUsers fetches users for provided database.
	FetchDatabaseUsers(ctx context.Context, database types.Database) ([]User, error)
}

// NewUsers returns a new instance of users service.
func NewUsers(cfg Config) (*Users, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	fetchersByType, err := makeFetchers(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Users{
		cfg:               cfg,
		fetchersByType:    fetchersByType,
		lookup:            newLookupMap(),
		usersByID:         make(map[string]User),
		setupDatabaseChan: make(chan types.Database),
	}, nil
}

// GetPassword returns the password for database login.
func (u *Users) GetPassword(ctx context.Context, database types.Database, username string) (string, error) {
	user, found := u.lookup.getDatabaseUser(database, username)
	if !found {
		return "", trace.NotFound("database user %s is not managed", username)
	}

	return user.GetPassword(ctx)
}

// Setup starts to discover and manage users for provided database.
//
// Setup allows managed database users to become available as soon as new
// database is registered instead of waiting for the periodic setup goroutine.
// Note that there is no corresponding "Teardown" as cleanup will eventually
// happen in the periodic setup.
func (u *Users) Setup(_ context.Context, database types.Database) error {
	_, found := u.fetchersByType[database.GetType()]
	if !found {
		return nil
	}

	u.setupDatabaseChan <- database
	return nil
}

// Start starts users service to manage cloud database users.
func (u *Users) Start(ctx context.Context, getAllDatabases func() types.Databases) {
	u.cfg.Log.Debug("Starting cloud users service.")
	defer u.cfg.Log.Debug("Cloud users service done.")

	ticker := interval.New(interval.Config{
		// Use jitter for HA setups.
		Jitter: utils.NewSeventhJitter(),

		// NewSeventhJitter builds a new jitter on the range [6n/7,n).
		// Use n = cfg.Interval*7/6 gives an effective duration range of
		// [cfg.Interval, cfg.Interval*7/6), to ensure minimum is cfg.Interval.
		// The extra jitter also helps offset small clock skews.
		Duration: u.cfg.Interval * 7 / 6,
	})
	defer ticker.Stop()

	for {
		select {
		case database := <-u.setupDatabaseChan:
			u.setupDatabase(ctx, database)

		case <-ticker.Next():
			u.setupAllDatabases(ctx, getAllDatabases())

		case <-ctx.Done():
			return
		}
	}
}

// setupDatabase performs setup for a single database.
func (u *Users) setupDatabase(ctx context.Context, database types.Database) {
	u.setupDatabases(ctx, types.Databases{database})
}

// setupAllDatabases performs setup for all databases.
func (u *Users) setupAllDatabases(ctx context.Context, allDatabases types.Databases) {
	u.setupDatabases(ctx, allDatabases)

	// Clean up.
	u.lookup.removeUnusedDatabases(allDatabases)
	activeUsers := u.lookup.usersByID()
	for userID, user := range u.usersByID {
		if _, found := activeUsers[userID]; !found {
			delete(u.usersByID, userID)

			if err := user.Teardown(ctx); err != nil {
				u.cfg.Log.WithError(err).Errorf("Failed to tear down user %v.", user)
			}
		}
	}
}

// setupDatabases performs setup for provided databases.
func (u *Users) setupDatabases(ctx context.Context, databases types.Databases) {
	for _, database := range databases {
		// Fetch users.
		fetcher, found := u.fetchersByType[database.GetType()]
		if !found {
			continue
		}

		fetchedUsers, err := fetcher.FetchDatabaseUsers(ctx, database)
		if err != nil {
			if trace.IsAccessDenied(err) { // Permission errors are expected.
				u.cfg.Log.WithError(err).Debugf("No permissions to fetch users for %q.", database)
			} else {
				u.cfg.Log.WithError(err).Errorf("Failed to fetch users for database %v.", database)
			}
			continue
		}

		// Setup users.
		var users []User
		for _, fetchedUser := range fetchedUsers {
			if user, err := u.setupUser(ctx, fetchedUser); err != nil {
				u.cfg.Log.WithError(err).Errorf("Failed to setup user %v for database %v.", fetchedUser, database)
			} else {
				users = append(users, user)
			}
		}

		// Rotate passwords.
		for _, user := range users {
			if err = user.RotatePassword(ctx); err != nil {
				u.cfg.Log.WithError(err).Errorf("Failed to rotate password for user %v", user)
			}
		}

		// Update lookup.
		u.lookup.setDatabaseUsers(database, users)
	}
}

// setupUser finds existing user if it is already managed and tracked,
// otherwise try to setup the new user.
func (u *Users) setupUser(ctx context.Context, user User) (User, error) {
	if existingUser, found := u.usersByID[user.GetID()]; found {
		// TODO(greedy52) may want to compare secret store setting in case they
		// are different.
		return existingUser, nil
	}

	if err := user.Setup(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	u.usersByID[user.GetID()] = user
	return user, nil
}

// makeFetchers create a map of fetchers by their types.
func makeFetchers(cfg Config) (map[string]Fetcher, error) {
	newFetcherFuncs := []func(Config) (Fetcher, error){
		newElastiCacheFetcher,
	}

	fetchersByType := make(map[string]Fetcher)
	for _, newFetcherFunc := range newFetcherFuncs {
		fetcher, err := newFetcherFunc(cfg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		fetchersByType[fetcher.GetType()] = fetcher
	}
	return fetchersByType, nil
}
