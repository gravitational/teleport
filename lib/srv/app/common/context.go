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

package common

import (
	"context"
	"net/http"
)

// WithAWSAssumedRole adds AWS assumed role to the context of the provided request.
func WithAWSAssumedRole(r *http.Request, assumedRole string) *http.Request {
	if assumedRole == "" {
		return r
	}
	return r.WithContext(context.WithValue(
		r.Context(),
		contextKeyAWSAssumedRole,
		assumedRole,
	))
}

// GetAWSAssumedRole gets AWS assumed role from a request.
func GetAWSAssumedRole(r *http.Request) string {
	assumedRoleValue := r.Context().Value(contextKeyAWSAssumedRole)
	assumedRole, ok := assumedRoleValue.(string)
	if ok {
		return assumedRole
	}
	return ""
}

type contextKey string

// contextKeyAWSAssumedRole is the context key for AWS assumed role.
const contextKeyAWSAssumedRole contextKey = "aws-assumed-role"
