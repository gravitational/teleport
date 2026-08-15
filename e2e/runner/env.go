/**
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

package main

import (
	"os"
	"strings"
)

// envStripKeys is a set of process environment variables cleared before each Teleport
// instance is started as a safety mechanism to prevent ambient environment variables
// with known Teleport behavioral changes from accidentally leaking into the default
// config or other declared configs during restarts. This list can be expanded as needed.
var envStripKeys = []string{"TELEPORT_CLOUD_HOSTPORT"}

// instanceEnv builds an isolated slice of environment variables for the Teleport instance process.
// It uses the existing environment variables, removes envStripKeys and any key present in
// overrides, then applies overrides.
func instanceEnv(overrides map[string]string) []string {
	strip := make(map[string]struct{}, len(envStripKeys)+len(overrides))
	for _, k := range envStripKeys {
		strip[k] = struct{}{}
	}
	for k := range overrides {
		strip[k] = struct{}{}
	}

	env := os.Environ()
	n := 0
	for _, kv := range env {
		key, _, _ := strings.Cut(kv, "=")
		if _, ok := strip[key]; !ok {
			env[n] = kv
			n++
		}
	}
	env = env[:n]

	for k, v := range overrides {
		env = append(env, k+"="+v)
	}

	return env
}
