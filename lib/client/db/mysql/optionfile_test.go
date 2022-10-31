/*
Copyright 2021 Gravitational, Inc.

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
		CACertPath: "ca.pem",
		CertPath:   "cert.pem",
		KeyPath:    "key.pem",
	}

	err = optionFile.Upsert(profile)
	require.NoError(t, err)

	env, err := optionFile.Env(profile.Name)
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"MYSQL_GROUP_SUFFIX": "_test",
	}, env)

	err = optionFile.Delete(profile.Name)
	require.NoError(t, err)

	_, err = optionFile.Env(profile.Name)
	require.Error(t, err)
	require.IsType(t, trace.NotFound(""), err)
}
