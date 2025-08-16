/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
)

// PluginStaticCredentials are static credentials for plugins.
type PluginStaticCredentials interface {
	// ResourceWithLabels provides common resource methods.
	ResourceWithLabels

	// GetAPIToken will return the attached API token if possible or empty if it is not present.
	GetAPIToken() (apiToken string)

	// GetBasicAuth will return the attached username and password. If they are not present, both
	// the username and password will be mpty.
	GetBasicAuth() (username string, password string)

	// GetOAuthClientID will return the attached client ID. If it is not present, the client ID
	// will be empty.
	GetOAuthClientID() (clientID string)

	// GetOAuthClientSecret will return the attached client ID and client secret. IF they are not
	// present, the client ID and client secret will be empty.
	GetOAuthClientSecret() (clientID string, clientSecret string)

	// GetSSHCertAuthorities will return the attached SSH CA keys.
	GetSSHCertAuthorities() []*SSHKeyPair

	// GetPrivateKey will return the attached private key. If it is not present, the private key will
	// be empty.
	GetPrivateKey() []byte

	// Clone returns a copy of the credentials.
	Clone() PluginStaticCredentials
}

// NewPluginStaticCredentials creates a new PluginStaticCredentialsV1 resource.
func NewPluginStaticCredentials(metadata Metadata, spec PluginStaticCredentialsSpecV1) (PluginStaticCredentials, error) {
	p := &PluginStaticCredentialsV1{
		ResourceHeader: ResourceHeader{
			Metadata: metadata,
		},
		Spec: &spec,
	}

	if err := p.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return p, nil
}

// Clone returns a copy of the credentials.
func (p *PluginStaticCredentialsV1) Clone() PluginStaticCredentials {
	return utils.CloneProtoMsg(p)
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults.
func (p *PluginStaticCredentialsV1) CheckAndSetDefaults() error {
	p.setStaticFields()

	if err := p.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	switch credentials := p.Spec.Credentials.(type) {
	case *PluginStaticCredentialsSpecV1_APIToken:
		if credentials.APIToken == "" {
			return trace.BadParameter("api token object is missing")
		}
	case *PluginStaticCredentialsSpecV1_BasicAuth:
		if credentials.BasicAuth == nil {
			return trace.BadParameter("basic auth object is missing")
		}

		if credentials.BasicAuth.Username == "" {
			return trace.BadParameter("username is empty")
		}

		if credentials.BasicAuth.Password == "" {
			return trace.BadParameter("password is empty")
		}
	case *PluginStaticCredentialsSpecV1_OAuthClientSecret:
		if credentials.OAuthClientSecret == nil {
			return trace.BadParameter("oauth client secret object is missing")
		}

		if credentials.OAuthClientSecret.ClientId == "" {
			return trace.BadParameter("client ID is empty")
		}

		if credentials.OAuthClientSecret.ClientSecret == "" {
			return trace.BadParameter("client secret is empty")
		}
	case *PluginStaticCredentialsSpecV1_SSHCertAuthorities:
		if credentials.SSHCertAuthorities == nil {
			return trace.BadParameter("SSH CAs are missing")
		}
		for _, ca := range credentials.SSHCertAuthorities.CertAuthorities {
			if err := ca.CheckAndSetDefaults(); err != nil {
				return trace.Wrap(err, "invalid SSH CA")
			}
		}
	case *PluginStaticCredentialsSpecV1_PrivateKey:
		if credentials.PrivateKey == nil {
			return trace.BadParameter("private key object is missing")
		}
		if len(credentials.PrivateKey) == 0 {
			return trace.BadParameter("private key is empty")
		}
	default:
		return trace.BadParameter("credentials are not set or have an unknown type %T", credentials)
	}

	return nil
}

// setStaticFields sets static fields for the object.
func (p *PluginStaticCredentialsV1) setStaticFields() {
	p.Kind = KindPluginStaticCredentials
	p.Version = V1
}

// GetAPIToken will return the attached API token if possible or empty if it is not present.
func (p *PluginStaticCredentialsV1) GetAPIToken() (apiToken string) {
	credentials, ok := p.Spec.Credentials.(*PluginStaticCredentialsSpecV1_APIToken)
	if !ok {
		return ""
	}

	return credentials.APIToken
}

// GetBasicAuth will return the attached username and password. If they are not present, both
// the username and password will be mpty.
func (p *PluginStaticCredentialsV1) GetBasicAuth() (username string, password string) {
	credentials, ok := p.Spec.Credentials.(*PluginStaticCredentialsSpecV1_BasicAuth)
	if !ok {
		return "", ""
	}

	return credentials.BasicAuth.Username, credentials.BasicAuth.Password
}

// GetOAuthClientID will return the attached client ID. If it is not present, the client ID will be
// empty.
func (p *PluginStaticCredentialsV1) GetOAuthClientID() (clientID string) {
	credentials, ok := p.Spec.Credentials.(*PluginStaticCredentialsSpecV1_OAuthClientSecret)
	if !ok {
		return ""
	}

	return credentials.OAuthClientSecret.ClientId
}

// GetOAuthClientSecret will return the attached client ID and client secret. IF they are not
// present, the client ID and client secret will be empty.
func (p *PluginStaticCredentialsV1) GetOAuthClientSecret() (clientID string, clientSecret string) {
	credentials, ok := p.Spec.Credentials.(*PluginStaticCredentialsSpecV1_OAuthClientSecret)
	if !ok {
		return "", ""
	}

	return credentials.OAuthClientSecret.ClientId, credentials.OAuthClientSecret.ClientSecret
}

// GetSSHCertAuthorities will return the attached SSH CA keys.
func (p *PluginStaticCredentialsV1) GetSSHCertAuthorities() []*SSHKeyPair {
	credentials, ok := p.Spec.Credentials.(*PluginStaticCredentialsSpecV1_SSHCertAuthorities)
	if !ok {
		return nil
	}
	return credentials.SSHCertAuthorities.CertAuthorities
}

// GetPrivateKey will return the attached private key. If it is not present, the private key will
// be empty.
func (p *PluginStaticCredentialsV1) GetPrivateKey() []byte {
	credentials, ok := p.Spec.Credentials.(*PluginStaticCredentialsSpecV1_PrivateKey)
	if !ok {
		return nil
	}

	return credentials.PrivateKey
}

// MatchSearch is a dummy value as credentials are not searchable.
func (p *PluginStaticCredentialsV1) MatchSearch(_ []string) bool {
	return false
}
