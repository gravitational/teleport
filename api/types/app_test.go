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
)

// TestAppPublicAddrValidation tests PublicAddr field validation to make sure that
// an app with internal "kube." ServerName prefix won't be created.
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
			publicAddr: "kube.example.com:3080",
			check:      hasErrTypeBadParameter(),
		},
		{
			name:       "https public address with internal kube ServerName prefix",
			publicAddr: "https://kube.example.com:3080",
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
		name         string
		uri          string
		isAWSConsole bool
	}{
		{
			name:         "AWS Standard",
			uri:          "https://console.aws.amazon.com/ec2/v2/home",
			isAWSConsole: true,
		},
		{
			name:         "AWS China",
			uri:          "https://console.amazonaws.cn/console/home",
			isAWSConsole: true,
		},
		{
			name:         "AWS GovCloud (US)",
			uri:          "https://console.amazonaws-us-gov.com/console/home",
			isAWSConsole: true,
		},
		{
			name:         "Region based not supported yet",
			uri:          "https://us-west-1.console.aws.amazon.com",
			isAWSConsole: false,
		},
		{
			name:         "Not an AWS Console URL",
			uri:          "https://hello.world",
			isAWSConsole: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, err := NewAppV3(Metadata{
				Name: "TestApp",
			}, AppSpecV3{
				URI: test.uri,
			})
			require.NoError(t, err)
			if test.isAWSConsole {
				require.True(t, app.IsAWSConsole())
			} else {
				require.False(t, app.IsAWSConsole())
			}
		})
	}
}
