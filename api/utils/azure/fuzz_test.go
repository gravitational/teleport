/*
Copyright 2022 Gravitational, Inc.

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

package azure

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzParseDatabaseEndpoint(f *testing.F) {
	f.Add("")
	f.Add("foo")
	f.Add(":1234")
	f.Add("foo:1234")
	f.Add("name.mysql.database.azure.com:1234")

	f.Fuzz(func(t *testing.T, endpoint string) {
		require.NotPanics(t, func() {
			ParseDatabaseEndpoint(endpoint)
		})
	})
}

func FuzzParseCacheForRedisEndpoint(f *testing.F) {
	f.Add("")
	f.Add("foo")
	f.Add("name.redis.cache.windows.net")
	f.Add("name.redis.cache.windows.net:1234")
	f.Add("name.region.redisenterprise.cache.azure.net")

	f.Fuzz(func(t *testing.T, endpoint string) {
		require.NotPanics(t, func() {
			ParseCacheForRedisEndpoint(endpoint)
		})
	})
}

func FuzzNormalizeLocation(f *testing.F) {
	f.Add("")
	f.Add("foo")
	f.Add("northcentralusstage")
	f.Add("North Central US (Stage)")
	f.Add("(US) North Central US (Stage)")

	f.Fuzz(func(t *testing.T, location string) {
		require.NotPanics(t, func() {
			NormalizeLocation(location)
		})
	})
}

func FuzzParseMSSQLEndpoint(f *testing.F) {
	f.Add("")
	f.Add("foo")
	f.Add(":1234")
	f.Add("foo:1234")
	f.Add("name.database.windows.net:1234")
	f.Add(".database.windows.net:1234")

	f.Fuzz(func(t *testing.T, endpoint string) {
		require.NotPanics(t, func() {
			ParseMSSQLEndpoint(endpoint)
		})
	})
}
