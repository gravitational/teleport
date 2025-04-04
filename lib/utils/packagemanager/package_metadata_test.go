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

package packagemanager

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRepositoryEndpoint(t *testing.T) {
	for _, tt := range []struct {
		name                    string
		inputProd               bool
		inputPackageManager     string
		checkErr                require.ErrorAssertionFunc
		expectedRepoEndpoint    string
		expectedRepoKeyEndpoint string
	}{
		{
			name:                    "development apt",
			inputProd:               false,
			inputPackageManager:     "apt",
			checkErr:                require.NoError,
			expectedRepoEndpoint:    "https://apt.releases.development.teleport.dev/",
			expectedRepoKeyEndpoint: "https://apt.releases.development.teleport.dev/gpg",
		},
		{
			name:                    "production apt",
			inputProd:               true,
			inputPackageManager:     "apt",
			checkErr:                require.NoError,
			expectedRepoEndpoint:    "https://apt.releases.teleport.dev/",
			expectedRepoKeyEndpoint: "https://apt.releases.teleport.dev/gpg",
		},
		{
			name:                    "development yum",
			inputProd:               false,
			inputPackageManager:     "yum",
			checkErr:                require.NoError,
			expectedRepoEndpoint:    "https://yum.releases.development.teleport.dev/",
			expectedRepoKeyEndpoint: "https://yum.releases.development.teleport.dev/gpg",
		},
		{
			name:                    "production yum",
			inputProd:               true,
			inputPackageManager:     "yum",
			checkErr:                require.NoError,
			expectedRepoEndpoint:    "https://yum.releases.teleport.dev/",
			expectedRepoKeyEndpoint: "https://yum.releases.teleport.dev/gpg",
		},
		{
			name:                    "development zypper",
			inputProd:               false,
			inputPackageManager:     "zypper",
			checkErr:                require.NoError,
			expectedRepoEndpoint:    "https://zypper.releases.development.teleport.dev/",
			expectedRepoKeyEndpoint: "https://zypper.releases.development.teleport.dev/gpg",
		},
		{
			name:                    "production zypper",
			inputProd:               true,
			inputPackageManager:     "zypper",
			checkErr:                require.NoError,
			expectedRepoEndpoint:    "https://zypper.releases.teleport.dev/",
			expectedRepoKeyEndpoint: "https://zypper.releases.teleport.dev/gpg",
		},
		{
			name:                    "unknown tool",
			inputProd:               true,
			inputPackageManager:     "pacman",
			checkErr:                require.Error,
			expectedRepoEndpoint:    "",
			expectedRepoKeyEndpoint: "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gotEndpoint, gotKeyEndpoint, err := repositoryEndpoint(tt.inputProd, tt.inputPackageManager)
			if tt.checkErr != nil {
				tt.checkErr(t, err)
			}

			require.Equal(t, tt.expectedRepoEndpoint, gotEndpoint)
			require.Equal(t, tt.expectedRepoKeyEndpoint, gotKeyEndpoint)
		})
	}
}
