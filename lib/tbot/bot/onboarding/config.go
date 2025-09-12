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
	// RegistrationSecret is the name of the initial joining secret, if any. If
	// not specified, a keypair must be created using `tbot keypair create` and
	// registered with Teleport in advance.
	RegistrationSecret string `yaml:"registration_secret,omitempty"`
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
