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

import "github.com/gravitational/trace"

// PluginStaticCredentials are static credentials for plugins.
type PluginStaticCredentials interface {
	// ResourceWithLabels provides common resource methods.
	ResourceWithLabels

	// GetAPIToken will return a flag indicating whether this object contains an API token
	// credentials and the attached API token if possible.
	GetAPIToken() (apiToken string)

	// GetBasicAuth will return a flag indicating whether this object contains basic auth
	// credentials and the attached username and password if possible.
	GetBasicAuth() (username string, password string)

	// GetOAuthClientSecret will return a flag indicating whether this object contains OAuth
	// client secret credentials and the attached client ID and client secret.
	GetOAuthClientSecret() (clientID string, clientSecret string)
}

// NewPluginStaticCredentials creates a new PluginStaticCredentialsV1 resource.
func NewPluginStaticCredentials(metadata Metadata, spec PluginStaticCredentialsSpecV1) (PluginStaticCredentials, error) {
	p := &PluginStaticCredentialsV1{
		ResourceHeader: ResourceHeader{
			Metadata: metadata,
		},
		Spec: spec,
	}

	if err := p.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return p, nil
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
	case *PluginStaticCredentialsSpecV1_OauthClientSecret:
		if credentials.OauthClientSecret == nil {
			return trace.BadParameter("oauth client secret object is missing")
		}

		if credentials.OauthClientSecret.ClientId == "" {
			return trace.BadParameter("client ID is empty")
		}

		if credentials.OauthClientSecret.ClientSecret == "" {
			return trace.BadParameter("client secret is empty")
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

// GetAPIToken will return a flag indicating whether this object contains an API token
// credentials and the attached API token if possible.
func (p *PluginStaticCredentialsV1) GetAPIToken() (apiToken string) {
	credentials, ok := p.Spec.Credentials.(*PluginStaticCredentialsSpecV1_APIToken)
	if !ok {
		return ""
	}

	return credentials.APIToken
}

// GetBasicAuth will return a flag indicating whether this object contains basic auth
// credentials and the attached username and password if possible.
func (p *PluginStaticCredentialsV1) GetBasicAuth() (username string, password string) {
	credentials, ok := p.Spec.Credentials.(*PluginStaticCredentialsSpecV1_BasicAuth)
	if !ok {
		return "", ""
	}

	return credentials.BasicAuth.Username, credentials.BasicAuth.Password
}

// GetOAuthClientSecret will return a flag indicating whether this object contains OAuth
// client secret credentials and the attached client ID and client secret.
func (p *PluginStaticCredentialsV1) GetOAuthClientSecret() (clientID string, clientSecret string) {
	credentials, ok := p.Spec.Credentials.(*PluginStaticCredentialsSpecV1_OauthClientSecret)
	if !ok {
		return "", ""
	}

	return credentials.OauthClientSecret.ClientId, credentials.OauthClientSecret.ClientSecret
}

// MatchSearch is a dummy value as credentials are not searchable.
func (p *PluginStaticCredentialsV1) MatchSearch(_ []string) bool {
	return false
}
