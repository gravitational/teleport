/*
Copyright 2026 Gravitational, Inc.

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

package userexternalcredentials

import (
	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	userexternalcredentialsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
	"github.com/gravitational/teleport/api/types"
)

// NewGitHubOAuth creates a new UserExternalCredentials resource for GitHub OAuth.
// name should be the GitHub App client_id.
func NewGitHubOAuth(user, name string, creds *userexternalcredentialsv1.GitHubOAuthCredentials) (*userexternalcredentialsv1.UserExternalCredentials, error) {
	c := userexternalcredentialsv1.UserExternalCredentials_builder{
		Kind:    types.KindUserExternalCredentials,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: userexternalcredentialsv1.UserExternalCredentialsSpec_builder{
			User:        user,
			GithubOauth: creds,
		}.Build(),
	}.Build()

	if err := Validate(c); err != nil {
		return nil, trace.Wrap(err)
	}
	return c, nil
}

// Validate checks that required parameters are set.
func Validate(c *userexternalcredentialsv1.UserExternalCredentials) error {
	switch {
	case c.GetKind() != types.KindUserExternalCredentials:
		return trace.BadParameter("wrong kind: %q", c.GetKind())
	case c.GetVersion() != types.V1:
		return trace.BadParameter("wrong version: %q", c.GetVersion())
	case c.GetMetadata().GetName() == "":
		return trace.BadParameter("missing name")
	case c.GetSpec() == nil:
		return trace.BadParameter("missing spec")
	case c.GetSpec().GetUser() == "":
		return trace.BadParameter("missing user")
	default:
		return nil
	}
}
