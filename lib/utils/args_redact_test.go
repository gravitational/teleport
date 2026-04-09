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

package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactFlagArgs(t *testing.T) {
	t.Parallel()

	mask := func(v string) string {
		return strings.Repeat("*", len(v))
	}

	original := []string{
		"node",
		"configure",
		"--token=secret-token",
		"--password",
		"super-secret",
		"--proxy=example.teleport.sh:443",
		"--unrelated",
	}

	got := RedactFlagArgs(original, map[string]ArgValueRedactor{
		"--token":    mask,
		"--password": mask,
	})

	require.Equal(t, []string{
		"node",
		"configure",
		"--token=************",
		"--password",
		"************",
		"--proxy=example.teleport.sh:443",
		"--unrelated",
	}, got)
	require.Equal(t, []string{
		"node",
		"configure",
		"--token=secret-token",
		"--password",
		"super-secret",
		"--proxy=example.teleport.sh:443",
		"--unrelated",
	}, original)
}

func TestRedactFlagArgsMalformed(t *testing.T) {
	t.Parallel()

	mask := func(v string) string {
		return strings.Repeat("*", len(v))
	}

	t.Run("empty value after equals", func(t *testing.T) {
		t.Parallel()

		got := RedactFlagArgs(
			[]string{"--token=", "--password=1234"},
			map[string]ArgValueRedactor{"--token": mask, "--password": mask},
		)

		require.Equal(t, []string{"--token=", "--password=****"}, got)
	})

	t.Run("equals sign in value", func(t *testing.T) {
		t.Parallel()

		got := RedactFlagArgs(
			[]string{"--token=abc=def"},
			map[string]ArgValueRedactor{"--token": mask},
		)

		require.Equal(t, []string{"--token=*******"}, got)
	})

	t.Run("space-joined flags in single arg", func(t *testing.T) {
		t.Parallel()

		got := RedactFlagArgs(
			[]string{"--token= --password=1234"},
			map[string]ArgValueRedactor{"--token": mask, "--password": mask},
		)

		// The entire string after the first "=" is treated as the
		// token value, so "--password=1234" is masked as part of
		// the token. The password redactor never fires independently.
		require.Equal(t, []string{"--token=****************"}, got)
	})

	t.Run("empty args", func(t *testing.T) {
		t.Parallel()

		got := RedactFlagArgs(nil, map[string]ArgValueRedactor{"--token": mask})

		require.Empty(t, got)
	})
}

func TestRedactFlagArgsMissingFlagValue(t *testing.T) {
	t.Parallel()

	original := []string{"node", "configure", "--token"}

	got := RedactFlagArgs(original, map[string]ArgValueRedactor{
		"--token": func(v string) string { return "redacted:" + v },
	})

	require.Equal(t, original, got)
}
