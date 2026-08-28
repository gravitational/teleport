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

package circleci

import (
	"github.com/gravitational/trace"
)

type getEnvFunc func(key string) string

func GetIDToken(getEnv getEnvFunc) (string, error) {
	token := getEnv("CIRCLE_OIDC_TOKEN")
	if token == "" {
		return "", trace.BadParameter("'CIRCLE_OIDC_TOKEN' must be present to use 'circleci' join method")
	}
	return token, nil
}
