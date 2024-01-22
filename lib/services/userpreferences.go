/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package services

import (
	"context"

	userpreferencesv1 "github.com/gravitational/teleport/api/gen/proto/go/userpreferences/v1"
)

// UserPreferences is the interface for managing user preferences.
type UserPreferences interface {
	// GetUserPreferences returns the user preferences for a given user.
	GetUserPreferences(ctx context.Context, username string) (*userpreferencesv1.UserPreferences, error)
	// UpsertUserPreferences creates or updates user preferences for a given username.
	UpsertUserPreferences(ctx context.Context, username string, prefs *userpreferencesv1.UserPreferences) error
}
