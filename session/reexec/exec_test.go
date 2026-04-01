// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package reexec

import (
	"flag"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if IsReexec() {
		RunAndExit(os.Args[1])
		return
	}

	if !flag.Parsed() {
		flag.Parse()
	}
	if testing.Verbose() {
		slog.SetDefault(slog.New(slog.NewJSONHandler(
			os.Stderr,
			&slog.HandlerOptions{
				Level: slog.LevelDebug,
			},
		)))
	} else {
		slog.SetDefault(slog.New(slog.DiscardHandler))
	}

	os.Exit(m.Run())
}

func TestLoginDefsParser(t *testing.T) {
	t.Parallel()

	expectedEnvSuPath := "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/bar"
	expectedSuPath := "PATH=/usr/local/bin:/usr/bin:/bin:/foo"

	require.Equal(t, expectedEnvSuPath, getDefaultEnvPathWithLoginDefs("0", "../../fixtures/login.defs"))
	require.Equal(t, expectedSuPath, getDefaultEnvPathWithLoginDefs("1000", "../../fixtures/login.defs"))
	require.Equal(t, defaultEnvPath, getDefaultEnvPathWithLoginDefs("1000", "bad/file"))
}
