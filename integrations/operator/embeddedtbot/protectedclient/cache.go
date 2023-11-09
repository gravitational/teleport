package protectedclient

import (
	"bytes"
	"context"
	"sync"

	"github.com/gravitational/trace"
)

// Cache stores the current ProtectedClient and checks if it must be rebuilt
// by comparing the client certs and the tbot ones. When a new client is built,
// the previous client is asynchronously closed.
type Cache struct {
	// mutex protects cachedCert and cachedClient
	mutex        sync.Mutex
	cachedCert   []byte
	cachedClient *ProtectedClient

	// clientBuilder is used for testing purposes. Outside of tests, its value should always be buildClient.
	clientBuilder func(ctx context.Context) (*ProtectedClient, error)
	certGetter    func() ([]byte, error)
}

// NewCache creates a new Cache.
func NewCache(clientBuilder func(ctx context.Context) (*ProtectedClient, error), certGetter func() ([]byte, error)) *Cache {
	return &Cache{
		clientBuilder: clientBuilder,
		certGetter:    certGetter,
	}
}

// Get is an Accessor: it returns a protected teleport client.
// When possible, the cached client is returned. If not, a new client is
// built and the previous client is retired.
func (c *Cache) Get(ctx context.Context) (*ProtectedClient, func(), error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// This is where caching happens. We don't know when tbot renews the certificates, so we need to check
	// if the current certificate stored in memory changed since last time. If it did not and we already built a
	// working client, then we hit the cache. Else we build a new client, replace the cached client with the new one,
	// and fire a separate goroutine to close the previous client.
	cert, err := c.certGetter()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if len(cert) == 0 {
		return nil, nil, trace.CompareFailed("no certificate in tbot's memory, cannot compare")
	}

	// We can use the cached client
	if c.cachedClient != nil && bytes.Equal(cert, c.cachedCert) {
		return c.cachedClient, c.cachedClient.lockClient(), nil
	}

	// We cannot use the cached client and build a new one
	oldClient := c.cachedClient
	freshClient, err := c.clientBuilder(ctx)

	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	c.cachedCert = cert
	c.cachedClient = freshClient

	if oldClient != nil {
		go oldClient.RetireClient()
	}

	return c.cachedClient, c.cachedClient.lockClient(), nil
}
