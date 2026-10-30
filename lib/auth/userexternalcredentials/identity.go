/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package userexternalcredentials

import (
	"github.com/gravitational/trace"

	usercredpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

// KeyIDFromIdentity returns the ID of the user external credentials of the
// session of the identity. Sessions of a delegated bot are stored under the
// delegator.
func KeyIDFromIdentity(identity *tlsca.Identity) (*usercredpb.EncryptionKeyID, error) {
	if identity.EncryptionKeyID == "" {
		return nil, trace.BadParameter("certificate does not have a session encryption key, please log in again")
	}
	if chain := identity.Delegation.GetChain(); len(chain) > 0 {
		delegator, err := services.DelegatorUsername(chain[0])
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return usercredpb.EncryptionKeyID_builder{
			User:     delegator,
			Delegate: identity.Username,
			KeyId:    identity.EncryptionKeyID,
		}.Build(), nil
	}
	return usercredpb.EncryptionKeyID_builder{
		User:  identity.Username,
		KeyId: identity.EncryptionKeyID,
	}.Build(), nil
}
