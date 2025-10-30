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

package common

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/lib/services"
)

// GetUserTraits returns the specified user's traits.
func GetUserTraits(ctx context.Context, client services.UserOrLoginStateGetter, username string) (trait.Traits, error) {
	// Get user traits from the user_login_state.
	// This includes traits derived from access lists, login rules, and other
	// mechanisms.
	userLoginState, err := client.GetUserLoginState(ctx, username)
	if err == nil {
		return userLoginState.GetTraits(), nil
	}

	// If unable to get traits from user login state due to missing permissions,
	// get traits from original user resource.
	const withSecretsFalse = false
	user, err := client.GetUser(ctx, username, withSecretsFalse)
	if err == nil {
		return user.GetTraits(), nil
	}

	return nil, trace.Wrap(err)
}
