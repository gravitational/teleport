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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/join/boundkeypair"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/tbot/cli"
	"github.com/gravitational/teleport/lib/tbot/config"
)

// getSuiteFromProxy fetches cryptosuite config from the given remote proxy.
func getSuiteFromProxy(proxyAddr string, insecure bool) cryptosuites.GetSuiteFunc {
	// TODO: It's annoying to need to specify a proxy here. This won't be needed
	// for keypairs generated at normal runtime since we'll have a proxy address
	// available, but alternatives should be explored, since this UX is not
	// good.
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

// KeypairDocument is the JSON struct printed to stdout when `--format=json` is
// specified.
type KeypairDocument struct {
	PublicKey string `json:"public_key"`
}

// printKeypair prints the current keypair from the given client state using the
// specified format.
func printKeypair(state *boundkeypair.ClientState, format string) error {
	publicKeyBytes, err := state.ToPublicKeyBytes()
	if err != nil {
		return trace.Wrap(err)
	}

	keyString := strings.TrimSpace(string(publicKeyBytes))

	switch format {
	case teleport.Text:
		// TODO: maybe just print out an example token resource to copy and paste? Or a tctl command.
		fmt.Printf(
			"\nTo register the keypair with Teleport, include this public key in the token's\n"+
				"`spec.bound_keypair.onboarding.initial_public_key`:\n\n"+
				"\t%s\n\n",
			keyString,
		)
	case teleport.JSON:
		bytes, err := json.Marshal(&KeypairDocument{
			PublicKey: keyString,
		})
		if err != nil {
			return trace.Wrap(err, "generating json")
		}

		fmt.Printf("%s\n", string(bytes))
	default:
		return trace.BadParameter("unsupported output format %s; keypair has been generated", format)
	}

	return nil
}

// onKeypairCreate command handles `tbot keypair create`
func onKeypairCreateCommand(ctx context.Context, globals *cli.GlobalArgs, cmd *cli.KeypairCreateCommand) error {
	dest, err := config.DestinationFromURI(cmd.Storage)
	if err != nil {
		return trace.Wrap(err, "parsing storage URI")
	}

	if err := dest.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "initializing storage")
	}

	fsAdapter := config.NewBoundkeypairDestinationAdapter(dest)

	// Check for existing client state.
	state, err := boundkeypair.LoadClientState(ctx, fsAdapter)
	if err == nil {
		if !cmd.Overwrite {
			log.InfoContext(ctx, "Existing client state found, printing existing public key. To generate a new key, pass --overwrite")
			return trace.Wrap(printKeypair(state, cmd.Format))
		} else {
			log.WarnContext(ctx, "Overwriting existing client state and generating a new keypair.")
		}
	}

	state, err = boundkeypair.NewUnboundClientState(
		ctx,
		fsAdapter,
		getSuiteFromProxy(cmd.ProxyServer, globals.Insecure),
	)
	if err != nil {
		return trace.Wrap(err, "initializing new client state")
	}

	if err := state.Store(ctx); err != nil {
		return trace.Wrap(err, "writing bound keypair state")
	}

	log.InfoContext(
		ctx,
		"keypair has been written to storage",
		"storage", dest.String(),
	)

	return trace.Wrap(printKeypair(state, cmd.Format))
}
