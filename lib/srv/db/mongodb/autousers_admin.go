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

package mongodb

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	mongoauth "go.mongodb.org/mongo-driver/x/mongo/driver/auth"

	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/utils"
)

// adminClient defines a subset of functions provided by *mongo.Client plus some
// helper functions.
type adminClient interface {
	// Database returns a database handle based on database name.
	Database(name string, opts ...*options.DatabaseOptions) *mongo.Database
	// Disconnect disconnects the client.
	Disconnect(ctx context.Context) error
	// ServerVersion returns the server version.
	ServerVersion(ctx context.Context) (*semver.Version, error)
}

func (e *Engine) connectAsAdmin(ctx context.Context, sessionCtx *common.Session) (adminClient, error) {
	if isAdminClientCacheDisabled() || adminClientFnCache == nil {
		client, err := makeBasicAdminClient(ctx, sessionCtx, e)
		return client, trace.Wrap(err)
	}

	// The shared client will be cached for a short period. The shared client
	// will be terminated when the cache expires and all callers released the
	// shared client.
	//
	// The shared client (and mongo client in general) is goroutine-safe and
	// handles reconnection in case of network issues.
	//
	// During a CA rotation, the cached client may have the wrong TLS config,
	// but the cache should expire shortly so a new shared client will be
	// created.
	shareableClient, err := getShareableAdminClient(ctx, adminClientFnCache, sessionCtx, e, makeBasicAdminClient)
	return shareableClient, trace.Wrap(err)
}

func getShareableAdminClient(ctx context.Context, cache *utils.FnCache, sessionCtx *common.Session, e *Engine, makeBasicClient makeBasicAdminClientFunc) (*shareableAdminClient, error) {
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

	shareableClient, err := utils.FnCacheGet(ctx, cache, key, func(ctx context.Context) (*shareableAdminClient, error) {
		rawClient, err := makeBasicClient(ctx, sessionCtx, e)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		shareableClient, err := newShareableAdminClient(shareableAdminClientConfig{
			rawClient:    rawClient,
			adminUser:    sessionCtx.Database.GetAdminUser().Name,
			databaseName: sessionCtx.Database.GetName(),
			log:          e.Log,
			clock:        e.Clock,
			context:      ctx,
		})
		return shareableClient, trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return shareableClient.acquire(), nil
}

type shareableAdminClientConfig struct {
	rawClient    adminClient
	adminUser    string
	databaseName string
	clock        clockwork.Clock
	log          *slog.Logger
	cleanupTTL   time.Duration
	context      context.Context
}

// checkAndSetDefaults validates config and sets defaults.
func (c *shareableAdminClientConfig) checkAndSetDefaults() error {
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
		c.log = slog.Default()
	}
	if c.cleanupTTL <= 0 {
		c.cleanupTTL = adminClientCleanupTTL
	}
	return nil
}

type shareableAdminClient struct {
	adminClient
	shareableAdminClientConfig
	wg    sync.WaitGroup
	timer clockwork.Timer
}

func newShareableAdminClient(cfg shareableAdminClientConfig) (*shareableAdminClient, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	c := &shareableAdminClient{
		adminClient:                cfg.rawClient,
		shareableAdminClientConfig: cfg,
		timer:                      cfg.clock.NewTimer(cfg.cleanupTTL),
	}

	go c.waitAndCleanup()
	return c, nil
}

func (c *shareableAdminClient) waitAndCleanup() {
	c.log.DebugContext(c.context, "Created new MongoDB admin connection.", "user", c.adminUser, "database", c.databaseName)

	// Wait until TTL.
	<-c.timer.Chan()

	// Wait until all shared sessions are released.
	c.wg.Wait()

	// Disconnect connection. This happens after cache item expired to ensure
	// that shared client won't be reused when wrapped client was disconnected.
	if err := c.adminClient.Disconnect(context.Background()); err != nil {
		c.log.WarnContext(c.context, "Failed to disconnect MongoDB admin connection.", "user", c.adminUser, "database", c.databaseName, "error", err)
	} else {
		c.log.DebugContext(c.context, "Terminated a MongoDB admin connection.", "user", c.adminUser, "database", c.databaseName)
	}
}

func (c *shareableAdminClient) acquire() *shareableAdminClient {
	// acquire() should only be called while the client is still cached (while
	// waitAndCleanup waits for the timer).
	c.wg.Add(1)
	return c
}

func (c *shareableAdminClient) Disconnect(_ context.Context) error {
	c.wg.Done()
	return nil
}

type makeBasicAdminClientFunc func(context.Context, *common.Session, *Engine) (adminClient, error)

func makeBasicAdminClient(ctx context.Context, sessionCtx *common.Session, e *Engine) (adminClient, error) {
	sessionCtx = sessionCtx.WithUser(sessionCtx.Database.GetAdminUser().Name)

	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx.GetExpiry(), sessionCtx.Database, sessionCtx.DatabaseUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientCfg, err := makeClientOptionsFromDatabaseURI(sessionCtx.Database.GetURI())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clientCfg.SetTLSConfig(tlsConfig)
	clientCfg.SetAuth(options.Credential{
		AuthMechanism: mongoauth.MongoDBX509,
		Username:      x509Username(sessionCtx),
	})

	client, err := mongo.Connect(ctx, clientCfg)
	return newBasicAdminClient(client), trace.Wrap(err)
}

// basicAdminClient is a simple wrapper of mongo.Client with some helper
// functions. Note that mongo.Client is goroutine-safe.
type basicAdminClient struct {
	*mongo.Client

	serverVersion      *semver.Version
	serverVersionError error
	serverVersionOnce  sync.Once
}

func newBasicAdminClient(client *mongo.Client) *basicAdminClient {
	return &basicAdminClient{
		Client: client,
	}
}

func (c *basicAdminClient) ServerVersion(ctx context.Context) (*semver.Version, error) {
	c.serverVersionOnce.Do(func() {
		// Doesn't really matter which database to run "buildInfo" command on, and
		// "buildInfo" doesn't require extra permissions.
		resp := struct {
			Version string `bson:"version"`
		}{}
		c.serverVersionError = c.Database(externalDatabaseName).RunCommand(ctx, bson.D{
			{Key: "buildInfo", Value: true},
		}).Decode(&resp)
		if c.serverVersionError != nil {
			return
		}

		c.serverVersion, c.serverVersionError = semver.NewVersion(resp.Version)
	})
	return c.serverVersion, trace.Wrap(c.serverVersionError)
}

const (
	// adminClientFnCacheTTL specifies how long the shareableAdminClient will be
	// cached in FnCache.
	adminClientFnCacheTTL = time.Minute
	// adminClientCleanupTTL specifies how long to wait to terminate the
	// shareableAdminClient. Use a longer TTL than adminClientFnCacheTTL to make
	// sure the shareableAdminClient will be not returned by the FnCache while
	// getting terminated.
	adminClientCleanupTTL = adminClientFnCacheTTL + time.Second*10
)

var adminClientFnCache *utils.FnCache

func isAdminClientCacheDisabled() bool {
	return utils.AsBool(os.Getenv("TELEPORT_DISABLE_MONGODB_ADMIN_CLIENT_CACHE"))
}

func init() {
	if isAdminClientCacheDisabled() {
		return
	}

	var err error
	adminClientFnCache, err = utils.NewFnCache(utils.FnCacheConfig{
		TTL: adminClientFnCacheTTL,
	})
	// This should never fail. There is also an unit test to verify it's always
	// created. Logging error just in case.
	if err != nil {
		slog.WarnContext(context.Background(), "Failed to create MongoDB admin connection cache.", "error", err)
	}
}
