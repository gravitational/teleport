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

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToChangelog(t *testing.T) {
	prsText, err := os.ReadFile(filepath.Join("testdata", "listed-prs.json"))
	require.NoError(t, err)
	expectedCL, err := os.ReadFile(filepath.Join("testdata", "expected-cl.md"))
	require.NoError(t, err)

	gen := &changelogGenerator{
		isEnt: false,
	}
	got, err := gen.toChangelog(string(prsText))
	assert.NoError(t, err)
	assert.Equal(t, string(expectedCL), got)
}

func TestToChangelogEnterprise(t *testing.T) {
	prsText, err := os.ReadFile(filepath.Join("testdata", "ent-listed-prs.json"))
	require.NoError(t, err)
	expectedCL, err := os.ReadFile(filepath.Join("testdata", "ent-expected-cl.md"))
	require.NoError(t, err)
	gen := &changelogGenerator{
		isEnt: true,
	}
	got, err := gen.toChangelog(string(prsText))
	require.NoError(t, err)
	assert.Equal(t, string(expectedCL), got)
}
