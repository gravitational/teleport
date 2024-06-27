/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package auth

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"math"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/genmap"
)

// ClientTLSConfigGeneratorConfig holds parameters for ClientTLSConfigGenerator setup.
type ClientTLSConfigGeneratorConfig struct {
	// TLS is the upstream TLS config that per-cluster configs are generated from.
	TLS *tls.Config
	// ClusterName is the name of the current cluster.
	ClusterName string
	// PermiteRemoteClusters must be true for non-local cluster CAs to be used. Most usecases
	// want this to be true, but it defaults to false to help avoid accidentally expanding the
	// CA pool in cases where remote cluster CA usage is inappropriate.
	PermitRemoteClusters bool
	// AccessPoint is the upstream data source used to lookup cert authorities
	// and watch for changes.
	AccessPoint AccessCacheWithEvents
}

// CheckAndSetDefaults checks that required parameters were supplied and
// sets default values as needed.
func (cfg *ClientTLSConfigGeneratorConfig) CheckAndSetDefaults() error {
	if cfg.TLS == nil {
		return trace.BadParameter("missing required parameter 'TLS' for client tls config generator")
	}

	if cfg.ClusterName == "" {
		return trace.BadParameter("missing required parameter 'ClusterName' for client tls config generator")
	}

	if cfg.AccessPoint == nil {
		return trace.BadParameter("missing required parameter 'AccessPoint' for client tls config generator")
	}

	return nil
}

// ClientTLSConfigGenerator is a helper type used to implement fast & efficient client tls config specialization based upon
// the target cluster specified in the client TLS hello. This type keeps per-cluster client TLS configs pre-generated and
// refreshes them periodically and/or when ca modification events are observed. The GetConfigForClient method of this type
// is intended to be slotted into the GetConfigForClient field of tls.Config.
type ClientTLSConfigGenerator struct {
	// cfg holds the config parameters for this generator.
	cfg ClientTLSConfigGeneratorConfig
	// clientTLSConfigs is a specialized cache that stores tls configs
	// by cluster name.
	clientTLSConfigs *genmap.GenMap[string, *tls.Config]
	// cancel terminates the above close context.
	cancel context.CancelFunc
}

// NewClientTLSConfigGenerator sets up a new generator based on the supplied parameters.
func NewClientTLSConfigGenerator(cfg ClientTLSConfigGeneratorConfig) (*ClientTLSConfigGenerator, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &ClientTLSConfigGenerator{
		cfg:    cfg,
		cancel: cancel,
	}

	var err error
	c.clientTLSConfigs, err = genmap.New(genmap.Config[string, *tls.Config]{
		Generator: c.generator,
	})

	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	go c.refreshClientTLSConfigs(ctx)

	return c, nil
}

// GetConfigForClient is intended to be slotted into the GetConfigForClient field of tls.Config.
func (c *ClientTLSConfigGenerator) GetConfigForClient(info *tls.ClientHelloInfo) (*tls.Config, error) {
	var clusterName string
	var err error
	switch info.ServerName {
	case "":
		// Client does not use SNI, will validate against all known CAs.
	default:
		clusterName, err = apiutils.DecodeClusterName(info.ServerName)
		if err != nil {
			if !trace.IsNotFound(err) {
				slog.WarnContext(context.Background(), "ignoring unsupported cluster name in client hello", "cluster_name", info.ServerName)
				clusterName = ""
			}
		}
	}

	cfg, err := c.clientTLSConfigs.Get(context.Background(), clusterName)
	return cfg, trace.Wrap(err)
}

var errNonLocalCluster = errors.New("non-local cluster specified in client hello")

// generator is the underlying lookup function used to resolve the tls config that should be used for a
// given cluster. this method is used by the underlying genmap to load/refresh values as-needed.
func (c *ClientTLSConfigGenerator) generator(ctx context.Context, clusterName string) (*tls.Config, error) {
	if !c.cfg.PermitRemoteClusters && clusterName != c.cfg.ClusterName {
		if clusterName != "" {
			slog.WarnContext(ctx, "refusing to set up client cert pool for non-local cluster", "cluster_name", clusterName)
			return nil, trace.Wrap(errNonLocalCluster)
		}
		// unsepcified cluster name should be treated as a request for local cluster CAs
		clusterName = c.cfg.ClusterName
	}

	// update client certificate pool based on currently trusted TLS
	// certificate authorities.
	pool, totalSubjectsLen, err := authclient.DefaultClientCertPool(ctx, c.cfg.AccessPoint, clusterName)
	if err != nil {
		slog.ErrorContext(ctx, "failed to retrieve client cert pool for target cluster", "cluster_name", clusterName, "error", err)
		// this falls back to the default config
		return nil, nil
	}

	// Per https://tools.ietf.org/html/rfc5246#section-7.4.4 the total size of
	// the known CA subjects sent to the client can't exceed 2^16-1 (due to
	// 2-byte length encoding). The crypto/tls stack will panic if this
	// happens.
	//
	// This usually happens on the root cluster with a very large (>500) number
	// of leaf clusters. In these cases, the client cert will be signed by the
	// current (root) cluster.
	//
	// If the number of CAs turns out too large for the handshake, drop all but
	// the current cluster CA. In the unlikely case where it's wrong, the
	// client will be rejected.
	if totalSubjectsLen >= int64(math.MaxUint16) {
		slog.WarnContext(ctx, "cluster subject name set too large for TLS handshake, falling back to using local cluster CAs only")
		pool, _, err = authclient.DefaultClientCertPool(ctx, c.cfg.AccessPoint, c.cfg.ClusterName)
		if err != nil {
			slog.ErrorContext(ctx, "failed to retrieve client cert pool for current cluster", "cluster_name", c.cfg.ClusterName, "error", err)
			// this falls back to the default config
			return nil, nil
		}
	}

	tlsCopy := c.cfg.TLS.Clone()
	tlsCopy.ClientCAs = pool
	return tlsCopy, nil
}

// refreshClientTLSConfigs is the top-level loop for client TLS config regen. note that it
// has a fairly aggressive retry since this is a server-side singleton.
func (c *ClientTLSConfigGenerator) refreshClientTLSConfigs(ctx context.Context) {
	var lastWarning time.Time
	for {
		err := c.watchForCAChanges(ctx)
		if ctx.Err() != nil {
			return
		}

		if lastWarning.IsZero() || time.Since(lastWarning) > time.Second*30 {
			slog.WarnContext(ctx, "cert authority watch loop for client TLS config generator failed", "error", err)
			lastWarning = time.Now()
		}

		select {
		case <-time.After(utils.FullJitter(time.Second * 3)):
		case <-ctx.Done():
			return
		}
	}
}

// watchForCAChanges sets up a cert authority watcher and triggers regeneration of client
// tls configs for a given cluster whenever a CA associated with that cluster is modified.
// note that this function errs on the side of regenerating more often than might be
// strictly necessary.
func (c *ClientTLSConfigGenerator) watchForCAChanges(ctx context.Context) error {
	watcher, err := c.cfg.AccessPoint.NewWatcher(ctx, types.Watch{
		Name: "client-tls-config-generator",
		Kinds: []types.WatchKind{
			{Kind: types.KindCertAuthority, LoadSecrets: false},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()

	select {
	case <-watcher.Done():
		return trace.Errorf("ca watcher exited while waiting for init: %v", watcher.Error())
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("expected init event from ca watcher, got %v instead", event.Type)
		}
	case <-time.After(time.Second * 30):
		return trace.Errorf("timeout waiting for ca watcher init")
	case <-ctx.Done():
		return nil
	}

	c.clientTLSConfigs.RegenAll()

	for {
		select {
		case <-watcher.Done():
			return trace.Errorf("ca watcher exited with: %v", watcher.Error())
		case event := <-watcher.Events():
			if event.Type == types.OpDelete {
				c.clientTLSConfigs.Terminate(event.Resource.GetName())
			} else {
				if !c.cfg.PermitRemoteClusters && event.Resource.GetName() != c.cfg.ClusterName {
					// ignore non-local cluster CA events when we aren't configured to support them
					continue
				}
				// trigger regen of client tls configs for the associated cluster.
				c.clientTLSConfigs.Generate(event.Resource.GetName())
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// Close terminates background ca load/refresh operations.
func (c *ClientTLSConfigGenerator) Close() error {
	c.clientTLSConfigs.Close()
	c.cancel()
	return nil
}
