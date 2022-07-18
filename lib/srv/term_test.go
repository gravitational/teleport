/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
