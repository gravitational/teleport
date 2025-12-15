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

package internal

import (
	"context"
	"log/slog"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
)

// CachingProxyPingerConfig is the configuration for CachingProxyPinger.
type CachingProxyPingerConfig struct {
	Connection connection.Config
	Client     *client.Client
	Logger     *slog.Logger
}

func (cfg *CachingProxyPingerConfig) CheckAndSetDefaults() error {
	if err := cfg.Connection.Validate(); err != nil {
		return trace.Wrap(err)
	}
	if cfg.Client == nil {
		return trace.BadParameter("Client is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return nil
}

// NewCachingProxyPinger creates a new CachingProxyPinger with the given configuration.
func NewCachingProxyPinger(cfg CachingProxyPingerConfig) (*CachingProxyPinger, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &CachingProxyPinger{
		connCfg: cfg.Connection,
		client:  cfg.Client,
		logger:  cfg.Logger,
	}, nil
}

// CachingProxyPinger pings the proxy server for connection information caches
// the result indefinitely. If the user has configured an auth server address,
// it will be pinged via gRPC to get the proxy address.
type CachingProxyPinger struct {
	connCfg connection.Config
	client  *client.Client
	logger  *slog.Logger

	mu          sync.Mutex
	cachedValue *connection.ProxyPong
}

// Ping the proxy for connection information.
func (c *CachingProxyPinger) Ping(ctx context.Context) (*connection.ProxyPong, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cachedValue != nil {
		return c.cachedValue, nil
	}

	var addr string
	switch c.connCfg.AddressKind {
	case connection.AddressKindProxy:
		// Use the proxy address as-is.
		addr = c.connCfg.Address
	case connection.AddressKindAuth:
		// Ping the auth server to determine the proxy address.
		authPong, err := c.client.Ping(ctx)
		if err != nil {
			c.logger.DebugContext(ctx, "Failed to ping auth server", "error", err)
			return nil, trace.Wrap(err)
		}
		addr = authPong.ProxyPublicAddr
	default:
		return nil, trace.BadParameter("unsupported address kind: %v", c.connCfg.AddressKind)
	}

	c.logger.DebugContext(ctx, "Pinging proxy", "addr", addr)
	res, err := webclient.Find(&webclient.Config{
		Context:   ctx,
		ProxyAddr: addr,
		Insecure:  c.connCfg.Insecure,
	})
	if err != nil {
		c.logger.ErrorContext(ctx, "Failed to ping proxy", "error", err)
		return nil, trace.Wrap(err)
	}
	c.logger.DebugContext(ctx, "Successfully pinged proxy", "pong", res)

	c.cachedValue = &connection.ProxyPong{PingResponse: res}
	if c.connCfg.AddressKind == connection.AddressKindProxy && c.connCfg.StaticProxyAddress {
		c.cachedValue.StaticProxyAddress = addr
	}
	return c.cachedValue, nil
}

var _ connection.ProxyPinger = (*CachingProxyPinger)(nil)
