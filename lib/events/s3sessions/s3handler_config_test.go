// Copyright 2022 Gravitational, Inc
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

package s3sessions

import (
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestConfig_SetFromURL(t *testing.T) {
	useFipsCfg := Config{
		UseFIPSEndpoint: types.ClusterAuditConfigSpecV2_FIPS_ENABLED,
	}
	cases := []struct {
		name         string
		url          string
		cfg          Config
		cfgAssertion func(*testing.T, Config)
	}{
		{
			name: "fips enabled via url",
			url:  "s3://bucket/audit?insecure=true&disablesse=true&acl=private&use_fips_endpoint=true&sse_kms_key=abcdefg",
			cfgAssertion: func(t *testing.T, config Config) {

				var (
					expectedBucket = "bucket"
					expectedACL    = "private"
					expectedRegion = "us-east-1"
					sseKMSKey      = "abcdefg"
				)
				require.Equal(t, expectedBucket, config.Bucket)
				require.Equal(t, expectedACL, config.ACL)
				require.Equal(t, sseKMSKey, config.SSEKMSKey)
				require.Equal(t, expectedRegion, config.Region)
				require.Equal(t, types.ClusterAuditConfigSpecV2_FIPS_ENABLED, config.UseFIPSEndpoint)
				require.True(t, config.Insecure)
				require.True(t, config.DisableServerSideEncryption)
			},
		},
		{
			name: "fips disabled via url",
			url:  "s3://bucket/audit?insecure=false&disablesse=false&use_fips_endpoint=false&endpoint=s3.example.com",
			cfgAssertion: func(t *testing.T, config Config) {

				var (
					expectedBucket   = "bucket"
					expectedEndpoint = "s3.example.com"
				)

				require.Equal(t, expectedBucket, config.Bucket)
				require.Equal(t, expectedEndpoint, config.Endpoint)

				require.False(t, config.Insecure)
				require.False(t, config.DisableServerSideEncryption)

				require.Equal(t, types.ClusterAuditConfigSpecV2_FIPS_DISABLED, config.UseFIPSEndpoint)
			},
		},
		{
			name: "fips mode not set",
			url:  "s3://bucket/audit",
			cfgAssertion: func(t *testing.T, config Config) {
				require.Equal(t, "bucket", config.Bucket)
				require.Equal(t, types.ClusterAuditConfigSpecV2_FIPS_UNSET, config.UseFIPSEndpoint)
			},
		},
		{
			name: "fips mode enabled by default",
			url:  "s3://bucket/audit",
			cfg:  useFipsCfg,
			cfgAssertion: func(t *testing.T, config Config) {
				require.Equal(t, types.ClusterAuditConfigSpecV2_FIPS_ENABLED, config.UseFIPSEndpoint)
			},
		},
		{
			name: "fips mode can be overridden",
			url:  "s3://bucket/audit?use_fips_endpoint=false",
			cfg:  useFipsCfg,
			cfgAssertion: func(t *testing.T, config Config) {
				require.Equal(t, types.ClusterAuditConfigSpecV2_FIPS_DISABLED, config.UseFIPSEndpoint)
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			uri, err := url.Parse(tt.url)
			require.NoError(t, err)
			require.NoError(t, tt.cfg.SetFromURL(uri, "us-east-1"))

			tt.cfgAssertion(t, tt.cfg)
		})
	}
}
