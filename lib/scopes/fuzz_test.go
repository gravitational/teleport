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
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 */

package scopes

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// FuzzParseQualifiedName verifies that ParseQualifiedName never panics and that
// any successfully parsed value round-trips through String() unchanged.
func FuzzParseQualifiedName(f *testing.F) {
	// valid SQNs
	f.Add("/staging/west::myrole")
	f.Add("/staging::myrole")
	f.Add("/::myrole")
	f.Add("/staging::my-role")
	f.Add("/staging::318ea8be-129c-41f4-ad95-fd830e14e3e7")
	// multiple separators — splits on first
	f.Add("/staging::my::role")
	// invalid but parseable
	f.Add("staging::myrole")
	// parse errors
	f.Add("")
	f.Add("::")
	f.Add("::myrole")
	f.Add("/staging::")
	f.Add("no-separator")
	f.Add("/staging/west::my role")

	f.Fuzz(func(t *testing.T, sqn string) {
		var qn QualifiedName
		var err error
		require.NotPanics(t, func() { qn, err = ParseQualifiedName(sqn) })
		if err != nil {
			return
		}

		// round-trip: String() must re-parse to the same value
		require.NotPanics(t, func() {
			qn2, err2 := ParseQualifiedName(qn.String())
			require.NoError(t, err2, "String() of a parsed QualifiedName must re-parse without error")
			require.Equal(t, qn, qn2, "String() of a parsed QualifiedName must round-trip to an identical value")
		})
	})
}

// FuzzValidateQualifiedName verifies that neither validate function panics, and that
// strong validation passing implies weak validation passing.
func FuzzValidateQualifiedName(f *testing.F) {
	// valid under both
	f.Add("/staging/west::myrole")
	f.Add("/staging::myrole")
	f.Add("/::myrole")
	f.Add("/staging::my-role")
	f.Add("/staging::318ea8be-129c-41f4-ad95-fd830e14e3e7")
	// valid under weak only
	f.Add("staging::myrole")
	f.Add("/Staging/west::myrole")
	f.Add("/staging::MyRole")
	f.Add("/a/west::myrole")
	f.Add("/staging::x")
	// invalid under both
	f.Add("")
	f.Add("::")
	f.Add("::myrole")
	f.Add("/staging::")
	f.Add("no-separator")
	f.Add("/staging::my role")
	f.Add("/stag@ing::myrole")
	f.Add("/staging::my::role")

	f.Fuzz(func(t *testing.T, sqn string) {
		var strongErr, weakErr error

		require.NotPanics(t, func() { strongErr = StrongValidateQualifiedName(sqn) })
		require.NotPanics(t, func() { weakErr = WeakValidateQualifiedName(sqn) })

		// strong passing must imply weak passing
		if strongErr == nil {
			require.NoError(t, weakErr, "StrongValidateQualifiedName passed but WeakValidateQualifiedName failed for %q", sqn)
		}
	})
}
