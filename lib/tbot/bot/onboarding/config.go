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

package onboarding

import (
	"context"
	"encoding/base64"
	"log/slog"
	"os"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// SupportedJoinMethods are the supported methods by which bots may join the
// cluster.
var SupportedJoinMethods = []string{
	string(types.JoinMethodAzure),
	string(types.JoinMethodAzureDevops),
	string(types.JoinMethodBitbucket),
	string(types.JoinMethodCircleCI),
	string(types.JoinMethodGCP),
	string(types.JoinMethodGitHub),
	string(types.JoinMethodGitLab),
	string(types.JoinMethodIAM),
	string(types.JoinMethodKubernetes),
	string(types.JoinMethodSpacelift),
	string(types.JoinMethodToken),
	string(types.JoinMethodTPM),
	string(types.JoinMethodTerraformCloud),
	string(types.JoinMethodOracle),
	string(types.JoinMethodBoundKeypair),
}

const (
	// registrationSecretEnv is an environment variable that contains a
	// registration secret for bound keypair joining.
	registrationSecretEnv = "TBOT_REGISTRATION_SECRET"

	// BoundKeypairStaticKeyEnv is an env var that if set, contains a base64
	// encoded private key (with a nested PEM encoded private key).
	BoundKeypairStaticKeyEnv = "TBOT_BOUND_KEYPAIR_STATIC_KEY"
)

// AzureOnboardingConfig holds configuration relevant to the "azure" join method.
type AzureOnboardingConfig struct {
	// ClientID of the managed identity to use. Required if the VM has more
	// than one assigned identity.
	ClientID string `yaml:"client_id,omitempty"`
}

// TerraformOnboardingConfig contains parameters for the "terraform" join method
type TerraformOnboardingConfig struct {
	// TokenTag is the name of the tag configured via the environment variable
	// `TERRAFORM_WORKLOAD_IDENTITY_AUDIENCE(_$TAG)`. If unset, the untagged
	// variant is used.
	AudienceTag string `yaml:"audience_tag,omitempty"`
}

// GitlabOnboardingConfig holds configuration relevant to the "gitlab" join method.
type GitlabOnboardingConfig struct {
	// TokenEnvVarName is the name of the environment variable that contains the
	// GitLab ID token. This can be useful to override in cases where a single
	// gitlab job needs to authenticate to multiple Teleport clusters.
	TokenEnvVarName string `yaml:"token_env_var_name,omitempty"`
}

// BoundKeypairOnboardingConfig contains parameters for the `bound_keypair` join
// method
type BoundKeypairOnboardingConfig struct {
	// RegistrationSecretValue is the name of the initial joining secret, if
	// any. If not specified, a keypair must be created using `tbot keypair
	// create` and registered with Teleport in advance. This can either be a
	// static value or an absolute path to a file containing the secret value.
	RegistrationSecretValue string `yaml:"registration_secret,omitempty"`

	// RegistrationSecretPath is a path to a registration secret to be read from
	// a file.
	RegistrationSecretPath string `yaml:"registration_secret_path,omitempty"`

	// StaticPrivateKeyPath is a static private key, containing either a path to
	// a private key file. Unlike keys managed automatically in the bot storage,
	// this will be treated as immutable. It must be preregistered, does not
	// support rotation, and must be used with a token in `insecure` recovery
	// mode.
	StaticPrivateKeyPath string `yaml:"static_private_key_path,omitempty"`
}

// RegistrationSecret returns the registration secret, if set. If the value
// appears to be an absolute filepath and points to a real file on the system,
// the contents of that file will be returned; otherwise, the literal value is
// returned. If `TBOT_REGISTRATION_SECRET` is set and neither explicit config
// value is set (CLI, YAML) that value will be returned.
func (c *BoundKeypairOnboardingConfig) RegistrationSecret() (string, error) {
	if c.RegistrationSecretValue != "" && c.RegistrationSecretPath != "" {
		return "", trace.BadParameter("only one of 'registration_secret' and 'registration_secret_path' may be set")
	}

	env, envExists := os.LookupEnv(registrationSecretEnv)

	switch {
	case c.RegistrationSecretPath != "":
		if envExists {
			slog.WarnContext(
				context.Background(),
				"'registration_secret_path' in tbot's configuration will override the value set in the environment",
				"env", registrationSecretEnv,
				"path", c.RegistrationSecretPath,
			)
		}

		bytes, err := os.ReadFile(c.RegistrationSecretPath)
		if err != nil {
			return "", trace.ConvertSystemError(err)
		}

		slog.DebugContext(context.Background(), "loading registration secret from file", "path", c.RegistrationSecretPath)

		return strings.TrimSpace(string(bytes)), nil
	case c.RegistrationSecretValue != "":
		if envExists {
			slog.WarnContext(
				context.Background(),
				"'registration_secret' in tbot's configuration will override the value set in the environment",
				"env", registrationSecretEnv,
			)
		}

		slog.DebugContext(context.Background(), "using registration secret from config file or CLI")

		return c.RegistrationSecretValue, nil
	case envExists:
		slog.DebugContext(context.Background(), "using registration secret from environment", "env", registrationSecretEnv)
		return env, nil
	default:
		return "", nil
	}
}

// StaticPrivateKeyBytes returns a statically configured private key for bound
// keypair joining. If not nil, this value should be used as an immutable
// `StaticClientState` instead of a traditional `FSClientState` which would
// otherwise mutably write to the bot storage directory. These static keys do
// not support rotation or join state verification.
//
// Users can either configure `static_private_key_path` in the bound keypair
// onboarding config, or by inserting a base64-encoded private key (in PEM
// format) into the `TBOT_BOUND_KEYPAIR_STATIC_KEY` environment variable. The
// configuration value supersedes the environment variable if both are set.
func (c *BoundKeypairOnboardingConfig) StaticPrivateKeyBytes() ([]byte, error) {
	if c.StaticPrivateKeyPath != "" {
		bytes, err := os.ReadFile(c.StaticPrivateKeyPath)
		if err != nil {
			return nil, trace.Wrap(err, "reading static key from %s", c.StaticPrivateKeyPath)
		}

		return bytes, nil
	}

	if env, envExists := os.LookupEnv(BoundKeypairStaticKeyEnv); envExists {
		bytes, err := base64.StdEncoding.DecodeString(env)
		if err != nil {
			return nil, trace.Wrap(err, "decoding private key from environment")
		}

		return bytes, nil
	}

	return nil, nil
}

// Config contains values relevant to how the bot authenticates with
// and joins the Teleport cluster.
type Config struct {
	// TokenValue is either the token needed to join the auth server, or a path pointing to a file
	// that contains the token
	//
	// You should use Token() instead - this has to be an exported field for YAML unmarshaling
	// to work correctly, but this could be a path instead of a token
	TokenValue string `yaml:"token,omitempty"`

	// CAPath is an optional path to a CA certificate.
	CAPath string `yaml:"ca_path,omitempty"`

	// CAPins is a list of certificate authority pins, used to validate the
	// connection to the Teleport auth server.
	CAPins []string `yaml:"ca_pins,omitempty"`

	// JoinMethod is the method the bot should use to exchange a token for the
	// initial certificate
	JoinMethod types.JoinMethod `yaml:"join_method"`

	// Azure holds configuration relevant to the azure joining method.
	Azure AzureOnboardingConfig `yaml:"azure,omitempty"`

	// Terraform holds configuration relevant to the `terraform` join method.
	Terraform TerraformOnboardingConfig `yaml:"terraform,omitempty"`

	// Gitlab holds configuration relevant to the `gitlab` join method.
	Gitlab GitlabOnboardingConfig `yaml:"gitlab,omitempty"`

	// BoundKeypair holds configuration relevant to the `bound_keypair` join method
	BoundKeypair BoundKeypairOnboardingConfig `yaml:"bound_keypair,omitempty"`
}

// HasToken gives the ability to check if there has been a token value stored
// in the config
func (conf *Config) HasToken() bool {
	return conf.TokenValue != ""
}

// SetToken stores the value for --token or auth_token in the config
//
// In the case of the token value pointing to a file, this allows us to
// fetch the value of the token when it's needed (when connecting for the first time)
// instead of trying to read the file every time that teleport is launched.
// This means we can allow temporary token files that are removed after teleport has
// successfully connected the first time.
func (conf *Config) SetToken(token string) {
	conf.TokenValue = token
}

// Token returns token needed to join the auth server
//
// If the value stored points to a file, it will attempt to read the token value from the file
// and return an error if it wasn't successful
// If the value stored doesn't point to a file, it'll return the value stored
func (conf *Config) Token() (string, error) {
	token, err := utils.TryReadValueAsFile(conf.TokenValue)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return token, nil
}
