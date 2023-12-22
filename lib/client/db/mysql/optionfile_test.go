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

package mysql

import (
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/client/db/profile"
)

func TestOptionFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), mysqlOptionFile)

	optionFile, err := LoadFromPath(path)
	require.NoError(t, err)

	profile := profile.ConnectProfile{
		Name:       "test",
		Host:       "localhost",
		Port:       3036,
		User:       "root",
		Database:   "mysql",
		Insecure:   false,
		CACertPath: "c:\\users\\user\\.tsh\\foo\\ca.pem",
		CertPath:   "c:\\users\\user\\.tsh\\foo\\cert.pem",
		KeyPath:    "c:\\users\\user\\.tsh\\foo\\key.pem",
	}

	err = optionFile.Upsert(profile)
	require.NoError(t, err)

	env, err := optionFile.Env(profile.Name)
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"MYSQL_GROUP_SUFFIX": "_test",
	}, env)

	// load, compare
	optionFileRead, err := LoadFromPath(path)
	require.NoError(t, err)
	require.Equal(t, optionFile, optionFileRead)

	clientTest, err := optionFileRead.iniFile.GetSection("client_test")
	require.NoError(t, err)

	require.Equal(t,
		map[string]string{
			"host":     "localhost",
			"port":     "3036",
			"user":     "root",
			"database": "mysql",
			"ssl-mode": "VERIFY_IDENTITY",
			"ssl-ca":   `c:\\users\\user\\.tsh\\foo\\ca.pem`,
			"ssl-cert": `c:\\users\\user\\.tsh\\foo\\cert.pem`,
			"ssl-key":  `c:\\users\\user\\.tsh\\foo\\key.pem`,
		},
		clientTest.KeysHash())

	// delete
	err = optionFile.Delete(profile.Name)
	require.NoError(t, err)

	_, err = optionFile.Env(profile.Name)
	require.Error(t, err)
	require.IsType(t, trace.NotFound(""), err)
}
