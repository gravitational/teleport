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

package tools

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFilterEnv verifies excluding environment variables by the list of the keys.
func TestFilterEnv(t *testing.T) {
	env := "TEST_ENV_WITHOUT_FILTER"
	env1 := "TEST_ENV_WITHOUT_FILTER1"
	env2 := "TEST_ENV_WITHOUT_FILTER2"
	env3 := "TEST_ENV_WITHOUT_FILTER3"

	source := []string{
		env3,
		env,
		fmt.Sprint(env1, "=", "test"),
		fmt.Sprint(teleportToolsVersionEnv, "=", teleportToolsVersionEnvDisabled),
		fmt.Sprint(env2, "=", "test"),
		env3,
		env,
		env3,
	}

	assert.Equal(t, []string{
		env,
		fmt.Sprint(env1, "=", "test"),
		fmt.Sprint(env2, "=", "test"),
		env,
	}, filterEnvs(source, []string{teleportToolsVersionEnv, env3}))
}
