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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMatchSearch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		expectMatch require.BoolAssertionFunc
		fieldVals   []string
		searchVals  []string
		customFn    func(v string) bool
	}{
		{
			name:        "no match",
			expectMatch: require.False,
			fieldVals:   []string{"foo", "bar", "baz"},
			searchVals:  []string{"cat"},
			customFn: func(v string) bool {
				return false
			},
		},
		{
			name:        "no match for partial match",
			expectMatch: require.False,
			fieldVals:   []string{"foo"},
			searchVals:  []string{"foo", "dog"},
		},
		{
			name:        "no match for partial custom match",
			expectMatch: require.False,
			fieldVals:   []string{"foo", "bar", "baz"},
			searchVals:  []string{"foo", "bee", "rat"},
			customFn: func(v string) bool {
				return v == "bee"
			},
		},
		{
			name:        "no match for search phrase",
			expectMatch: require.False,
			fieldVals:   []string{"foo", "dog", "dog foo", "foodog"},
			searchVals:  []string{"foo dog"},
		},
		{
			name:        "match",
			expectMatch: require.True,
			fieldVals:   []string{"foo", "bar", "baz"},
			searchVals:  []string{"baz"},
		},
		{
			name:        "match with nil search values",
			expectMatch: require.True,
		},
		{
			name:        "match with repeat search vals",
			expectMatch: require.True,
			fieldVals:   []string{"foo", "bar", "baz"},
			searchVals:  []string{"foo", "foo", "baz"},
		},
		{
			name:        "match for a list of search vals contained within one field value",
			expectMatch: require.True,
			fieldVals:   []string{"foo barbaz"},
			searchVals:  []string{"baz", "foo", "bar"},
		},
		{
			name:        "match with mix of single vals and phrases",
			expectMatch: require.True,
			fieldVals:   []string{"foo baz", "bar"},
			searchVals:  []string{"baz", "foo", "foo baz", "bar"},
		},
		{
			name:        "match ignore case",
			expectMatch: require.True,
			fieldVals:   []string{"FOO barBaz"},
			searchVals:  []string{"baZ", "foo", "BaR"},
		},
		{
			name:        "match with custom match",
			expectMatch: require.True,
			fieldVals:   []string{"foo", "bar", "baz"},
			searchVals:  []string{"foo", "bar", "tunnel"},
			customFn: func(v string) bool {
				return v == "tunnel"
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			matched := MatchSearch(tc.fieldVals, tc.searchVals, tc.customFn)
			tc.expectMatch(t, matched)
		})
	}
}

func TestMatchSearch_ResourceSpecific(t *testing.T) {
	t.Parallel()

	labels := map[string]string{"env": "prod", "os": "mac"}

	cases := []struct {
		name string
		// searchNotDefined refers to resources where the searcheable field values are not defined.
		searchNotDefined   bool
		matchingSearchVals []string
		newResource        func(*testing.T) ResourceWithLabels
	}{
		{
			name:               "node",
			matchingSearchVals: []string{"foo", "bar", "prod", "os"},
			newResource: func(t *testing.T) ResourceWithLabels {
				server, err := NewServerWithLabels("_", KindNode, ServerSpecV2{
					Hostname: "foo",
					Addr:     "bar",
				}, labels)
				require.NoError(t, err)

				return server
			},
		},
		{
			name:               "node using tunnel",
			matchingSearchVals: []string{"tunnel"},
			newResource: func(t *testing.T) ResourceWithLabels {
				server, err := NewServer("_", KindNode, ServerSpecV2{
					UseTunnel: true,
				})
				require.NoError(t, err)

				return server
			},
		},
		{
			name:               "windows desktop",
			matchingSearchVals: []string{"foo", "bar", "env", "prod", "os"},
			newResource: func(t *testing.T) ResourceWithLabels {
				desktop, err := NewWindowsDesktopV3("foo", labels, WindowsDesktopSpecV3{
					Addr: "bar",
				})
				require.NoError(t, err)

				return desktop
			},
		},
		{
			name:               "application",
			matchingSearchVals: []string{"foo", "bar", "baz", "mac"},
			newResource: func(t *testing.T) ResourceWithLabels {
				app, err := NewAppV3(Metadata{
					Name:        "foo",
					Description: "bar",
					Labels:      labels,
				}, AppSpecV3{
					PublicAddr: "baz",
					URI:        "_",
				})
				require.NoError(t, err)

				return app
			},
		},
		{
			name:               "kube cluster",
			matchingSearchVals: []string{"foo", "prod", "env"},
			newResource: func(t *testing.T) ResourceWithLabels {
				kc, err := NewKubernetesClusterV3FromLegacyCluster("_", &KubernetesCluster{
					Name:         "foo",
					StaticLabels: labels,
				})
				require.NoError(t, err)

				return kc
			},
		},
		{
			name:               "database",
			matchingSearchVals: []string{"foo", "bar", "baz", "prod", DatabaseTypeRedshift},
			newResource: func(t *testing.T) ResourceWithLabels {
				db, err := NewDatabaseV3(Metadata{
					Name:        "foo",
					Description: "bar",
					Labels:      labels,
				}, DatabaseSpecV3{
					Protocol: "baz",
					URI:      "_",
					AWS: AWS{
						Redshift: Redshift{
							ClusterID: "_",
						},
					},
				})
				require.NoError(t, err)

				return db
			},
		},
		{
			name:               "database with gcp keywords",
			matchingSearchVals: []string{"cloud", "cloud sql"},
			newResource: func(t *testing.T) ResourceWithLabels {
				db, err := NewDatabaseV3(Metadata{
					Name:   "foo",
					Labels: labels,
				}, DatabaseSpecV3{
					Protocol: "_",
					URI:      "_",
					GCP: GCPCloudSQL{
						ProjectID:  "_",
						InstanceID: "_",
					},
				})
				require.NoError(t, err)

				return db
			},
		},
		{
			name:             "app server",
			searchNotDefined: true,
			newResource: func(t *testing.T) ResourceWithLabels {
				appServer, err := NewAppServerV3(Metadata{
					Name: "foo",
				}, AppServerSpecV3{
					HostID: "_",
					App:    &AppV3{Metadata: Metadata{Name: "_"}, Spec: AppSpecV3{URI: "_"}},
				})
				require.NoError(t, err)

				return appServer
			},
		},
		{
			name:             "db server",
			searchNotDefined: true,
			newResource: func(t *testing.T) ResourceWithLabels {
				dbServer, err := NewDatabaseServerV3(Metadata{
					Name: "foo",
				}, DatabaseServerSpecV3{
					HostID:   "_",
					Hostname: "_",
				})
				require.NoError(t, err)

				return dbServer
			},
		},
		{
			name:             "kube service",
			searchNotDefined: true,
			newResource: func(t *testing.T) ResourceWithLabels {
				kubeServer, err := NewKubernetesServerV3(
					Metadata{
						Name: "foo",
					}, KubernetesServerSpecV3{
						HostID:   "_",
						Hostname: "_",
						Cluster: &KubernetesClusterV3{
							Metadata: Metadata{
								Name: "_",
							},
						},
					})
				require.NoError(t, err)

				return kubeServer
			},
		},
		{
			name:             "desktop service",
			searchNotDefined: true,
			newResource: func(t *testing.T) ResourceWithLabels {
				desktopService, err := NewWindowsDesktopServiceV3(Metadata{
					Name: "foo",
				}, WindowsDesktopServiceSpecV3{
					Addr:            "_",
					TeleportVersion: "_",
				})
				require.NoError(t, err)

				return desktopService
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resource := tc.newResource(t)

			// Nil search values, should always return true
			match := resource.MatchSearch(nil)
			require.True(t, match)

			switch {
			case tc.searchNotDefined:
				// Trying to search something in resources without search field values defined
				// should always return false.
				match := resource.MatchSearch([]string{"_"})
				require.False(t, match)
			default:
				// Test no match.
				match := resource.MatchSearch([]string{"foo", "llama"})
				require.False(t, match)

				// Test match.
				match = resource.MatchSearch(tc.matchingSearchVals)
				require.True(t, match)
			}
		})
	}
}

func TestResourcesWithLabels_ToMap(t *testing.T) {
	mkServerHost := func(name string, hostname string) ResourceWithLabels {
		server, err := NewServerWithLabels(name, KindNode, ServerSpecV2{
			Hostname: hostname + ".example.com",
			Addr:     name + ".example.com",
		}, nil)
		require.NoError(t, err)

		return server
	}

	mkServer := func(name string) ResourceWithLabels {
		return mkServerHost(name, name)
	}

	tests := []struct {
		name string
		r    ResourcesWithLabels
		want ResourcesWithLabelsMap
	}{
		{
			name: "empty",
			r:    nil,
			want: map[string]ResourceWithLabels{},
		},
		{
			name: "simple list",
			r:    []ResourceWithLabels{mkServer("a"), mkServer("b"), mkServer("c")},
			want: map[string]ResourceWithLabels{
				"a": mkServer("a"),
				"b": mkServer("b"),
				"c": mkServer("c"),
			},
		},
		{
			name: "first duplicate wins",
			r:    []ResourceWithLabels{mkServerHost("a", "a1"), mkServerHost("a", "a2"), mkServerHost("a", "a3")},
			want: map[string]ResourceWithLabels{
				"a": mkServerHost("a", "a1"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.r.ToMap(), tt.want)
		})
	}
}

func TestValidLabelKey(t *testing.T) {
	for _, tc := range []struct {
		label string
		valid bool
	}{
		{
			label: "1x/Y*_-",
			valid: true,
		},
		{
			label: "x:y",
			valid: true,
		},
		{
			label: "x\\y",
			valid: false,
		},
	} {
		isValid := IsValidLabelKey(tc.label)
		require.Equal(t, tc.valid, isValid)
	}
}
