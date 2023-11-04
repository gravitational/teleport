/*
Copyright 2023 Gravitational, Inc.

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

package mongodb

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	mongooptions "go.mongodb.org/mongo-driver/mongo/options"
	mongoauth "go.mongodb.org/mongo-driver/x/mongo/driver/auth"

	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/utils"
)

// adminClient defines a subset of functions provided by *mongo.Client.
type adminClient interface {
	Database(name string, opts ...*options.DatabaseOptions) *mongo.Database
	Disconnect(ctx context.Context) error
}

func (e *Engine) connectAsAdmin(ctx context.Context, sessionCtx *common.Session) (adminClient, error) {
	if adminClientFnCache == nil {
		client, err := makeRawAdminClient(ctx, sessionCtx, e)
		return client, trace.Wrap(err)
	}

	sharedClient, err := getSharedAdminClient(ctx, adminClientFnCache, sessionCtx, e, makeRawAdminClient)
	return sharedClient, trace.Wrap(err)
}

func getSharedAdminClient(ctx context.Context, cache *utils.FnCache, sessionCtx *common.Session, e *Engine, makeRaw makeRawAdminClientFunc) (*sharedAdminClient, error) {
	key := struct {
		databaseName string
		databaseURI  string
		adminUser    string
	}{
		databaseName: sessionCtx.Database.GetName(),
		adminUser:    sessionCtx.Database.GetAdminUser().Name,
		// Just in case the same database name is updated to a different database server.
		databaseURI: sessionCtx.Database.GetURI(),
	}

	sharedClient, err := utils.FnCacheGet(ctx, cache, key, func(ctx context.Context) (*sharedAdminClient, error) {
		rawClient, err := makeRaw(ctx, sessionCtx, e)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		sharedClient, err := newSharedAdminClient(sharedAdminClientConfig{
			rawClient:    rawClient,
			adminUser:    sessionCtx.Database.GetAdminUser().Name,
			databaseName: sessionCtx.Database.GetName(),
			log:          e.Log,
			clock:        e.Clock,
		})
		return sharedClient, trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sharedClient.acquire(), nil
}

type sharedAdminClientConfig struct {
	rawClient    adminClient
	adminUser    string
	databaseName string
	clock        clockwork.Clock
	log          logrus.FieldLogger
	cleanupTTL   time.Duration
}

// checkAndSetDefaults validates config and sets defaults.
func (c *sharedAdminClientConfig) checkAndSetDefaults() error {
	if c.rawClient == nil {
		return trace.BadParameter("missing mongo.Client")
	}
	if c.adminUser == "" {
		return trace.BadParameter("missing adminUser")
	}
	if c.databaseName == "" {
		return trace.BadParameter("missing databaseName")
	}
	if c.clock == nil {
		c.clock = clockwork.NewRealClock()
	}
	if c.log == nil {
		c.log = logrus.StandardLogger()
	}
	if c.cleanupTTL <= 0 {
		c.cleanupTTL = adminClientCleanupTTL
	}
	return nil
}

type sharedAdminClient struct {
	adminClient
	sharedAdminClientConfig
	wg    sync.WaitGroup
	timer clockwork.Timer
}

func newSharedAdminClient(cfg sharedAdminClientConfig) (*sharedAdminClient, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	c := &sharedAdminClient{
		adminClient:             cfg.rawClient,
		sharedAdminClientConfig: cfg,
		timer:                   cfg.clock.NewTimer(cfg.cleanupTTL),
	}

	go c.waitAndCleanup()
	return c, nil
}

func (c *sharedAdminClient) waitAndCleanup() {
	c.log.Debugf("Created new MongoDB connection as admin user %q on %q.", c.adminUser, c.databaseName)

	// Wait until TTL.
	<-c.timer.Chan()

	// Wait until all shared sessions are released.
	c.wg.Wait()

	// Disconnect.
	if err := c.adminClient.Disconnect(context.Background()); err != nil {
		c.log.Warnf("Failed to disconnect MongoDB connection as admin user %q on %q: %v.", c.adminUser, c.databaseName, err)
	} else {
		c.log.Debugf("Terminated a MongoDB connection as admin user %q on %q.", c.adminUser, c.databaseName)
	}
}

func (c *sharedAdminClient) acquire() *sharedAdminClient {
	c.wg.Add(1)
	return c
}

func (c *sharedAdminClient) Disconnect(_ context.Context) error {
	c.wg.Done()
	return nil
}

type makeRawAdminClientFunc func(context.Context, *common.Session, *Engine) (adminClient, error)

func makeRawAdminClient(ctx context.Context, sessionCtx *common.Session, e *Engine) (adminClient, error) {
	sessionCtx = sessionCtx.WithUser(sessionCtx.Database.GetAdminUser().Name)

	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientCfg, err := makeClientOptionsFromDatabaseURI(sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clientCfg.SetTLSConfig(tlsConfig)
	clientCfg.SetAuth(mongooptions.Credential{
		AuthMechanism: mongoauth.MongoDBX509,
		Username:      x509Username(sessionCtx),
	})

	client, err := mongo.Connect(ctx, clientCfg)
	return client, trace.Wrap(err)
}

const (
	// adminClientFnCacheTTL specifies how long the sharedAdminClient will be
	// cached in FnCache.
	adminClientFnCacheTTL = time.Minute
	// adminClientCleanupTTL specifies how long to wait to terminate the
	// sharedAdminClient. Use a longer TTL than adminClientFnCacheTTL to make
	// sure the sharedAdminClient will be not returned by the FnCache while
	// getting terminated.
	adminClientCleanupTTL = adminClientFnCacheTTL + time.Second*10
)

var adminClientFnCache *utils.FnCache

func init() {
	if os.Getenv("TELEPORT_DISABLE_MONGODB_ADMIN_CLIENT_CACHE") != "" {
		return
	}

	var err error
	adminClientFnCache, err = utils.NewFnCache(utils.FnCacheConfig{
		TTL: adminClientFnCacheTTL,
	})
	// This should never fail. There is also an unit test to verify it's always
	// created. Logging error just in case.
	if err != nil {
		logrus.Warnf("Failed to create MongoDB admin connection cache: %v.", err)
	}
}
