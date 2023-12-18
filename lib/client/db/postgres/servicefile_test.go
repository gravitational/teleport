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

package postgres

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/client/db/profile"
)

func TestServiceFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), pgServiceFile)

	serviceFile, err := LoadFromPath(path)
	require.NoError(t, err)

	profile := profile.ConnectProfile{
		Name:       "test",
		Host:       "localhost",
		Port:       5342,
		User:       "postgres",
		Database:   "postgres",
		Insecure:   false,
		CACertPath: "ca.pem",
		CertPath:   "cert.pem",
		KeyPath:    "key.pem",
	}

	err = serviceFile.Upsert(profile)
	require.NoError(t, err)

	env, err := serviceFile.Env(profile.Name)
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"PGHOST":        profile.Host,
		"PGPORT":        strconv.Itoa(profile.Port),
		"PGUSER":        profile.User,
		"PGDATABASE":    profile.Database,
		"PGSSLMODE":     SSLModeVerifyFull,
		"PGSSLROOTCERT": profile.CACertPath,
		"PGSSLCERT":     profile.CertPath,
		"PGSSLKEY":      profile.KeyPath,
		"PGGSSENCMODE":  "disable",
	}, env)

	err = serviceFile.Delete(profile.Name)
	require.NoError(t, err)

	_, err = serviceFile.Env(profile.Name)
	require.Error(t, err)
	require.IsType(t, trace.NotFound(""), err)
}
