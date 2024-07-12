// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package ui

import (
	"time"

	yaml "github.com/ghodss/yaml"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// JoinToken is a UI-friendly representation of a JoinToken
type JoinToken struct {
	// ID is the name of the token
	ID string `json:"id"`
	// SafeName returns the name of the token, sanitized appropriately for
	// join methods where the name is secret.
	SafeName string `json:"safeName"`
	// Expiry is the time that the token resource expires. Tokens that do not expire
	// should expect a zero value time to be returned.
	Expiry time.Time `json:"expiry"`
	// Roles are the roles granted to the token
	Roles types.SystemRoles `json:"roles"`
	// IsStatic is true if the token is statically configured
	IsStatic bool `json:"isStatic"`
	// Method is the join method that the token supports
	Method types.JoinMethod `json:"method"`
	// AllowRules is a list of allow rules
	AllowRules []string `json:"allowRules,omitempty"`
	// Content is resource yaml content.
	Content string `json:"content"`
}

func MakeJoinToken(token types.ProvisionToken) (*JoinToken, error) {
	content, err := yaml.Marshal(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &JoinToken{
		ID:       token.GetName(),
		SafeName: token.GetSafeName(),
		Expiry:   token.Expiry(),
		Roles:    token.GetRoles(),
		IsStatic: token.IsStatic(),
		Method:   token.GetJoinMethod(),
		Content:  string(content[:]),
	}, nil
}

func MakeJoinTokens(tokens []types.ProvisionToken) (joinTokens []JoinToken, err error) {
	for _, t := range tokens {
		uiToken, err := MakeJoinToken(t)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		joinTokens = append(joinTokens, *uiToken)
	}
	return joinTokens, nil
}
