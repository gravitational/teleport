/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package types

import (
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestResourceSorter(t *testing.T) {
	t.Parallel()

	testValsUnordered := []string{"d", "b", "a", "c"}
	testValsAsc := []string{"a", "b", "c", "d"}
	testValsDesc := []string{"d", "c", "b", "a"}

	extractDbTypes := func(dbs []Database) []string {
		vals := make([]string, len(dbs))
		for i := range dbs {
			vals[i] = dbs[i].GetType()
		}
		return vals
	}

	cases := []struct {
		name         string
		wantErr      bool
		resourceKind string
		fieldName    string
	}{
		{
			name:         "apps by name",
			resourceKind: KindApp,
			fieldName:    ResourceFieldName,
		},
		{
			name:         "apps by description",
			resourceKind: KindApp,
			fieldName:    ResourceFieldDescription,
		},
		{
			name:         "apps by publicAddr",
			resourceKind: KindApp,
			fieldName:    ResourceFieldPublicAddr,
		},
		{
			name:         "databases by description",
			resourceKind: KindDatabase,
			fieldName:    ResourceFieldDescription,
		},
		{
			name:         "databases by name",
			resourceKind: KindDatabase,
			fieldName:    ResourceFieldName,
		},
		{
			name:         "databases by type",
			resourceKind: KindDatabase,
			fieldName:    ResourceFieldType,
		},
		{
			name:         "desktops by name",
			resourceKind: KindWindowsDesktop,
			fieldName:    ResourceFieldName,
		},
		{
			name:         "desktops by addr",
			resourceKind: KindWindowsDesktop,
			fieldName:    ResourceFieldAddr,
		},
		{
			name:         "kubernetes clusters by name",
			resourceKind: KindKubernetesCluster,
			fieldName:    ResourceFieldName,
		},
		{
			name:         "nodes by addr",
			resourceKind: KindNode,
			fieldName:    ResourceFieldAddr,
		},
		{
			name:         "nodes by hostname",
			resourceKind: KindNode,
			fieldName:    ResourceFieldHostname,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(fmt.Sprintf("%s desc", c.name), func(t *testing.T) {
			t.Parallel()

			sortBy := SortBy{Field: c.fieldName, Dir: SortDir_SORT_DIR_DESC}
			resources, err := makeDummyResources(testValsUnordered, sortBy.Field, c.resourceKind)
			require.NoError(t, err)
			rs := Resources(resources, c.resourceKind)
			require.NoError(t, rs.Sort(sortBy))

			switch {
			case c.resourceKind == KindDatabase && c.fieldName == ResourceFieldType:
				dbs, err := rs.asDatabases()
				require.NoError(t, err)
				require.IsDecreasing(t, extractDbTypes(dbs))
			default:
				resourcesDesc, err := makeDummyResources(testValsDesc, sortBy.Field, c.resourceKind)
				require.NoError(t, err)
				require.Equal(t, resourcesDesc, resources)
			}
		})

		t.Run(fmt.Sprintf("%s asc", c.name), func(t *testing.T) {
			t.Parallel()

			sortBy := SortBy{Field: c.fieldName, Dir: SortDir_SORT_DIR_ASC}
			resources, err := makeDummyResources(testValsUnordered, sortBy.Field, c.resourceKind)
			require.NoError(t, err)
			rs := Resources(resources, c.resourceKind)
			require.NoError(t, rs.Sort(sortBy))

			switch {
			case c.resourceKind == KindDatabase && c.fieldName == ResourceFieldType:
				dbs, err := rs.asDatabases()
				require.NoError(t, err)
				require.IsIncreasing(t, extractDbTypes(dbs))
			default:
				resourcesAsc, err := makeDummyResources(testValsAsc, sortBy.Field, c.resourceKind)
				require.NoError(t, err)
				require.Equal(t, resourcesAsc, resources)
			}
		})
	}
}

func TestResourceSorter_WithErrors(t *testing.T) {
	t.Parallel()

	testVals := []string{"d", "b", "a", "c"}
	resources, err := makeDummyResources(testVals, ResourceFieldName, KindApp)
	require.NoError(t, err)

	cases := []struct {
		name         string
		wantBadParam bool
		resourceKind string
		fieldName    string
	}{
		{
			name:         "unknowns",
			resourceKind: "kind_unknown",
			fieldName:    "field_unknown",
		},
		{
			name:         "unknown resource",
			resourceKind: "kind_unknown",
			fieldName:    ResourceFieldName,
		},
		{
			name:         "unknown field name",
			resourceKind: KindApp,
			fieldName:    "field_unknown",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			sortBy := SortBy{Field: c.fieldName}
			err := Resources(resources, c.resourceKind).Sort(sortBy)
			require.True(t, trace.IsNotImplemented(err))
		})
	}

	// Test mixed resources returns error.
	sortBy := SortBy{Field: ResourceFieldPublicAddr}
	resources, err = makeDummyResources(testVals, sortBy.Field, KindApp)
	require.NoError(t, err)

	sortBy2 := SortBy{Field: ResourceFieldHostname}
	resources2, err := makeDummyResources(testVals, sortBy2.Field, KindNode)
	require.NoError(t, err)

	mixedResources := append(resources, resources2...)
	err = Resources(mixedResources, KindApp).Sort(sortBy)
	require.True(t, trace.IsBadParameter(err))
}

func makeDummyResources(testVals []string, field string, resourceKind string) ([]Resource, error) {
	resources := make([]Resource, len(testVals))
	switch resourceKind {
	case KindApp:
		for i := 0; i < len(testVals); i++ {
			testVal := testVals[i]
			var err error
			resources[i], err = NewAppV3(Metadata{
				Name:        getFieldVal(field == ResourceFieldName, testVal),
				Description: getFieldVal(field == ResourceFieldDescription, testVal),
			}, AppSpecV3{
				PublicAddr: getFieldVal(field == ResourceFieldPublicAddr, testVal),
				URI:        "_",
			})
			if err != nil {
				return nil, err
			}
		}
	case KindNode:
		for i := 0; i < len(testVals); i++ {
			testVal := testVals[i]
			var err error
			resources[i], err = NewServer(
				getFieldVal(field == ResourceFieldName, testVal),
				KindNode,
				ServerSpecV2{
					Hostname: getFieldVal(field == ResourceFieldHostname, testVal),
				})
			if err != nil {
				return nil, err
			}
		}
	case KindDatabase:
		// DB types are hardcoded, values don't matter:
		dbSpecs := []DatabaseSpecV3{
			{
				Protocol: "_",
				URI:      "_",
				AWS: AWS{
					Redshift: Redshift{
						ClusterID: "_",
					},
				},
			},
			{
				Protocol: "_",
				URI:      "_",
				Azure: Azure{
					Name: "_",
				},
			},
			// rds
			{
				Protocol: "_",
				URI:      "_",
				AWS: AWS{
					Region: "_",
				},
			},
			{
				Protocol: "_",
				URI:      "_",
				GCP: GCPCloudSQL{
					ProjectID: "_",
				},
			},
		}
		for i := 0; i < len(testVals); i++ {
			dbSpec := dbSpecs[0]
			if field == ResourceFieldType {
				dbSpec = dbSpecs[i%len(dbSpecs)]
			}
			testVal := testVals[i]
			var err error
			resources[i], err = NewDatabaseV3(Metadata{
				Name:        getFieldVal(field == ResourceFieldName, testVal),
				Description: getFieldVal(field == ResourceFieldDescription, testVal),
			}, dbSpec)
			if err != nil {
				return nil, err
			}
		}
	case KindKubernetesCluster:
		for i := 0; i < len(testVals); i++ {
			testVal := testVals[i]
			var err error
			resources[i], err = NewKubernetesClusterV3FromLegacyCluster("_", &KubernetesCluster{
				Name: getFieldVal(field == ResourceFieldName, testVal),
			})
			if err != nil {
				return nil, err
			}
		}
	case KindWindowsDesktop:
		for i := 0; i < len(testVals); i++ {
			testVal := testVals[i]
			var err error
			resources[i], err = NewWindowsDesktopV3(
				getFieldVal(field == ResourceFieldName, testVal),
				nil,
				WindowsDesktopSpecV3{
					Addr: getFieldVal(field == ResourceFieldAddr, testVal),
				})
			if err != nil {
				return nil, err
			}
		}
	default:
		return nil, trace.NotImplemented("resource type %q is not defined", resourceKind)
	}

	return resources, nil
}

func getFieldVal(isTestField bool, testVal string) string {
	if isTestField {
		return testVal
	}

	return "_"
}
