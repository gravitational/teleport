/*
Copyright 2015 Gravitational, Inc.

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

package ui

import (
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type accessStrategy struct {
	// Type determines how a user should access teleport resources.
	// ie: does the user require a request to access resources?
	Type types.RequestStrategy `json:"type"`
	// Prompt is the optional dialog shown to user,
	// when the access strategy type requires a reason.
	Prompt string `json:"prompt"`
}

// AccessCapabilities defines allowable access request rules defined in a user's roles.
type AccessCapabilities struct {
	// RequestableRoles is a list of roles that the user can select when requesting access.
	RequestableRoles []string `json:"requestableRoles"`
	// SuggestedReviewers is a list of reviewers that the user can select when creating a request.
	SuggestedReviewers []string `json:"suggestedReviewers"`
}

type authType string

const (
	authLocal authType = "local"
	authSSO   authType = "sso"
)

// UserContext describes user settings and access to various resources.
type UserContext struct {
	// AuthType is auth method of this user.
	AuthType authType `json:"authType"`
	// Name is this user name.
	Name string `json:"userName"`
	// ACL contains user access control list.
	ACL services.UserACL `json:"userAcl"`
	// Cluster contains cluster detail for this user's context.
	Cluster *Cluster `json:"cluster"`
	// AccessStrategy describes how a user should access teleport resources.
	AccessStrategy accessStrategy `json:"accessStrategy"`
	// AccessCapabilities defines allowable access request rules defined in a user's roles.
	AccessCapabilities AccessCapabilities `json:"accessCapabilities"`
	// ConsumedAccessRequestID is the request ID of the access request from which the assumed role was
	// obtained
	ConsumedAccessRequestID string `json:"accessRequestId,omitempty"`
}

func getAccessStrategy(roleset services.RoleSet) accessStrategy {
	strategy := types.RequestStrategyOptional
	prompt := ""

	for _, role := range roleset {
		options := role.GetOptions()

		if options.RequestAccess == types.RequestStrategyReason {
			strategy = types.RequestStrategyReason
			prompt = options.RequestPrompt
			break
		}

		if options.RequestAccess == types.RequestStrategyAlways {
			strategy = types.RequestStrategyAlways
		}
	}

	return accessStrategy{
		Type:   strategy,
		Prompt: prompt,
	}
}

// NewUserContext returns user context
func NewUserContext(user types.User, userRoles services.RoleSet, features proto.Features, desktopRecordingEnabled bool) (*UserContext, error) {
	acl := services.NewUserACL(user, userRoles, features, desktopRecordingEnabled)
	accessStrategy := getAccessStrategy(userRoles)

	// local user
	authType := authLocal

	// check for any SSO identities
	isSSO := len(user.GetOIDCIdentities()) > 0 ||
		len(user.GetGithubIdentities()) > 0 ||
		len(user.GetSAMLIdentities()) > 0

	if isSSO {
		// SSO user
		authType = authSSO
	}

	return &UserContext{
		Name:           user.GetName(),
		ACL:            acl,
		AuthType:       authType,
		AccessStrategy: accessStrategy,
	}, nil
}
