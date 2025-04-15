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

package services

import (
	"strings"

	"github.com/gravitational/trace"
)

// NewExpiredReusableMFAResponseError returns an access-denied error hinting MFA
// validation failure caused by expired reusable MFA response.
func NewExpiredReusableMFAResponseError(errMsg string) error {
	return trace.AccessDenied("Reusable MFA response validation failed and possibly expired: %s", errMsg)
}

// IsExpiredReusableMFAResponseError checks if received error is hinting expired
// reusable MFA response.
func IsExpiredReusableMFAResponseError(err error) bool {
	return trace.IsAccessDenied(err) &&
		strings.Contains(err.Error(), NewExpiredReusableMFAResponseError("").Error())
}
