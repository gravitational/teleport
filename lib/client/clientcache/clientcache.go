// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package clientcache

import (
	"context"
	"crypto/x509"
	"errors"
	"log/slog"
	"sync"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/singleflight"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	apiprofile "github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// Cache stores clients keyed by profile name and leaf cluster name.
// Safe for concurrent access.
//
// A cached client lives in Cache.clients and can be leased. A retired client
// has been removed from Cache.clients: it can no longer be leased but stays
// open for its outstanding leases and is closed by the last release. Close
// retires every client (but does not close active leases) and rejects new acquisitions.
type Cache struct {
	cfg Config
	mu  sync.Mutex
	// clients holds clients available for acquisition.
	clients map[key]*clientWithMetadata
	// closed rejects new acquisitions after Close.
	closed bool
	// group prevents duplicate requests to create clients for a given cluster.
	group singleflight.Group
}

type clientWithMetadata struct {
	client *client.ClusterClient
	// tlsCert is the certificate that was loaded when the client was created.
	tlsCert *x509.Certificate
	// getProfile reads the fresh profile for the client from disk.
	getProfile func() (profile, error)
	// leases counts active leases. Protected by Cache.mu.
	leases int
	// retired is set once the client has been removed from Cache.clients
	// while there were active leases. Protected by Cache.mu.
	retired bool
}

type profile interface {
	// TLSCert returns the current TLS cert, stored in the agent.
	TLSCert() ([]byte, error)
}

// isTLSCertStale checks if the cached client uses the current TLS cert
// (read from the agent). If not, the TLS cert is considered stale.
func (c *clientWithMetadata) isTLSCertStale() (bool, error) {
	pr, err := c.getProfile()
	if err != nil {
		if apiprofile.IsMissingOrInvalid(err) {
			return true, nil
		}
		return false, trace.Wrap(err)
	}

	tlsCert, err := pr.TLSCert()
	if err != nil {
		if trace.IsNotFound(err) {
			return true, nil
		}
		return false, trace.Wrap(err)
	}

	parsedTLSCert, err := tlsca.ParseCertificatePEM(tlsCert)
	if err != nil {
		return false, trace.Wrap(err)
	}

	return !c.tlsCert.Equal(parsedTLSCert), nil
}

// expiredError returns an error if the cert the client
// authenticated with has expired according to clock, or nil if it's still
// valid.
func (c *clientWithMetadata) expiredError(clock clockwork.Clock) error {
	if err := utils.VerifyCertificateExpiry(c.tlsCert, clock); err != nil {
		return trace.Wrap(apiclient.ErrClientCredentialsHaveExpired, "tls: expired certificate: %v", err)
	}
	return nil
}

// NewClientFunc is a function that will return a new [*client.TeleportClient] for a given profile and leaf
// cluster. [leafClusterName] may be empty, in which case implementations should return a client for the root cluster.
type NewClientFunc func(ctx context.Context, profileName, leafClusterName string) (*client.TeleportClient, error)

// Config describes the client cache configuration.
type Config struct {
	NewClientFunc NewClientFunc
	Logger        *slog.Logger
	// Clock is used to determine whether a cached certificate has expired.
	Clock clockwork.Clock
}

func (c *Config) checkAndSetDefaults() error {
	if c.NewClientFunc == nil {
		return trace.BadParameter("NewClientFunc is required")
	}
	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "clientcache")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

type key struct {
	profile     string
	leafCluster string
}

func (k key) String() string {
	if k.leafCluster != "" {
		return k.profile + "/" + k.leafCluster
	}
	return k.profile
}

// New creates an instance of Cache.
func New(c Config) (*Cache, error) {
	if err := c.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Cache{
		cfg:     c,
		clients: make(map[key]*clientWithMetadata),
	}, nil
}

var errCacheClosed = errors.New("client cache is closed")

// Lease provides exclusive lifetime tracking for an operation using a cached
// client. Call Release when the operation, including any returned stream, has
// finished. The caller must not close Client.
type Lease struct {
	client  *client.ClusterClient
	release func() error
	once    sync.Once
	err     error
}

// NewLease creates a lease for clt. The caller must not close.
func NewLease(clt *client.ClusterClient, release func() error) *Lease {
	return &Lease{client: clt, release: release}
}

// Client returns the leased cluster client.
func (l *Lease) Client() *client.ClusterClient {
	return l.client
}

// Release records that the operation using the client has completed. It is
// safe to call more than once.
func (l *Lease) Release() error {
	l.once.Do(func() { l.err = l.release() })
	return l.err
}

// Do acquires a client for the duration of fn. Most callers should prefer Do
// over managing a lease directly via Acquire.
// Acquire is intended for operations whose lifetime cannot be expressed by
// a synchronous callback, such as streams.
func (c *Cache) Do(ctx context.Context, profileName, leafClusterName string, fn func(*client.ClusterClient) error) (err error) {
	lease, err := c.Acquire(ctx, profileName, leafClusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		err = trace.NewAggregate(err, lease.Release())
	}()
	return trace.Wrap(fn(lease.Client()))
}

// Acquire returns a client lease. If the cached client's certificate has
// expired, Acquire reports that directly instead of discarding and recreating it.
// Existing leases remain usable until they are released.
func (c *Cache) Acquire(ctx context.Context, profileName, leafClusterName string) (*Lease, error) {
	k := key{profile: profileName, leafCluster: leafClusterName}

	// ClearStaleClientsForRoot can remove a client after c.create publishes it
	// but before c.acquireOrExpired leases it. We use the loop to check the cache
	// again after c.group.Do completes and create a replacement if needed.
	// Allow at most three creation attempts so repeated invalidation cannot retry indefinitely.
	const maxCreateAttempts = 3
	for attempt := range maxCreateAttempts + 1 {
		lease, acquired, err := c.acquireOrExpired(k)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if acquired {
			c.cfg.Logger.DebugContext(ctx, "Retrieved client from cache", "cluster", k)
			return lease, nil
		}
		if attempt == maxCreateAttempts {
			// Each iteration checks the cache before creating.
			// If we don't have a client lease at this point, do not attempt another create.
			break
		}

		// create returns without publishing when another caller published a client first.
		// The next iteration checks the cache again.
		if _, err, _ := c.group.Do(k.String(), func() (any, error) {
			return nil, c.create(ctx, k)
		}); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return nil, trace.LimitExceeded("client for %s was invalidated %d times while connecting", k, maxCreateAttempts)
}

// create connects a new client for k and publishes it. It returns nil without
// publishing when a client for k already exists; Acquire then checks the cache
// again.
func (c *Cache) create(ctx context.Context, k key) error {
	c.mu.Lock()
	closed, exists := c.closed, c.clients[k] != nil
	c.mu.Unlock()
	if closed {
		return trace.Wrap(errCacheClosed)
	}
	if exists {
		return nil
	}

	tc, err := c.cfg.NewClientFunc(ctx, k.profile, k.leafCluster)
	if err != nil {
		return trace.Wrap(err)
	}

	// The cert is read before connecting. If it changes while the connection
	// is in flight, the client may have authenticated with the older cert.
	// Record that cert so publish can compare it against the one on disk and
	// drop such a client instead of caching it.
	tlsCert, err := currentTLSCert(tc)
	if err != nil {
		return trace.Wrap(err)
	}
	parsedTLSCert, err := tlsca.ParseCertificatePEM(tlsCert)
	if err != nil {
		return trace.Wrap(err)
	}
	clt, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	entry := &clientWithMetadata{
		client:  clt,
		tlsCert: parsedTLSCert,
		getProfile: func() (profile, error) {
			return tc.GetProfile(tc.WebProxyAddr)
		},
	}

	return trace.Wrap(c.publish(k, entry))
}

func currentTLSCert(tc *client.TeleportClient) ([]byte, error) {
	keyRing, err := tc.LocalAgent().GetCoreKeyRing()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keyRing.TLSCert, nil
}

// publish stores a newly created client. The client is closed instead if the
// cache was closed or if the cert it authenticated with was superseded while
// it was connecting.
func (c *Cache) publish(k key, entry *clientWithMetadata) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return trace.NewAggregate(trace.Wrap(errCacheClosed), entry.client.Close())
	}

	// A relogin may have rotated the cert while the client was connecting.
	stale, err := entry.isTLSCertStale()
	if err != nil {
		return trace.NewAggregate(trace.Wrap(err), entry.client.Close())
	}
	if stale {
		c.cfg.Logger.InfoContext(context.Background(), "Discarding client connected with a superseded cert", "cluster", k)
		return trace.Wrap(entry.client.Close())
	}

	c.clients[k] = entry
	c.cfg.Logger.InfoContext(context.Background(), "Added client to cache", "cluster", k)
	return nil
}

// ClearStaleClientsForRoot removes clients from the cache for the root
// cluster and its leaf clusters, if their cert is outdated. Removed clients
// are closed once no lease refers to them.
func (c *Cache) ClearStaleClientsForRoot(profileName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return trace.Wrap(errCacheClosed)
	}

	var (
		errors  []error
		retired []string
	)

	for k, clt := range c.clients {
		if k.profile != profileName {
			continue
		}
		stale, err := clt.isTLSCertStale()
		// If an error occurs, close the client as well.
		if err != nil {
			errors = append(errors, err)
		} else if !stale {
			continue
		}
		if err := c.retireLocked(k, clt); err != nil {
			errors = append(errors, err)
		}
		retired = append(retired, k.String())
	}

	c.cfg.Logger.InfoContext(context.Background(), "Invalidated cached clients for root cluster",
		"cluster", profileName,
		"clients", retired,
	)

	return trace.NewAggregate(errors...)
}

// Close rejects new acquisitions and retires every client. Idle clients are
// closed immediately; leased clients are closed by their last release.
func (c *Cache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true

	var errors []error
	for k, clt := range c.clients {
		if err := c.retireLocked(k, clt); err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

// acquireOrExpired leases the cached client for k if one exists and is not
// expired. The returned bool reports whether a lease was acquired; when it is
// false (and err is nil) there is no cached client at all and the caller
// should attempt to create one.
func (c *Cache) acquireOrExpired(k key) (*Lease, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, false, trace.Wrap(errCacheClosed)
	}

	clt := c.clients[k]
	if clt == nil {
		return nil, false, nil
	}
	if err := clt.expiredError(c.cfg.Clock); err != nil {
		return nil, false, trace.Wrap(err)
	}
	clt.leases++
	return NewLease(clt.client, func() error { return c.release(clt) }), true, nil
}

func (c *Cache) release(clt *clientWithMetadata) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	clt.leases--
	if clt.retired && clt.leases == 0 {
		return clt.client.Close()
	}
	return nil
}

// retireLocked removes a cached client from new acquisitions. The client is
// closed immediately when idle, or on the final release of an active lease.
// It must be called with a client that is currently stored under k.
func (c *Cache) retireLocked(k key, clt *clientWithMetadata) error {
	delete(c.clients, k)
	if clt.leases > 0 {
		clt.retired = true
		return nil
	}
	return clt.client.Close()
}

// NoCache implements the same leasing API as [Cache], but creates a new
// client for every acquisition. Clients are closed when their lease is
// released.
type NoCache struct {
	mu            sync.Mutex
	newClientFunc NewClientFunc
	closed        bool
}

func NewNoCache(newClientFunc NewClientFunc) *NoCache {
	return &NoCache{newClientFunc: newClientFunc}
}

func (c *NoCache) Acquire(ctx context.Context, profileName, leafClusterName string) (*Lease, error) {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return nil, trace.Wrap(errCacheClosed)
	}

	tc, err := c.newClientFunc(ctx, profileName, leafClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Close may race with client creation. Do not hand out a client after the
	// NoCache has been closed.
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, trace.NewAggregate(trace.Wrap(errCacheClosed), clt.Close())
	}

	return NewLease(clt, clt.Close), nil
}

func (c *NoCache) Do(ctx context.Context, profileName, leafClusterName string, fn func(*client.ClusterClient) error) (err error) {
	lease, err := c.Acquire(ctx, profileName, leafClusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		err = trace.NewAggregate(err, lease.Release())
	}()
	return trace.Wrap(fn(lease.Client()))
}

func (c *NoCache) ClearStaleClientsForRoot(string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return trace.Wrap(errCacheClosed)
	}
	return nil
}

// Close rejects new acquisitions. Clients with active leases are closed when
// those leases are released, matching [Cache.Close].
func (c *NoCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}
