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

package types

import (
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
)

// TestAppPublicAddrValidation tests PublicAddr field validation to make sure that
// an app with internal "kube-teleport-proxy-alpn." ServerName prefix won't be created.
func TestAppPublicAddrValidation(t *testing.T) {
	type check func(t *testing.T, err error)

	hasNoErr := func() check {
		return func(t *testing.T, err error) {
			require.NoError(t, err)
		}
	}
	hasErrTypeBadParameter := func() check {
		return func(t *testing.T, err error) {
			require.IsType(t, &trace.BadParameterError{}, err.(*trace.TraceErr).OrigError())
		}
	}

	tests := []struct {
		name       string
		publicAddr string
		check      check
	}{
		{
			name:       "kubernetes app",
			publicAddr: "kubernetes.example.com:3080",
			check:      hasNoErr(),
		},
		{
			name:       "kubernetes app public addr without port",
			publicAddr: "kubernetes.example.com",
			check:      hasNoErr(),
		},
		{
			name:       "kubernetes app http",
			publicAddr: "http://kubernetes.example.com:3080",
			check:      hasNoErr(),
		},
		{
			name:       "kubernetes app https",
			publicAddr: "https://kubernetes.example.com:3080",
			check:      hasNoErr(),
		},
		{
			name:       "public address with internal kube ServerName prefix",
			publicAddr: constants.KubeTeleportProxyALPNPrefix + "example.com:3080",
			check:      hasErrTypeBadParameter(),
		},
		{
			name:       "https public address with internal kube ServerName prefix",
			publicAddr: "https://" + constants.KubeTeleportProxyALPNPrefix + "example.com:3080",
			check:      hasErrTypeBadParameter(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewAppV3(Metadata{
				Name: "TestApp",
			}, AppSpecV3{
				PublicAddr: tc.publicAddr,
				URI:        "localhost:3080",
			})
			tc.check(t, err)
		})
	}
}

func TestAppServerSorter(t *testing.T) {
	t.Parallel()

	testValsUnordered := []string{"d", "b", "a", "c"}

	makeServers := func(testVals []string, testField string) []AppServer {
		servers := make([]AppServer, len(testVals))
		for i := 0; i < len(testVals); i++ {
			testVal := testVals[i]
			var err error
			servers[i], err = NewAppServerV3(Metadata{
				Name: "_",
			}, AppServerSpecV3{
				HostID: "_",
				App: &AppV3{
					Metadata: Metadata{
						Name:        getTestVal(testField == ResourceMetadataName, testVal),
						Description: getTestVal(testField == ResourceSpecDescription, testVal),
					},
					Spec: AppSpecV3{
						URI:        "_",
						PublicAddr: getTestVal(testField == ResourceSpecPublicAddr, testVal),
					},
				},
			})
			require.NoError(t, err)
		}
		return servers
	}

	cases := []struct {
		name      string
		fieldName string
	}{
		{
			name:      "by name",
			fieldName: ResourceMetadataName,
		},
		{
			name:      "by description",
			fieldName: ResourceSpecDescription,
		},
		{
			name:      "by publicAddr",
			fieldName: ResourceSpecPublicAddr,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(fmt.Sprintf("%s desc", c.name), func(t *testing.T) {
			sortBy := SortBy{Field: c.fieldName, IsDesc: true}
			servers := AppServers(makeServers(testValsUnordered, c.fieldName))
			require.NoError(t, servers.SortByCustom(sortBy))
			targetVals, err := servers.GetFieldVals(c.fieldName)
			require.NoError(t, err)
			require.IsDecreasing(t, targetVals)
		})

		t.Run(fmt.Sprintf("%s asc", c.name), func(t *testing.T) {
			sortBy := SortBy{Field: c.fieldName}
			servers := AppServers(makeServers(testValsUnordered, c.fieldName))
			require.NoError(t, servers.SortByCustom(sortBy))
			targetVals, err := servers.GetFieldVals(c.fieldName)
			require.NoError(t, err)
			require.IsIncreasing(t, targetVals)
		})
	}

	// Test error.
	sortBy := SortBy{Field: "unsupported"}
	servers := makeServers(testValsUnordered, "does-not-matter")
	require.True(t, trace.IsNotImplemented(AppServers(servers).SortByCustom(sortBy)))
}

func TestAppIsAWSConsole(t *testing.T) {
	tests := []struct {
		name               string
		uri                string
		cloud              string
		assertIsAWSConsole require.BoolAssertionFunc
	}{
		{
			name:               "AWS Standard",
			uri:                "https://console.aws.amazon.com/ec2/v2/home",
			assertIsAWSConsole: require.True,
		},
		{
			name:               "AWS China",
			uri:                "https://console.amazonaws.cn/console/home",
			assertIsAWSConsole: require.True,
		},
		{
			name:               "AWS GovCloud (US)",
			uri:                "https://console.amazonaws-us-gov.com/console/home",
			assertIsAWSConsole: require.True,
		},
		{
			name:               "Region based not supported yet",
			uri:                "https://us-west-1.console.aws.amazon.com",
			assertIsAWSConsole: require.False,
		},
		{
			name:               "Not an AWS Console URL",
			uri:                "https://hello.world",
			assertIsAWSConsole: require.False,
		},
		{
			name:               "CLI-only AWS App",
			cloud:              CloudAWS,
			assertIsAWSConsole: require.True,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, err := NewAppV3(Metadata{
				Name: "aws",
			}, AppSpecV3{
				URI:   test.uri,
				Cloud: test.cloud,
			})
			require.NoError(t, err)

			test.assertIsAWSConsole(t, app.IsAWSConsole())
		})
	}
}

func TestApplicationGetAWSExternalID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		appAWS             *AppAWS
		expectedExternalID string
	}{
		{
			name: "not configured",
		},
		{
			name: "configured",
			appAWS: &AppAWS{
				ExternalID: "default-external-id",
			},
			expectedExternalID: "default-external-id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, err := NewAppV3(Metadata{
				Name: "aws",
			}, AppSpecV3{
				URI: constants.AWSConsoleURL,
				AWS: test.appAWS,
			})
			require.NoError(t, err)

			require.Equal(t, test.expectedExternalID, app.GetAWSExternalID())
		})
	}
}

func TestAppIsAzureCloud(t *testing.T) {
	tests := []struct {
		name     string
		cloud    string
		expected bool
	}{
		{
			name:     "Azure Cloud",
			cloud:    CloudAzure,
			expected: true,
		},
		{
			name:     "not Azure Cloud",
			cloud:    CloudAWS,
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, err := NewAppV3(Metadata{Name: "myapp"}, AppSpecV3{Cloud: test.cloud})
			require.NoError(t, err)
			require.Equal(t, test.expected, app.IsAzureCloud())
		})
	}
}

func TestNewAppV3(t *testing.T) {
	tests := []struct {
		name    string
		meta    Metadata
		spec    AppSpecV3
		want    *AppV3
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:    "empty app",
			meta:    Metadata{},
			spec:    AppSpecV3{},
			want:    nil,
			wantErr: require.Error,
		},
		{
			name: "non-cloud app",
			meta: Metadata{
				Name:        "myapp",
				Description: "my fancy app",
				ID:          123,
			},
			spec: AppSpecV3{URI: "example.com"},
			want: &AppV3{
				Kind:    "app",
				Version: "v3",
				Metadata: Metadata{
					Name:        "myapp",
					Namespace:   "default",
					Description: "my fancy app",
					ID:          123,
				}, Spec: AppSpecV3{URI: "example.com"},
			},
			wantErr: require.NoError,
		},
		{
			name: "non-cloud app #2",
			meta: Metadata{
				Name:        "myapp",
				Description: "my fancy app",
				ID:          123,
			},
			spec: AppSpecV3{URI: "example.com"},
			want: &AppV3{
				Kind:    "app",
				Version: "v3",
				Metadata: Metadata{
					Name:        "myapp",
					Namespace:   "default",
					Description: "my fancy app",
					ID:          123,
				},
				Spec: AppSpecV3{URI: "example.com"},
			},
			wantErr: require.NoError,
		},
		{
			name: "azure app",
			meta: Metadata{Name: "myazure"},
			spec: AppSpecV3{Cloud: CloudAzure},
			want: &AppV3{
				Kind:     "app",
				Version:  "v3",
				Metadata: Metadata{Name: "myazure", Namespace: "default"},
				Spec:     AppSpecV3{URI: "cloud://Azure", Cloud: CloudAzure},
			},
			wantErr: require.NoError,
		},
		{
			name: "aws app CLI only",
			meta: Metadata{Name: "myaws"},
			spec: AppSpecV3{Cloud: CloudAWS},
			want: &AppV3{
				Kind:     "app",
				Version:  "v3",
				Metadata: Metadata{Name: "myaws", Namespace: "default"},
				Spec:     AppSpecV3{URI: "cloud://AWS", Cloud: CloudAWS},
			},
			wantErr: require.NoError,
		},
		{
			name: "aws app console",
			meta: Metadata{Name: "myaws"},
			spec: AppSpecV3{Cloud: CloudAWS, URI: constants.AWSConsoleURL},
			want: &AppV3{
				Kind:     "app",
				Version:  "v3",
				Metadata: Metadata{Name: "myaws", Namespace: "default"},
				Spec:     AppSpecV3{URI: constants.AWSConsoleURL, Cloud: CloudAWS},
			},
			wantErr: require.NoError,
		},
		{
			name: "aws app using integration",
			meta: Metadata{Name: "myaws"},
			spec: AppSpecV3{Cloud: CloudAWS, URI: constants.AWSConsoleURL, Integration: "my-integration"},
			want: &AppV3{
				Kind:     "app",
				Version:  "v3",
				Metadata: Metadata{Name: "myaws", Namespace: "default"},
				Spec:     AppSpecV3{URI: constants.AWSConsoleURL, Cloud: CloudAWS, Integration: "my-integration"},
			},
			wantErr: require.NoError,
		},
		{
			name:    "invalid cloud identifier",
			meta:    Metadata{Name: "dummy"},
			spec:    AppSpecV3{Cloud: "dummy"},
			want:    nil,
			wantErr: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := NewAppV3(tt.meta, tt.spec)
			tt.wantErr(t, err)
			require.Equal(t, tt.want, actual)
		})
	}
}
