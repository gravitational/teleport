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
	"crypto"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/join/boundkeypair"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/bot/onboarding"
	"github.com/gravitational/teleport/lib/tbot/botfs"
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

	PrivateKey string `json:"private_key,omitempty"`
}

// KeypairMessageParams are parameters used with `keypairMessageTemplate`
type KeypairMessageParams struct {
	PublicKey         string
	EnvName           string
	EncodedPrivateKey string
	StaticKeyPath     string
}

var keypairMessageTemplate = template.Must(template.New("keypair_message").Parse(`
To register the keypair with Teleport, include this public key in the token's
'spec.bound_keypair.onboarding.initial_public_key' field:

	{{ .PublicKey }}
{{ if .StaticKeyPath }}
Note that you must also set 'spec.bound_keypair.recovery.mode' to 'insecure'
to use static keys.

The static key has been written to: {{ .StaticKeyPath }}

Configure your bot to use this static key by setting the following 'tbot.yaml'
field:
	onboarding:
	  join_method: bound_keypair
	  bound_keypair:
	    static_private_key_path: {{ .StaticKeyPath }}
{{ else if .EncodedPrivateKey }}
Configure your bot to use this static key by inserting the following private key
value into the bot's environment, ideally via a platform-specific keystore if
available:
	export {{ .EnvName }}={{ .EncodedPrivateKey }}
{{ end }}
`))

func printKeypair(params KeypairMessageParams, format string) error {
	switch format {
	case teleport.Text:
		if err := keypairMessageTemplate.Execute(os.Stdout, params); err != nil {
			return trace.Wrap(err)
		}
	case teleport.JSON:
		bytes, err := json.Marshal(&KeypairDocument{
			PublicKey:  params.PublicKey,
			PrivateKey: params.EncodedPrivateKey,
		})
		if err != nil {
			return trace.Wrap(err, "generating json")
		}

		fmt.Printf("%s\n", string(bytes))
	default:
		return trace.BadParameter("unsupported output format %s; keypair has not been generated", format)
	}

	return nil
}

// printKeypairFromState prints the current keypair from the given client state using the
// specified format.
func printKeypairFromState(state *boundkeypair.FSClientState, format string) error {
	publicKeyBytes, err := state.ToPublicKeyBytes()
	if err != nil {
		return trace.Wrap(err)
	}

	keyString := strings.TrimSpace(string(publicKeyBytes))
	return trace.Wrap(printKeypair(KeypairMessageParams{
		PublicKey: keyString,
	}, format))
}

// generateStaticKeypair generates a static keypair, used when --static is set
func generateStaticKeypair(ctx context.Context, globals *cli.GlobalArgs, cmd *cli.KeypairCreateCommand) error {
	var key crypto.Signer
	var err error

	if cmd.StaticPath != "" {
		bytes, err := os.ReadFile(cmd.StaticPath)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err, "could not read from static key path %s", cmd.StaticPath)
		}

		parsed, err := keys.ParsePrivateKey(bytes)
		if err != nil {
			return trace.Wrap(err, "could not parse existing key at static key path %s", cmd.StaticPath)
		}

		// MarshalPrivateKey expects an actual signer impl, so unpack it.
		key = parsed.Signer
		log.InfoContext(ctx, "Loaded existing static key from path", "path", cmd.StaticPath)
	}

	if key == nil || cmd.Overwrite {
		if key != nil {
			log.WarnContext(
				ctx,
				"An existing static key was found at the specified path and will be overwritten",
				"static_path", cmd.StaticPath,
			)
		}

		key, err = cryptosuites.GenerateKey(
			ctx,
			getSuiteFromProxy(cmd.ProxyServer, globals.Insecure),
			cryptosuites.BoundKeypairJoining,
		)
		if err != nil {
			return trace.Wrap(err, "generating keypair")
		}
	} else {
		log.InfoContext(ctx, "An existing static key was found at the given path and will be printed. To generate a new key, pass --overwrite")
	}

	privateKeyBytes, err := keys.MarshalPrivateKey(key)
	if err != nil {
		return trace.Wrap(err, "marshaling private key")
	}

	encodedPrivateKey := base64.StdEncoding.EncodeToString(privateKeyBytes)

	sshPubKey, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		return trace.Wrap(err, "creating ssh public key")
	}

	publicKeyBytes := ssh.MarshalAuthorizedKey(sshPubKey)
	publicKeyString := strings.TrimSpace(string(publicKeyBytes))

	if cmd.StaticPath != "" {
		if err := os.WriteFile(cmd.StaticPath, privateKeyBytes, botfs.DefaultMode); err != nil {
			return trace.Wrap(err, "writing static key to %s", cmd.StaticPath)
		}

		log.InfoContext(
			ctx,
			"A static keypair has been written to the specified static key path",
			"path", cmd.StaticPath,
		)
	}

	return trace.Wrap(printKeypair(KeypairMessageParams{
		PublicKey:         publicKeyString,
		EnvName:           onboarding.BoundKeypairStaticKeyEnv,
		EncodedPrivateKey: encodedPrivateKey,
		StaticKeyPath:     cmd.StaticPath,
	}, cmd.Format))
}

// onKeypairCreate command handles `tbot keypair create`
func onKeypairCreateCommand(ctx context.Context, globals *cli.GlobalArgs, cmd *cli.KeypairCreateCommand) error {
	if cmd.Static {
		return trace.Wrap(generateStaticKeypair(ctx, globals, cmd))
	}

	if cmd.Storage == "" {
		return trace.BadParameter("a storage path must be provided with --storage")
	}

	dest, err := config.DestinationFromURI(cmd.Storage)
	if err != nil {
		return trace.Wrap(err, "parsing storage URI")
	}

	if err := dest.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "initializing storage")
	}

	fsAdapter := destination.NewBoundkeypairDestinationAdapter(dest)

	// Check for existing client state.
	state, err := boundkeypair.LoadClientState(ctx, fsAdapter)
	if err == nil {
		if !cmd.Overwrite {
			log.InfoContext(ctx, "Existing client state found, printing existing public key. To generate a new key, pass --overwrite")
			return trace.Wrap(printKeypairFromState(state, cmd.Format))
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

	return trace.Wrap(printKeypairFromState(state, cmd.Format))
}
