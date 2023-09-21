// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package atlas

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsAtlasEndpoint(t *testing.T) {
	for _, tc := range []struct {
		desc     string
		endpoint string
		result   bool
	}{
		// Valid
		{"public endpoint host only", "test.xxxxxxx.mongodb.net", true},
		{"public endpoint", "mongodb://test.xxxxxxx.mongodb.net", true},
		{"public endpoint with srv", "mongodb+srv://test.xxxxxxx.mongodb.net", true},
		{"private link", "mongodb://pl-0-us-east-1-xxxxx.mongodb.net:1024,pl-0-us-east-1-xxxxx.mongodb.net:1025,pl-0-us-east-1-xxxxx.mongodb.net:1026/?ssl=true&authSource=admin&replicaSet=Cluster0-shard-0-shard-0", true},
		{"private link with srv", "mongodb+srv://cluster0-pl-0-xxxxx.mongodb.net", true},
		{"azure private link", "mongodb://cluster0-pl-0.xxxxx.azure.mongodb.net", true},
		{"azure private link with srv", "mongodb://pl-0-eastus2.xxxxx.azure.mongodb.net:1024,pl-0-eastus2.xxxxx.azure.mongodb.net:1025,pl-0-eastus2.xxxxx.azure.mongodb.net:1026/?ssl=truereplicaSet=atlas-xxxxxx-shard-0 ", true},
		// Invalid
		{"internal name", "mongodb://mongodb", false},
		{"domain name with mongodb", "mongodb://mongodb.company.com", false},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			require.Equal(t, tc.result, IsAtlasEndpoint(tc.endpoint))
		})
	}
}

func TestParseAtlasEndpoint(t *testing.T) {
	for _, tc := range []struct {
		desc      string
		endpoint  string
		result    string
		errAssert require.ErrorAssertionFunc
	}{
		// Valid
		{"public endpoint host only", "test.xxxxxxx.mongodb.net", "test", require.NoError},
		{"public endpoint", "mongodb://test.xxxxxxx.mongodb.net", "test", require.NoError},
		{"public endpoint with srv", "mongodb+srv://test.xxxxxxx.mongodb.net", "test", require.NoError},
		{
			"private link",
			"mongodb://pl-0-us-east-1-xxxxx.mongodb.net:1024,pl-0-us-east-1-xxxxx.mongodb.net:1025,pl-0-us-east-1-xxxxx.mongodb.net:1026/?ssl=true&authSource=admin&replicaSet=Cluster0-shard-0-shard-0",
			"pl-0-us-east-1-xxxxx",
			require.NoError,
		},
		{"private link with srv", "mongodb+srv://cluster0-pl-0-xxxxx.mongodb.net", "cluster0-pl-0-xxxxx", require.NoError},
		{"azure private link", "mongodb://cluster0-pl-0.xxxxx.azure.mongodb.net", "cluster0-pl-0", require.NoError},
		{
			"azure private link with srv",
			"mongodb://pl-0-eastus2.xxxxx.azure.mongodb.net:1024,pl-0-eastus2.xxxxx.azure.mongodb.net:1025,pl-0-eastus2.xxxxx.azure.mongodb.net:1026/?ssl=truereplicaSet=atlas-xxxxxx-shard-0 ",
			"pl-0-eastus2",
			require.NoError,
		},
		// Invalid
		{"internal name", "mongodb://mongodb", "", require.Error},
		{"domain name with mongodb", "mongodb://mongodb.company.com", "", require.Error},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			res, err := ParseAtlasEndpoint(tc.endpoint)
			tc.errAssert(t, err)
			require.Equal(t, tc.result, res)
		})
	}
}
