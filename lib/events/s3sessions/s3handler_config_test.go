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

package s3sessions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
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

func TestUploadMetadata(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	handler, err := NewHandler(context.Background(), Config{
		Region:   "us-west-1",
		Path:     "/test/",
		Bucket:   "teleport-unit-tests",
		Endpoint: server.URL,
		CredentialsProvider: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{}, nil
		}),
	})
	require.NoError(t, err)
	defer handler.Close()

	meta := handler.GetUploadMetadata("test-session-id")
	require.Equal(t, "s3://teleport-unit-tests/test/test-session-id", meta.URL)
}

func TestEndpoints(t *testing.T) {
	tests := []struct {
		name string
		fips bool
	}{
		{
			name: "fips",
			fips: true,
		},
		{
			name: "without fips",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fips := types.ClusterAuditConfigSpecV2_FIPS_DISABLED
			if tt.fips {
				fips = types.ClusterAuditConfigSpecV2_FIPS_ENABLED
				modules.SetTestModules(t, &modules.TestModules{
					FIPS: true,
				})
			}

			var request *http.Request
			var once sync.Once
			mux := http.NewServeMux()
			mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				once.Do(func() { request = r.Clone(context.Background()) })
				w.WriteHeader(http.StatusTeapot)
			}))

			server := httptest.NewServer(mux)
			t.Cleanup(server.Close)

			handler, err := NewHandler(context.Background(), Config{
				Region: "us-west-1",
				Path:   "/test/",
				Bucket: "teleport-unit-tests",
				// The prefix is intentionally removed to validate that a scheme
				// is applied automatically. This validates backwards compatible behavior
				// with existing configurations and the behavior change from aws-sdk-go to aws-sdk-go-v2.
				Endpoint:        strings.TrimPrefix(server.URL, "http://"),
				UseFIPSEndpoint: fips,
				Insecure:        true,
				CredentialsProvider: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
					return aws.Credentials{}, nil
				}),
			})
			// FIPS mode should fail because it is a violation to enable FIPS
			// while also setting a custom endpoint.
			if tt.fips {
				assert.Error(t, err)
				require.ErrorContains(t, err, "FIPS")
				return
			}

			require.NoError(t, err)
			defer handler.Close()
			require.NotNil(t, request.URL)
			require.Equal(t, "/teleport-unit-tests", request.URL.Path)
		})
	}
}
