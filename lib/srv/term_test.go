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

package srv

import (
	"os"
	"os/user"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestGetOwner(t *testing.T) {
	t.Parallel()

	tests := []struct {
		inUserLookup  LookupUser
		inGroupLookup LookupGroup
		outUID        int
		outGID        int
		outMode       os.FileMode
	}{
		// Group "tty" exists.
		{
			inUserLookup: func(s string) (*user.User, error) {
				return &user.User{
					Uid: "1000",
					Gid: "1000",
				}, nil
			},
			inGroupLookup: func(s string) (*user.Group, error) {
				return &user.Group{
					Gid: "5",
				}, nil
			},
			outUID:  1000,
			outGID:  5,
			outMode: 0600,
		},
		// Group "tty" does not exist.
		{
			inUserLookup: func(s string) (*user.User, error) {
				return &user.User{
					Uid: "1000",
					Gid: "1000",
				}, nil
			},
			inGroupLookup: func(s string) (*user.Group, error) {
				return &user.Group{}, trace.BadParameter("")
			},
			outUID:  1000,
			outGID:  1000,
			outMode: 0620,
		},
	}

	for _, tt := range tests {
		uid, gid, mode, err := getOwner("", tt.inUserLookup, tt.inGroupLookup)
		require.NoError(t, err)

		require.Equal(t, tt.outUID, uid)
		require.Equal(t, tt.outGID, gid)
		require.Equal(t, tt.outMode, mode)
	}
}
