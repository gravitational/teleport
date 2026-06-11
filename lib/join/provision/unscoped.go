// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package provision

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes"
)

// UnscopedToken adapts a legacy [types.ProvisionToken] to the [Token]
// interface. [types.ProvisionToken] lives in the api module, which cannot
// depend on the scopes package, so it cannot implement [Token.GetBot] itself.
type UnscopedToken struct {
	types.ProvisionToken
}

// GetBot returns a reference to the bot that this token can join. Legacy
// provision tokens can only refer to unscoped bots, so the Scope component is
// always empty. The zero value indicates that this is not a bot token.
func (t UnscopedToken) GetBot() scopes.QualifiedName {
	return scopes.QualifiedName{Name: t.GetBotName()}
}

// AsProvisionTokenV2 attempts to return the [*types.ProvisionTokenV2] backing
// a [Token]. Returns false if the token is not an [UnscopedToken] wrapping a
// [*types.ProvisionTokenV2].
func AsProvisionTokenV2(token Token) (*types.ProvisionTokenV2, bool) {
	unscoped, ok := token.(UnscopedToken)
	if !ok {
		return nil, false
	}
	v2, ok := unscoped.ProvisionToken.(*types.ProvisionTokenV2)
	return v2, ok
}
