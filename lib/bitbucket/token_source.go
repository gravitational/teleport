/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package bitbucket

import (
	"github.com/gravitational/trace"
)

// IDTokenSource allows a Bitbucket OIDC token to be fetched whilst within a
// Pipelines execution.
type IDTokenSource struct {
	// getEnv is a function that returns a string from the environment, usually
	// os.Getenv except in tests.
	getEnv func(key string) string
}

// GetIDToken attempts to fetch a Bitbucket OIDC token from the environment.
func (its *IDTokenSource) GetIDToken() (string, error) {
	tok := its.getEnv("BITBUCKET_STEP_OIDC_TOKEN")
	if tok == "" {
		return "", trace.BadParameter(
			"BITBUCKET_STEP_OIDC_TOKEN environment variable missing",
		)
	}

	return tok, nil
}

// NewIDTokenSource builds a helper that can extract a Bitbucket OIDC token from
// the environment, using `getEnv`.
func NewIDTokenSource(getEnv func(key string) string) *IDTokenSource {
	return &IDTokenSource{
		getEnv,
	}
}
