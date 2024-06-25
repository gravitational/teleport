// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package utils

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/user"
	"testing"

	"github.com/stretchr/testify/require"
)

// RequireRoot skips the current test if it is not running as root.
func RequireRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("This test will be skipped because tests are not being run as root.")
	}
}

func generateUsername(t *testing.T) string {
	suffix := make([]byte, 8)
	_, err := rand.Read(suffix)
	require.NoError(t, err)
	return fmt.Sprintf("teleport-%x", suffix)
}

// GenerateLocalUsername generates the username for a local user that does not
// already exists (but it does not create the user).
func GenerateLocalUsername(t *testing.T) string {
	const maxAttempts = 10
	for i := 0; i < maxAttempts; i++ {
		login := generateUsername(t)
		_, err := user.Lookup(login)
		if errors.Is(err, user.UnknownUserError(login)) {
			return login
		}
		require.NoError(t, err)
	}
	t.Fatalf("Unable to generate unused username after %v attempts", maxAttempts)
	return ""
}
