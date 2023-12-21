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

package main

import (
	"os"
	"strings"

	"github.com/gravitational/trace"
)

const (
	namespacePath   = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	namespaceEnvVar = "POD_NAMESPACE"
)

func GetKubernetesNamespace() (string, error) {
	namespace := os.Getenv(namespaceEnvVar)
	if namespace != "" {
		return namespace, nil
	}

	bs, err := os.ReadFile(namespacePath)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return strings.TrimSpace(string(bs)), nil
}
