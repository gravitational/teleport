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

package token

import (
	"strings"

	"github.com/gravitational/trace"
)

const (
	kubernetesDefaultTokenPath      = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	EnvVarCustomKubernetesTokenPath = "KUBERNETES_TOKEN_PATH"
)

type getEnvFunc func(key string) string
type readFileFunc func(name string) ([]byte, error)

func GetIDToken(getEnv getEnvFunc, readFile readFileFunc) (string, error) {
	// We check if we should use a custom location instead of the default one. This env var is not standard.
	// This is useful when the operator wants to use a custom projected token, or another service account.
	path := kubernetesDefaultTokenPath
	if customPath := getEnv(EnvVarCustomKubernetesTokenPath); customPath != "" {
		path = customPath
	}

	token, err := readFile(path)
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}

	// Usually kubernetes tokens don't start or end with newlines, but better safe than sorry
	return strings.TrimSpace(string(token)), nil
}
