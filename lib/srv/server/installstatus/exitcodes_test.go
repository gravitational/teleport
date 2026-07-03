/*
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

package installstatus

import (
	"go/constant"
	"go/types"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

func TestExitCodeString(t *testing.T) {
	for _, tt := range []struct {
		name string
		code ExitCode
		want string
	}{
		{
			name: "windows installer download failure",
			code: WindowsInstallerDownloadFailure,
			want: "Failed to download the Teleport authentication package installer. " +
				"Ensure this host can reach https://cdn.teleport.dev and try again.",
		},
		{
			name: "windows installer execution failure",
			code: WindowsInstallerExecutionFailure,
			want: "The Teleport authentication package installer returned an error. " +
				"Check the standard output and standard error for details.",
		},
		{
			name: "unrecognized code falls back to the generic message",
			code: ExitCode(999),
			want: "Installation failed with exit code 999. " +
				"Please check stdout and stderr and try again.",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.code.String())
		})
	}
}

// TestAllExitCodesHaveDedicatedMessage ensures every registered ExitCode has a
// dedicated String() case
func TestAllExitCodesHaveDedicatedMessage(t *testing.T) {
	// genericMessage returns the fallback String() the default switch branch
	// produces for the given code. We generate it using the unregistered code
	// below, then replace the unregistered code with the code being tested.
	const unregistered ExitCode = -999999
	genericMessage := func(c ExitCode) string {
		return strings.Replace(unregistered.String(), strconv.Itoa(int(unregistered)), strconv.Itoa(int(c)), 1)
	}

	codes := registeredExitCodes(t)
	t.Log(codes)
	require.NotEmpty(t, codes, "no ExitCode constants were discovered, the source parser is likely broken")

	for name, code := range codes {
		require.NotEqualf(t, genericMessage(code), code.String(),
			"ExitCode %s (%d) has no dedicated message; add a case to ExitCode.String()", name, int(code))
	}
}

// registeredExitCodes gets all the registered ExitCodes in exitcodes.go
func registeredExitCodes(t *testing.T) map[string]ExitCode {
	t.Helper()

	pkgs, err := packages.Load(&packages.Config{
		// NeedTypes returns exported ExitCodes types. It doesn't catch unexported constants.
		Mode: packages.NeedTypes,
	}, ".")
	require.NoError(t, err)
	require.Len(t, pkgs, 1)

	pkg := pkgs[0]
	require.Empty(t, pkg.Errors, "failed to load and type-check package")
	require.NotNil(t, pkg.Types)

	codes := make(map[string]ExitCode)
	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		c, ok := scope.Lookup(name).(*types.Const)
		if !ok {
			continue
		}
		named, ok := c.Type().(*types.Named)
		if !ok || named.Obj().Name() != "ExitCode" {
			continue
		}
		v, ok := constant.Int64Val(c.Val())
		require.Truef(t, ok, "ExitCode const %s is not an integer", name)
		codes[name] = ExitCode(v)
	}
	return codes
}
