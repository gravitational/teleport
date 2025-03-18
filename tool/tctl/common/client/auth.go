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

package client

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	libmfa "github.com/gravitational/teleport/lib/client/mfa"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/common"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// InitFunc initiates connection to auth service, makes ping request and return the client instance.
// If the function does not return an error, the caller is responsible for calling the client close function
// once it does not need the client anymore.
type InitFunc func(ctx context.Context) (client *authclient.Client, close func(context.Context), err error)

// GetInitFunc wraps lazy loading auth init function for commands which requires the auth client.
func GetInitFunc(ccf tctlcfg.GlobalCLIFlags, cfg *servicecfg.Config) InitFunc {
	return func(ctx context.Context) (*authclient.Client, func(context.Context), error) {
		clientConfig, err := tctlcfg.ApplyConfig(&ccf, cfg)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		resolver, err := reversetunnelclient.CachingResolver(
			ctx,
			reversetunnelclient.WebClientResolver(&webclient.Config{
				Context:   ctx,
				ProxyAddr: clientConfig.AuthServers[0].String(),
				Insecure:  clientConfig.Insecure,
				Timeout:   clientConfig.DialTimeout,
			}),
			nil /* clock */)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		dialer, err := reversetunnelclient.NewTunnelAuthDialer(reversetunnelclient.TunnelAuthDialerConfig{
			Resolver:              resolver,
			ClientConfig:          clientConfig.SSH,
			Log:                   clientConfig.Log,
			InsecureSkipTLSVerify: clientConfig.Insecure,
			ClusterCAs:            clientConfig.TLS.RootCAs,
		})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		clientConfig.ProxyDialer = dialer

		client, err := authclient.Connect(ctx, clientConfig)
		if err != nil {
			if utils.IsUntrustedCertErr(err) {
				err = trace.WrapWithMessage(err, utils.SelfSignedCertsMsg)
			}
			fmt.Fprintf(os.Stderr,
				"ERROR: Cannot connect to the auth server. Is the auth server running on %q?\n",
				cfg.AuthServerAddresses()[0].Addr)
			return nil, nil, trace.NewAggregate(&common.ExitCodeError{Code: 1}, err)
		}

		// Get the proxy address and set the MFA prompt constructor.
		resp, err := client.Ping(ctx)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		proxyAddr := resp.ProxyPublicAddr
		client.SetMFAPromptConstructor(func(opts ...mfa.PromptOpt) mfa.Prompt {
			promptCfg := libmfa.NewPromptConfig(proxyAddr, opts...)
			return libmfa.NewCLIPrompt(&libmfa.CLIPromptConfig{
				PromptConfig: *promptCfg,
			})
		})

		return client, func(ctx context.Context) {
			ctx, cancel := context.WithTimeout(ctx, constants.TimeoutGetClusterAlerts)
			defer cancel()
			if err := common.ShowClusterAlerts(ctx, client, os.Stderr, nil,
				types.AlertSeverity_HIGH); err != nil {
				slog.WarnContext(ctx, "Failed to display cluster alerts.", "error", err)
			}
			if err := client.Close(); err != nil {
				slog.WarnContext(ctx, "Failed to close client.", "error", err)
			}
		}, nil
	}
}
