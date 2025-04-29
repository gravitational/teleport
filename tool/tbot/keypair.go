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

package main

import (
	"context"
	"fmt"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/join/boundkeypair"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/tbot/cli"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/trace"
)

func getSuiteFromProxy(proxyAddr string, insecure bool) cryptosuites.GetSuiteFunc {
	return func(ctx context.Context) (types.SignatureAlgorithmSuite, error) {
		pr, err := webclient.Find(&webclient.Config{
			Context:   ctx,
			ProxyAddr: proxyAddr,
			Insecure:  insecure,
		})
		if err != nil {
			return types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED, trace.Wrap(err, "pinging proxy to determine signature algorithm suite")
		}
		return pr.Auth.SignatureAlgorithmSuite, nil
	}
}

func onKeypairCreateCommand(ctx context.Context, globals *cli.GlobalArgs, cmd *cli.KeypairCreateCommand) error {
	dest, err := config.DestinationFromURI(cmd.Storage)
	if err != nil {
		return trace.Wrap(err, "parsing storage URI")
	}

	if err := dest.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "initializing storage")
	}

	state, err := boundkeypair.NewUnboundClientState(
		ctx,
		getSuiteFromProxy(cmd.ProxyServer, globals.Insecure),
	)
	if err != nil {
		return trace.Wrap(err, "initializing new client state")
	}

	if err := boundkeypair.StoreClientState(ctx, config.NewBoundkeypairDestinationAdapter(dest), state); err != nil {
		return trace.Wrap(err, "writing bound keypair state")
	}

	log.InfoContext(
		ctx,
		"keypair has been written to storage",
		"storage", dest.String(),
	)

	publicKeyBytes, err := state.ToPublicKeyBytes()
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO: maybe just print out an example token resource to copy and paste? Or a tctl command.
	fmt.Printf(
		"\nTo register the keypair with Teleport, include this public key in the token's\n"+
			"`spec.bound_keypair.onboarding.initial_public_key`:\n\n"+
			"\t%s\n\n",
		string(publicKeyBytes),
	)

	return nil
}
