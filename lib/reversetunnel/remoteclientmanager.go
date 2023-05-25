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
	// connectLock ensures synchrounous access to fields used to connect clients.
	connectLock sync.Mutex
	// parentCtx is the parent context used to connect clients.
	parentCtx context.Context
	// wg ensures a single call to connect runs at a time.
	wg sync.WaitGroup
	// stop cancels the context used by connect.
	stop func()
	// closing is set to false when close is called.
	closing bool
	// once ensures connectedOnceOrClosed is closed once.
	once sync.Once
	// connectedOnceOrClosed indicates that clients have been connected at least once or the manager is closed.
	connectedOnceOrClosed chan struct{}
	// clients managed by the manager.
	auth        auth.ClientI
	accessPoint auth.RemoteProxyAccessPoint
	nodeWatcher *services.NodeWatcher
	caWatcher   *services.CertAuthorityWatcher
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

	m := &remoteClientManager{
		remoteClientManagerConfig: config,
		parentCtx:                 ctx,
		connectedOnceOrClosed:     make(chan struct{}),
	}
	return m, nil
}

// connect runs until all clients have been successfully setup.
func (m *remoteClientManager) connect(ctx context.Context) error {
	var (
		client      auth.ClientI
		accessPoint auth.RemoteProxyAccessPoint
		nodeWatcher *services.NodeWatcher
		caWatcher   *services.CertAuthorityWatcher
		err         error
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

		if client == nil {
			client, err = m.newClientFunc(ctx)
			if err != nil {
				m.log.WithError(err).Warnf("Failed to connect to remote auth server.")
				continue
			}
		}

		response, err := client.Ping(ctx)
		if err != nil {
			m.log.WithError(err).Warnf("Failed to get remote auth server version.")
			continue
		}

		version := response.ServerVersion
		if accessPoint != nil {
			accessPoint.Close()
		}
		accessPoint, err = m.newAccessPointFunc(ctx, client, version)
		if err != nil {
			m.log.WithError(err).Warnf("Failed to create remote access point.")
			continue
		}

		if nodeWatcher != nil {
			nodeWatcher.Close()
		}
		nodeWatcher, err = m.newNodeWatcherFunc(ctx, accessPoint)
		if err != nil {
			m.log.WithError(err).Warnf("Failed to create remote node watcher.")
			continue
		}

		if caWatcher != nil {
			caWatcher.Close()
		}
		caWatcher, err = m.newCAWatcher(ctx, accessPoint)
		if err != nil {
			m.log.WithError(err).Warnf("Failed to create remote CA watcher.")
			continue
		}

		m.clientLock.Lock()
		m.auth = client
		m.accessPoint = accessPoint
		m.nodeWatcher = nodeWatcher
		m.caWatcher = caWatcher
		m.once.Do(func() {
			close(m.connectedOnceOrClosed)
		})
		m.clientLock.Unlock()

		return nil
	}
}

// Connect blocks while all clients are created. If connect is called more than once
// existing clients will be closed and new clients created. Connect will always fail
// after Close is called.
func (m *remoteClientManager) Connect() error {
	errC := make(chan error)
	m.connectLock.Lock()
	if m.closing {
		m.connectLock.Unlock()
		return trace.Errorf("unable to connect: auth manager is closing")
	}

	// Stop the current connection attempt.
	if m.stop != nil {
		m.stop()
		m.wg.Wait()
	}

	ctx, cancel := context.WithCancel(m.parentCtx)
	m.stop = cancel

	m.clientLock.Lock()
	client := m.auth
	accessPoint := m.accessPoint
	nodeWatcher := m.nodeWatcher
	caWatcher := m.caWatcher
	m.auth = nil
	m.accessPoint = nil
	m.nodeWatcher = nil
	m.caWatcher = nil

	// Close previous clients in background.
	go func() {
		m.close(client, accessPoint, nodeWatcher, caWatcher)
	}()
	m.clientLock.Unlock()

	m.wg.Add(1)
	m.connectLock.Unlock()
	go func() {
		defer m.wg.Done()
		errC <- m.connect(ctx)
	}()
	return trace.Wrap(<-errC)
}

// close closes the given clients returning an aggregate error.
func (m *remoteClientManager) close(auth auth.ClientI, ap auth.RemoteProxyAccessPoint, nw *services.NodeWatcher, caw *services.CertAuthorityWatcher) error {
	errs := make([]error, 2)
	if caw != nil {
		caw.Close()
	}
	if nw != nil {
		nw.Close()
	}
	if ap != nil {
		errs[0] = ap.Close()
	}
	if auth != nil {
		errs[1] = auth.Close()
	}
	return trace.NewAggregate(errs...)
}

// Wait waits until client connections are established at least once or the manager is closed.
func (m *remoteClientManager) Wait() {
	select {
	case <-m.connectedOnceOrClosed:
	case <-m.parentCtx.Done():
	}
}

// Close stops the manager from connecting clients and closes
// any existing clients.
func (m *remoteClientManager) Close() error {
	m.connectLock.Lock()
	if m.closing {
		m.connectLock.Unlock()
		return trace.Errorf("client manager is already closed")
	}

	m.closing = true
	m.stop()
	m.once.Do(func() {
		close(m.connectedOnceOrClosed)
	})
	m.connectLock.Unlock()

	m.wg.Wait()

	m.clientLock.Lock()
	client := m.auth
	accessPoint := m.accessPoint
	nodeWatcher := m.nodeWatcher
	caWatcher := m.caWatcher
	m.clientLock.Unlock()
	return trace.Wrap(m.close(client, accessPoint, nodeWatcher, caWatcher))
}

// Client returns a auth.ClientI or an error if the client is not connected.
func (m *remoteClientManager) Client() (auth.ClientI, error) {
	m.clientLock.RLock()
	defer m.clientLock.RUnlock()
	if m.auth == nil {
		return nil, trace.ConnectionProblem(nil, "")
	}
	return m.auth, nil
}

// RemoteProxyAccessPoint returns a RemoteProxyAccessPoint or an error if the client is not connected.
func (m *remoteClientManager) RemoteProxyAccessPoint() (auth.RemoteProxyAccessPoint, error) {
	m.clientLock.RLock()
	defer m.clientLock.RUnlock()
	if m.accessPoint == nil {
		return nil, trace.ConnectionProblem(nil, "")
	}
	return m.accessPoint, nil
}

// NodeWatcher returns a NodeWatcher or an error if the client is not connected.
func (m *remoteClientManager) NodeWatcher() (*services.NodeWatcher, error) {
	m.clientLock.RLock()
	defer m.clientLock.RUnlock()
	if m.nodeWatcher == nil {
		return nil, trace.ConnectionProblem(nil, "")
	}
	return m.nodeWatcher, nil
}

// CAWatcher returns a CertAuthorityWatcher or an error if the client is not connected.
func (m *remoteClientManager) CAWatcher() (*services.CertAuthorityWatcher, error) {
	m.clientLock.RLock()
	defer m.clientLock.RUnlock()
	if m.caWatcher == nil {
		return nil, trace.ConnectionProblem(nil, "")
	}
	return m.caWatcher, nil
}
