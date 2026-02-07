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
	"io"
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
	libboundkeypair "github.com/gravitational/teleport/lib/boundkeypair"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/bot/onboarding"
	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/tbot/cli"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/utils"
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
	TokenExample      string
}

var keypairMessageTemplate = template.Must(template.New("keypair_message").Parse(`
To register the keypair with Teleport, include this public key in the token's
'spec.bound_keypair.onboarding.initial_public_key' field:

	{{ .PublicKey }}

You can use 'tctl' to create a bot and token automatically:

	$ tctl bots add bot-name \
	  --roles=some-role,some-other-role \
	  --initial-public-key='{{ .PublicKey }}'{{ if or .StaticKeyPath .EncodedPrivateKey }} \
	  --recovery-mode=insecure{{ end }}

Refer to this token example as a reference:

{{ .TokenExample }}

{{- if or .EncodedPrivateKey .StaticKeyPath }}
Note that you must also set 'spec.bound_keypair.recovery.mode' to 'insecure'
to use static keys.
{{ if .StaticKeyPath }}
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
Note that bots joined with static tokens do not support keypair rotation and
will be unable to join if a rotation is requested server-side via the token's
'rotate_after' field. Additionally, 'insecure' recovery mode must be used, as
shown above. Read more at:

	https://goteleport.com/docs/reference/machine-workload-identity/machine-id/bound-keypair/concepts/#recovery
{{- end }}`))

func generateExampleToken(params KeypairMessageParams, indent string) (string, error) {
	mode := string(libboundkeypair.RecoveryModeStandard)
	if params.EncodedPrivateKey != "" {
		// EncodedPrivateKey is always set if a static key is used, even if we
		// only write the unencoded key to a file
		mode = string(libboundkeypair.RecoveryModeInsecure)
	}

	token := &types.ProvisionTokenV2{
		Version: types.V2,
		Kind:    types.KindToken,
		Metadata: types.Metadata{
			Name: "example-token",
		},
		Spec: types.ProvisionTokenSpecV2{
			Roles:      []types.SystemRole{types.RoleBot},
			BotName:    "example-bot",
			JoinMethod: types.JoinMethodBoundKeypair,
			BoundKeypair: &types.ProvisionTokenSpecV2BoundKeypair{
				Onboarding: &types.ProvisionTokenSpecV2BoundKeypair_OnboardingSpec{
					InitialPublicKey: params.PublicKey,
				},
				Recovery: &types.ProvisionTokenSpecV2BoundKeypair_RecoverySpec{
					Mode:  mode,
					Limit: 1,
				},
			},
		},
	}

	w := strings.Builder{}
	if err := utils.WriteYAML(&w, token); err != nil {
		return "", trace.Wrap(err, "generating example token spec")
	}

	if indent == "" {
		return w.String(), nil
	}

	indented := strings.Builder{}
	for line := range strings.Lines(w.String()) {
		indented.WriteString(indent + line)
	}

	return indented.String(), nil
}

func printKeypair(w io.Writer, params KeypairMessageParams, format string) error {
	example, err := generateExampleToken(params, "\t")
	if err != nil {
		return trace.Wrap(err)
	}

	params.TokenExample = example

	switch format {
	case teleport.Text:
		if err := keypairMessageTemplate.Execute(w, params); err != nil {
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

		fmt.Fprintf(w, "%s\n", string(bytes))
	default:
		return trace.BadParameter("unsupported output format %s; keypair has not been generated", format)
	}

	return nil
}

// printKeypairFromState prints the current keypair from the given client state using the
// specified format.
func printKeypairFromState(w io.Writer, state *boundkeypair.FSClientState, format string) error {
	publicKeyBytes, err := state.ToPublicKeyBytes()
	if err != nil {
		return trace.Wrap(err)
	}

	keyString := strings.TrimSpace(string(publicKeyBytes))
	return trace.Wrap(printKeypair(w, KeypairMessageParams{
		PublicKey: keyString,
	}, format))
}

func loadExistingStaticKeypair(path string) (crypto.Signer, error) {
	bytes, err := os.ReadFile(path)
	if trace.IsNotFound(err) {
		// No existing keypair was found at the path, nothing to load.
		return nil, nil
	} else if err != nil {
		return nil, trace.Wrap(err, "could not read from static key path %s", path)
	}

	parsed, err := keys.ParsePrivateKey(bytes)
	if err != nil {
		return nil, trace.Wrap(err, "could not parse existing key at static key path %s", path)
	}

	// MarshalPrivateKey expects an actual signer impl, so unpack it.
	return parsed.Signer, nil
}

// generateStaticKeypair generates a static keypair, used when --static is set
func generateStaticKeypair(ctx context.Context, globals *cli.GlobalArgs, cmd *cli.KeypairCreateCommand) error {
	var key crypto.Signer
	var err error

	if cmd.StaticKeyPath != "" {
		key, err = loadExistingStaticKeypair(cmd.StaticKeyPath)
		if err != nil {
			return trace.Wrap(err)
		}

		log.InfoContext(ctx, "Loaded existing static key from path", "path", cmd.StaticKeyPath)
	}

	if key == nil || cmd.Overwrite {
		if key != nil {
			log.WarnContext(
				ctx,
				"An existing static key was found at the specified path and will be overwritten",
				"static_path", cmd.StaticKeyPath,
			)
		}

		getSuite := cmd.GetSuite
		if getSuite == nil {
			getSuite = getSuiteFromProxy(cmd.ProxyServer, globals.Insecure)
		}

		key, err = cryptosuites.GenerateKey(
			ctx,
			getSuite,
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

	if cmd.StaticKeyPath != "" {
		if err := os.WriteFile(cmd.StaticKeyPath, privateKeyBytes, botfs.DefaultMode); err != nil {
			return trace.Wrap(err, "writing static key to %s", cmd.StaticKeyPath)
		}

		log.InfoContext(
			ctx,
			"A static keypair has been written to the specified static key path",
			"path", cmd.StaticKeyPath,
		)
	}

	w := cmd.Writer
	if w == nil {
		w = os.Stdout
	}

	return trace.Wrap(printKeypair(w, KeypairMessageParams{
		PublicKey:         publicKeyString,
		EnvName:           onboarding.BoundKeypairStaticKeyEnv,
		EncodedPrivateKey: encodedPrivateKey,
		StaticKeyPath:     cmd.StaticKeyPath,
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
	w := cmd.Writer
	if w == nil {
		w = os.Stdout
	}

	// Check for existing client state.
	state, err := boundkeypair.LoadClientState(ctx, fsAdapter)
	if err == nil {
		if !cmd.Overwrite {
			log.InfoContext(ctx, "Existing client state found, printing existing public key. To generate a new key, pass --overwrite")
			return trace.Wrap(printKeypairFromState(w, state, cmd.Format))
		} else {
			log.WarnContext(ctx, "Overwriting existing client state and generating a new keypair.")
		}
	}

	getSuite := cmd.GetSuite
	if getSuite == nil {
		getSuite = getSuiteFromProxy(cmd.ProxyServer, globals.Insecure)
	}

	state, err = boundkeypair.NewUnboundClientState(ctx, fsAdapter, getSuite)
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

	return trace.Wrap(printKeypairFromState(w, state, cmd.Format))
}
