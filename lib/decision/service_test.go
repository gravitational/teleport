// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package decision

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/testutils"
)

func TestLockTargetConversion(t *testing.T) {
	lt := types.LockTarget{
		User:           "user",
		Role:           "role",
		Login:          "login",
		MFADevice:      "mfadevice",
		WindowsDesktop: "windows",
		AccessRequest:  "request",
		Device:         "device",
		ServerID:       "server",
	}

	ignores := []string{
		"LockTarget.Node", // deprecated
		"LockTarget.XXX_NoUnkeyedLiteral",
		"LockTarget.XXX_unrecognized",
		"LockTarget.XXX_sizecache",
	}

	require.True(t, testutils.ExhaustiveNonEmpty(lt, ignores...), "empty=%+v", testutils.FindAllEmpty(lt, ignores...))

	proto := lockTargetToProto(lt)

	lt2 := lockTargetFromProto(proto)

	require.Empty(t, cmp.Diff(lt, lt2))
}
