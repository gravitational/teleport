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

// TestIsAppServer tests that the IsAppServer method correctly determines whether the AppServerOrSAMLIdPServiceProvider
// represents an AppServer.
func TestIsAppServer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		appOrSP           AppServerOrSAMLIdPServiceProvider
		shouldBeAppServer bool
	}{
		{
			name: "isAppServer should return true when AppOrSP represents an AppServer",
			appOrSP: &AppServerOrSAMLIdPServiceProviderV1{
				Resource: &AppServerOrSAMLIdPServiceProviderV1_AppServer{
					AppServer: newAppServer(t, "test-appserver").(*AppServerV3),
				},
			},
			shouldBeAppServer: true,
		},
		{
			name: "isAppServer should return false when AppOrSP represents a SAMLIdPServiceProvider",
			appOrSP: &AppServerOrSAMLIdPServiceProviderV1{
				Resource: &AppServerOrSAMLIdPServiceProviderV1_SAMLIdPServiceProvider{
					SAMLIdPServiceProvider: newSAMLIdPServiceProvider(t, "test-sp").(*SAMLIdPServiceProviderV1),
				},
			},
			shouldBeAppServer: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isAppServer := tc.appOrSP.IsAppServer()
			require.Equal(t, tc.shouldBeAppServer, isAppServer)
		})
	}
}

// TestSortAppServersOrSAMLIdPServiceProviders tests that the sorting of AppServers and SAMLIdPServiceProviders works correctly.
func TestSortAppServersOrSAMLIdPServiceProviders(t *testing.T) {
	t.Parallel()

	testValsUnordered := []string{"d", "b", "a", "c", "e"}

	makeAppsAndSPs := func(testVals []string, testField string) []AppServerOrSAMLIdPServiceProvider {
		appsAndSPs := make([]AppServerOrSAMLIdPServiceProvider, len(testVals))
		for i := 0; i < len(testVals); i++ {
			testVal := testVals[i]
			var err error
			if i%2 == 0 {
				appsAndSPs[i], err = createAppServerOrSPFromAppServer(NewAppServerV3(Metadata{
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
				}))
			} else {
				appsAndSPs[i], err = createAppServerOrSPFromSP(NewSAMLIdPServiceProvider(Metadata{
					Name: getTestVal(testField == ResourceMetadataName, testVal),
				},
					SAMLIdPServiceProviderSpecV1{
						EntityDescriptor: "_",
						EntityID:         "_",
					}))
			}
			require.NoError(t, err)
		}
		return appsAndSPs
	}

	t.Run(fmt.Sprintf("%s desc", "sort by name"), func(t *testing.T) {
		sortBy := SortBy{Field: ResourceMetadataName, IsDesc: true}
		appsAndSPs := AppServersOrSAMLIdPServiceProviders(makeAppsAndSPs(testValsUnordered, ResourceMetadataName))
		require.NoError(t, appsAndSPs.SortByCustom(sortBy))
		targetVals, err := appsAndSPs.GetFieldVals(ResourceMetadataName)
		require.NoError(t, err)
		require.IsDecreasing(t, targetVals)
	})

	t.Run(fmt.Sprintf("%s asc", "sort by name"), func(t *testing.T) {
		sortBy := SortBy{Field: ResourceMetadataName}
		appsAndSPs := AppServersOrSAMLIdPServiceProviders(makeAppsAndSPs(testValsUnordered, ResourceMetadataName))
		require.NoError(t, appsAndSPs.SortByCustom(sortBy))
		targetVals, err := appsAndSPs.GetFieldVals(ResourceMetadataName)
		require.NoError(t, err)
		require.IsIncreasing(t, targetVals)
	})

	// Test error.
	sortBy := SortBy{Field: "unsupported"}
	appsAndSPs := makeAppsAndSPs(testValsUnordered, "test-field")
	require.True(t, trace.IsNotImplemented(AppServersOrSAMLIdPServiceProviders(appsAndSPs).SortByCustom(sortBy)))
}

// createAppServerOrSPFromAppServer returns a AppServerOrSAMLIdPServiceProvider given an AppServer.
func createAppServerOrSPFromAppServer(appServer AppServer, err error) (AppServerOrSAMLIdPServiceProvider, error) {
	if err != nil {
		return nil, trace.Wrap(err)
	}
	appServerOrSP := &AppServerOrSAMLIdPServiceProviderV1{
		Resource: &AppServerOrSAMLIdPServiceProviderV1_AppServer{
			AppServer: appServer.(*AppServerV3),
		},
	}

	return appServerOrSP, nil
}

// createAppServerOrSPFromApp returns a AppServerOrSAMLIdPServiceProvider given a SAMLIdPServiceProvider.
func createAppServerOrSPFromSP(sp SAMLIdPServiceProvider, err error) (AppServerOrSAMLIdPServiceProvider, error) {
	if err != nil {
		return nil, trace.Wrap(err)
	}
	appServerOrSP := &AppServerOrSAMLIdPServiceProviderV1{
		Resource: &AppServerOrSAMLIdPServiceProviderV1_SAMLIdPServiceProvider{
			SAMLIdPServiceProvider: sp.(*SAMLIdPServiceProviderV1),
		},
	}

	return appServerOrSP, nil
}

func newAppServer(t *testing.T, name string) AppServer {
	app, err := NewAppServerV3(Metadata{
		Name:        name,
		Description: "description",
	}, AppServerSpecV3{
		HostID: "hostid",
		App: &AppV3{
			Metadata: Metadata{
				Name:        fmt.Sprintf("%s-app", name),
				Description: "app description",
			},
			Spec: AppSpecV3{
				URI:        "uri",
				PublicAddr: "publicaddr",
			},
		},
	})
	require.NoError(t, err)
	return app
}

func newSAMLIdPServiceProvider(t *testing.T, name string) SAMLIdPServiceProvider {
	app, err := NewSAMLIdPServiceProvider(Metadata{
		Name: name,
	},
		SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: fmt.Sprintf("<EntityDescriptor>%s</EntityDescriptor>", name),
			EntityID:         fmt.Sprintf("%s-id", name),
		})
	require.NoError(t, err)
	return app
}
