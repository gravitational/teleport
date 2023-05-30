/*
Copyright 2015-2019 Gravitational, Inc.

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

package reversetunnel

import (
	"context"
	"sync"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
)

// remoteClientManagerConfig configures a remote client manager.
type remoteClientManagerConfig struct {
	newClientFunc      func(context.Context) (auth.ClientI, error)
	newAccessPointFunc func(ctx context.Context, client auth.ClientI, version string) (auth.RemoteProxyAccessPoint, error)
	newNodeWatcherFunc func(context.Context, auth.RemoteProxyAccessPoint) (*services.NodeWatcher, error)
	newCAWatcher       func(context.Context, auth.RemoteProxyAccessPoint) (*services.CertAuthorityWatcher, error)
	log                logrus.FieldLogger
	backoff            *retryutils.RetryV2
}

// remoteClientManager handles connecting remote cluster clients.
type remoteClientManager struct {
	remoteClientManagerConfig
	// clientLock handles concurrent access to the various clients.
	clientLock sync.RWMutex
	// ctx is the context used to connect clients.
	ctx context.Context
	// stop cancels the context used by connect.
	stop func()
	// wg ensures a single call to connect runs at a time.
	wg sync.WaitGroup
	// connectOnce ensures connect is called exactly once.
	connectOnce sync.Once
	// clients are remote clients managed by the manager.
	clients *remoteClients
}

func newRemoteClientManager(ctx context.Context, config remoteClientManagerConfig) (*remoteClientManager, error) {
	if config.newClientFunc == nil {
		return nil, trace.BadParameter("missing new client func")
	}
	if config.newAccessPointFunc == nil {
		return nil, trace.BadParameter("missing new access point func")
	}
	if config.newNodeWatcherFunc == nil {
		return nil, trace.BadParameter("missing new node watcher func")
	}
	if config.newCAWatcher == nil {
		return nil, trace.BadParameter("missing new ca watcher func")
	}
	if config.log == nil {
		return nil, trace.BadParameter("missing logger")
	}

	ctx, close := context.WithCancel(ctx)
	m := &remoteClientManager{
		remoteClientManagerConfig: config,
		ctx:                       ctx,
		stop:                      close,
		clients:                   &remoteClients{},
	}
	m.wg.Add(1)
	return m, nil
}

// connect runs until all clients have been successfully setup.
func (m *remoteClientManager) connect(ctx context.Context) error {
	var (
		clients remoteClients
		err     error
	)

	firstIteration := true
	for {
		if err := ctx.Err(); err != nil {
			return trace.Wrap(err)
		}

		// Use backoff after first attempt.
		if !firstIteration {
			<-m.backoff.After()
			m.backoff.Inc()
		} else {
			firstIteration = false
		}

		if clients.auth == nil {
			clients.auth, err = m.newClientFunc(ctx)
			if err != nil {
				m.log.WithError(err).Warnf("Failed to connect to remote auth server.")
				continue
			}
		}

		response, err := clients.auth.Ping(ctx)
		if err != nil {
			m.log.WithError(err).Warnf("Failed to get remote auth server version.")
			continue
		}

		version := response.ServerVersion
		if clients.accessPoint == nil {
			clients.accessPoint, err = m.newAccessPointFunc(ctx, clients.auth, version)
			if err != nil {
				m.log.WithError(err).Warnf("Failed to create remote access point.")
				continue
			}
		}

		if clients.nodeWatcher == nil {
			clients.nodeWatcher, err = m.newNodeWatcherFunc(ctx, clients.accessPoint)
			if err != nil {
				m.log.WithError(err).Warnf("Failed to create remote node watcher.")
				continue
			}
		}

		if clients.caWatcher == nil {
			clients.caWatcher, err = m.newCAWatcher(ctx, clients.accessPoint)
			if err != nil {
				m.log.WithError(err).Warnf("Failed to create remote CA watcher.")
				continue
			}
		}

		m.clientLock.Lock()
		m.clients = &clients
		m.clientLock.Unlock()

		return nil
	}
}

// Connect blocks while all clients are created.
// Calling connect more than once is not supported.
func (m *remoteClientManager) Connect() error {
	var err error
	m.connectOnce.Do(func() {
		defer m.wg.Done()
		err = m.connect(m.ctx)
	})
	return trace.Wrap(err)
}

// Wait waits until client connections are established at least once or the manager is closed.
func (m *remoteClientManager) Wait() {
	m.wg.Wait()
}

// Close stops the manager from connecting clients and closes
// any existing clients.
func (m *remoteClientManager) Close() error {
	m.stop()
	m.wg.Wait()

	m.clientLock.Lock()
	clients := m.clients
	m.clients = &remoteClients{}
	m.clientLock.Unlock()

	if clients == nil {
		return trace.Errorf("remote client manager already closed")
	}

	return trace.Wrap(clients.Close())
}

// remoteClients wraps remote clients together for the convenience of the client manager.
type remoteClients struct {
	auth        auth.ClientI
	accessPoint auth.RemoteProxyAccessPoint
	nodeWatcher *services.NodeWatcher
	caWatcher   *services.CertAuthorityWatcher
}

func (c *remoteClients) Close() error {
	errs := make([]error, 2)
	if c.caWatcher != nil {
		c.caWatcher.Close()
	}
	if c.nodeWatcher != nil {
		c.nodeWatcher.Close()
	}
	if c.accessPoint != nil {
		errs[0] = c.accessPoint.Close()
	}
	if c.auth != nil {
		errs[1] = c.auth.Close()
	}
	return trace.NewAggregate(errs...)
}

// Client returns a auth.ClientI or an error if the client is not connected.
func (m *remoteClientManager) Auth() (auth.ClientI, error) {
	m.clientLock.RLock()
	defer m.clientLock.RUnlock()
	if m.clients.auth == nil {
		return nil, trace.ConnectionProblem(nil, "")
	}
	return m.clients.auth, nil
}

// RemoteProxyAccessPoint returns a RemoteProxyAccessPoint or an error if the client is not connected.
func (m *remoteClientManager) RemoteProxyAccessPoint() (auth.RemoteProxyAccessPoint, error) {
	m.clientLock.RLock()
	defer m.clientLock.RUnlock()
	if m.clients.accessPoint == nil {
		return nil, trace.ConnectionProblem(nil, "")
	}
	return m.clients.accessPoint, nil
}

// NodeWatcher returns a NodeWatcher or an error if the client is not connected.
func (m *remoteClientManager) NodeWatcher() (*services.NodeWatcher, error) {
	m.clientLock.RLock()
	defer m.clientLock.RUnlock()
	if m.clients.nodeWatcher == nil {
		return nil, trace.ConnectionProblem(nil, "")
	}
	return m.clients.nodeWatcher, nil
}

// CAWatcher returns a CertAuthorityWatcher or an error if the client is not connected.
func (m *remoteClientManager) CAWatcher() (*services.CertAuthorityWatcher, error) {
	m.clientLock.RLock()
	defer m.clientLock.RUnlock()
	if m.clients.caWatcher == nil {
		return nil, trace.ConnectionProblem(nil, "")
	}
	return m.clients.caWatcher, nil
}
