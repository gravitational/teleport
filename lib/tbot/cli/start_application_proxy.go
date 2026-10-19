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

package cli

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/services/application"
)

// ApplicationProxyCommand implements `tbot start application-proxy` and
// `tbot configure application-proxy`.
type ApplicationProxyCommand struct {
	*sharedStartArgs
	*genericMutatorHandler

	Listen string

	HTTPDialTimeout                   time.Duration
	HTTPIdleTimeout                   time.Duration
	HTTPReadHeaderTimeout             time.Duration
	HTTPReadTimeout                   time.Duration
	httpReadTimeoutSetByUser          bool
	httpWriteTimeoutSetByUser         bool
	HTTPWriteTimeout                  time.Duration
	HTTPUpstreamDisableKeepAlives     bool
	HTTPUpstreamIdleConnTimeout       time.Duration
	HTTPUpstreamMaxIdleConns          int
	HTTPUpstreamRequestTimeout        time.Duration
	HTTPUpstreamResponseHeaderTimeout time.Duration
}

// NewApplicationProxyCommand initializes flags for an app proxy command and
// returns a struct to contain the parse result.
func NewApplicationProxyCommand(parentCmd *kingpin.CmdClause, action MutatorAction, mode CommandMode) *ApplicationProxyCommand {
	cmd := parentCmd.Command("application-proxy", fmt.Sprintf("%s tbot with a HTTP application proxy.", mode)).Alias("app-proxy")

	c := &ApplicationProxyCommand{}
	c.sharedStartArgs = newSharedStartArgs(cmd)
	c.genericMutatorHandler = newGenericMutatorHandler(cmd, c, action)

	cmd.Flag("listen", "The socket URI on which the local proxy should listen, such as `tcp://0.0.0.0:8080`.").Required().StringVar(&c.Listen)

	cmd.Flag("http-dial-timeout", "The maximum duration for establishing a new upstream connection.").DurationVar(&c.HTTPDialTimeout)
	cmd.Flag("http-idle-timeout", "The maximum wait for the next incoming HTTP request on a keep-alive connection.").DurationVar(&c.HTTPIdleTimeout)
	cmd.Flag("http-read-header-timeout", "The maximum time to read the incoming HTTP request headers.").DurationVar(&c.HTTPReadHeaderTimeout)
	cmd.Flag("http-read-timeout", "The maximum time to read the entire incoming HTTP request, including headers and body.").IsSetByUser(&c.httpReadTimeoutSetByUser).DurationVar(&c.HTTPReadTimeout)
	cmd.Flag("http-write-timeout", "The maximum time to write the response, measured from completion of incoming request header reading.").IsSetByUser(&c.httpWriteTimeoutSetByUser).DurationVar(&c.HTTPWriteTimeout)
	cmd.Flag("http-upstream-disable-keep-alives", "Disable upstream HTTP connection reuse for troubleshooting. Each request establishes a new connection, adding TCP and TLS handshake overhead. Defaults to false.").BoolVar(&c.HTTPUpstreamDisableKeepAlives)
	cmd.Flag("http-upstream-idle-conn-timeout", "The maximum amount of time an idle upstream keep-alive connection will remain idle before closing itself. Has no effect when --http-upstream-disable-keep-alives is set. Defaults to 30s if omitted or zero. Must not exceed 20m.").DurationVar(&c.HTTPUpstreamIdleConnTimeout)
	cmd.Flag("http-upstream-max-idle-conns", "The maximum number of idle upstream keep-alive connections retained across all applications. Has no effect when --http-upstream-disable-keep-alives is set. Defaults to 100 if omitted or zero. Must not exceed 1000.").IntVar(&c.HTTPUpstreamMaxIdleConns)
	cmd.Flag("http-upstream-request-timeout", "The maximum duration for the upstream HTTP exchange, including connection establishment, redirects, and reading the response body.").DurationVar(&c.HTTPUpstreamRequestTimeout)
	cmd.Flag("http-upstream-response-header-timeout", "The maximum wait for upstream HTTP response headers after the request, including its body, has been fully written.").DurationVar(&c.HTTPUpstreamResponseHeaderTimeout)

	return c
}

func (c *ApplicationProxyCommand) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	if err := c.sharedStartArgs.ApplyConfig(cfg, l); err != nil {
		return trace.Wrap(err)
	}

	proxyServiceHTTPConfig := application.ProxyServiceHTTPConfig{
		DialTimeout:                   c.HTTPDialTimeout,
		IdleTimeout:                   c.HTTPIdleTimeout,
		ReadHeaderTimeout:             c.HTTPReadHeaderTimeout,
		UpstreamDisableKeepAlives:     c.HTTPUpstreamDisableKeepAlives,
		UpstreamIdleConnTimeout:       c.HTTPUpstreamIdleConnTimeout,
		UpstreamMaxIdleConns:          c.HTTPUpstreamMaxIdleConns,
		UpstreamRequestTimeout:        c.HTTPUpstreamRequestTimeout,
		UpstreamResponseHeaderTimeout: c.HTTPUpstreamResponseHeaderTimeout,
	}
	if c.httpReadTimeoutSetByUser {
		proxyServiceHTTPConfig.ReadTimeout = &c.HTTPReadTimeout
	}
	if c.httpWriteTimeoutSetByUser {
		proxyServiceHTTPConfig.WriteTimeout = &c.HTTPWriteTimeout
	}

	cfg.Services = append(cfg.Services, &application.ProxyServiceConfig{
		Listen: c.Listen,
		HTTP:   proxyServiceHTTPConfig,
	})

	return nil
}
