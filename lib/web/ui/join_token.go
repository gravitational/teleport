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
	// BotName is the name of the bot this token grants access to, if any
	BotName string `json:"bot_name"`
	// Expiry is the time that the token resource expires. Tokens that do not expire
	// should expect a zero value time to be returned.
	Expiry time.Time `json:"expiry"`
	// Roles are the roles granted to the token
	Roles types.SystemRoles `json:"roles"`
	// IsStatic is true if the token is statically configured
	IsStatic bool `json:"isStatic"`
	// Method is the join method that the token supports
	Method types.JoinMethod `json:"method"`
	// Allow is a list of allow rules
	Allow []*types.TokenRule `json:"allow,omitempty"`
	// GCP allows the configuration of options specific to the "gcp" join method.
	GCP *types.ProvisionTokenSpecV2GCP `json:"gcp,omitempty"`
	// Content is resource yaml content.
	Content string `json:"content"`
}

func MakeJoinToken(token types.ProvisionToken) (*JoinToken, error) {
	content, err := yaml.Marshal(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	uiToken := &JoinToken{
		ID:       token.GetName(),
		SafeName: token.GetSafeName(),
		BotName:  token.GetBotName(),
		Expiry:   token.Expiry(),
		Roles:    token.GetRoles(),
		IsStatic: token.IsStatic(),
		Method:   token.GetJoinMethod(),
		Allow:    token.GetAllowRules(),
		Content:  string(content[:]),
	}

	if uiToken.Method == types.JoinMethodGCP {
		uiToken.GCP = token.GetGCPRules()
	}
	return uiToken, nil
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
