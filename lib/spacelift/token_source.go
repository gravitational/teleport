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

package spacelift

import "github.com/gravitational/trace"

type envGetter func(key string) string

// IDTokenSource allows a SpaceLift ID token to be fetched whilst within a
// SpaceLift execution.
type IDTokenSource struct {
	getEnv envGetter
}

func (its *IDTokenSource) GetIDToken() (string, error) {
	tok := its.getEnv("SPACELIFT_OIDC_TOKEN")
	if tok == "" {
		return "", trace.BadParameter(
			"SPACELIFT_OIDC_TOKEN environment variable missing",
		)
	}

	return tok, nil
}

func NewIDTokenSource(getEnv envGetter) *IDTokenSource {
	return &IDTokenSource{
		getEnv,
	}
}
