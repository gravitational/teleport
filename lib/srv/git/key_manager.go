/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package git

import (
	"bytes"
	"context"
	"log/slog"
	"net"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/gitserver"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

// AccessPoint defines a subset of functions needed by git services.
type AccessPoint interface {
	services.AuthPreferenceGetter
	GitServerReadOnlyClient() gitserver.ReadOnlyClient
}

// KeyManagerConfig is the config used for KeyManager.
type KeyManagerConfig struct {
	// ParentContext is the parent's context. All background tasks started by
	// KeyManager will be stopped when ParentContext is closed.
	ParentContext context.Context
	// AuthClient is a client connected to the Auth server of this local cluster.
	AuthClient authclient.ClientI
	// AccessPoint is a caching client that provides access to this local cluster.
	AccessPoint AccessPoint
	// Logger is the slog.Logger
	Logger *slog.Logger

	githubServerKeys *githubKeyDownloader
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (c *KeyManagerConfig) CheckAndSetDefaults() error {
	if c.ParentContext == nil {
		return trace.BadParameter("missing parameter ParentContext")
	}
	if c.AuthClient == nil {
		return trace.BadParameter("missing parameter AuthClient")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("missing parameter AccessPoint")
	}
	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, teleport.ComponentGit)
	}
	if c.githubServerKeys == nil {
		c.githubServerKeys = newGitHubKeyDownloader()
	}
	return nil
}

// KeyManager manages and caches remote server keys.
type KeyManager struct {
	cfg *KeyManagerConfig
}

// NewKeyManager creates a service that manages and caches remote server keys.
// TODO(greedy52) move user cert generation here with caching.
func NewKeyManager(cfg *KeyManagerConfig) (*KeyManager, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	m := &KeyManager{
		cfg: cfg,
	}

	if err := m.startWatcher(cfg.ParentContext); err != nil {
		return nil, trace.Wrap(err)
	}
	return m, nil
}

func (m *KeyManager) startWatcher(ctx context.Context) error {
	watcher, err := services.NewGitServerWatcher(ctx, services.GitServerWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentGit,
			Logger:    m.cfg.Logger,
			Client:    m.cfg.AuthClient,
		},
		GitServerGetter:       m.cfg.AccessPoint.GitServerReadOnlyClient(),
		EnableUpdateBroadcast: true,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Start background downloads only when git_servers are found.
	// TODO(greedy52) use a reconciler and start downloader by type.
	go func() {
		defer m.cfg.Logger.DebugContext(ctx, "Git server resource watcher done.")
		defer watcher.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case resources := <-watcher.ResourcesC:
				m.cfg.Logger.DebugContext(ctx, "Received git server resources from watcher", "len", len(resources))
				if len(resources) > 0 {
					go m.cfg.githubServerKeys.Start(ctx)
					return
				}
			}
		}
	}()
	return nil
}

// HostKeyCallback creates an ssh.HostKeyCallback for verifying the target git-hosting service.
func (m *KeyManager) HostKeyCallback(targetServer types.Server) (ssh.HostKeyCallback, error) {
	switch targetServer.GetSubKind() {
	case types.SubKindGitHub:
		return m.verifyGitHub, nil
	default:
		return nil, trace.BadParameter("unsupported subkind %q", targetServer.GetSubKind())
	}
}

func (m *KeyManager) verifyGitHub(_ string, _ net.Addr, key ssh.PublicKey) error {
	knownKeys, err := m.cfg.githubServerKeys.GetKnownKeys()
	if err != nil {
		return trace.Wrap(err)
	}
	marshaledKey := key.Marshal()
	for _, knownKey := range knownKeys {
		if knownKey.Type() == key.Type() {
			if bytes.Equal(knownKey.Marshal(), marshaledKey) {
				return nil
			}
		}
	}
	return trace.BadParameter("cannot verify github.com")
}
