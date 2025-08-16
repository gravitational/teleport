/*
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
	"slices"
	"testing"
	"time"

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

func TestUnifiedNameCompare(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name      string
		resourceA func(*testing.T) ResourceWithLabels
		resourceB func(*testing.T) ResourceWithLabels
		isDesc    bool
		expect    bool
	}{
		{
			name: "sort by same kind",
			resourceA: func(t *testing.T) ResourceWithLabels {
				server, err := NewServer("node-cloud", KindNode, ServerSpecV2{
					Hostname: "node-cloud",
				})
				require.NoError(t, err)
				return server
			},
			resourceB: func(t *testing.T) ResourceWithLabels {
				server, err := NewServer("node-strawberry", KindNode, ServerSpecV2{
					Hostname: "node-strawberry",
				})
				require.NoError(t, err)
				return server
			},
			isDesc: true,
			expect: false,
		},
		{
			name: "sort by different kind",
			resourceA: func(t *testing.T) ResourceWithLabels {
				server := newAppServer(t, "app-cloud")
				return server
			},
			resourceB: func(t *testing.T) ResourceWithLabels {
				server, err := NewServer("node-strawberry", KindNode, ServerSpecV2{
					Hostname: "node-strawberry",
				})
				require.NoError(t, err)
				return server
			},
			isDesc: true,
			expect: false,
		},
		{
			name: "sort with different cases",
			resourceA: func(t *testing.T) ResourceWithLabels {
				server := newAppServer(t, "app-cloud")
				return server
			},
			resourceB: func(t *testing.T) ResourceWithLabels {
				server, err := NewServer("Node-strawberry", KindNode, ServerSpecV2{
					Hostname: "node-strawberry",
				})
				require.NoError(t, err)
				return server
			},
			isDesc: true,
			expect: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		resourceA := tc.resourceA(t)
		resourceB := tc.resourceB(t)
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actual := unifiedNameCompare(resourceA, resourceB, tc.isDesc)
			if actual != tc.expect {
				t.Errorf("Expected %v, but got %v for %+v and %+v with isDesc=%v", tc.expect, actual, resourceA, resourceB, tc.isDesc)
			}
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
				kc, err := NewKubernetesClusterV3FromLegacyCluster("", &KubernetesCluster{
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
				dbServer, err := NewDatabaseServerV3(Metadata{
					Name: "foo",
				}, DatabaseServerSpecV3{
					HostID:   "_",
					Hostname: "_",
					Database: db,
				})
				require.NoError(t, err)

				return dbServer
			},
		},
		{
			name:             "kube server",
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
			require.Equal(t, tt.want, tt.r.ToMap())
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

func TestFriendlyName(t *testing.T) {
	newApp := func(t *testing.T, name, description string, labels map[string]string) Application {
		app, err := NewAppV3(Metadata{
			Name:        name,
			Description: description,
			Labels:      labels,
		}, AppSpecV3{
			URI: "https://some-uri.com",
		})
		require.NoError(t, err)

		return app
	}

	newGroup := func(t *testing.T, name, description string, labels map[string]string) UserGroup {
		group, err := NewUserGroup(Metadata{
			Name:        name,
			Description: description,
			Labels:      labels,
		}, UserGroupSpecV1{})
		require.NoError(t, err)

		return group
	}

	newRole := func(t *testing.T, name string, labels map[string]string) Role {
		role, err := NewRole(name, RoleSpecV6{})
		require.NoError(t, err)
		metadata := role.GetMetadata()
		metadata.Labels = labels
		role.SetMetadata(metadata)
		return role
	}

	node, err := NewServer("node", KindNode, ServerSpecV2{
		Hostname: "friendly hostname",
	})
	require.NoError(t, err)

	tests := []struct {
		name     string
		resource ResourceWithLabels
		expected string
	}{
		{
			name:     "no friendly name",
			resource: newApp(t, "no friendly", "no friendly", map[string]string{}),
			expected: "",
		},
		{
			name: "friendly app name (uses description)",
			resource: newApp(t, "friendly", "friendly name", map[string]string{
				OriginLabel: OriginOkta,
			}),
			expected: "friendly name",
		},
		{
			name: "friendly app name (uses label)",
			resource: newApp(t, "friendly", "friendly name", map[string]string{
				OriginLabel:      OriginOkta,
				OktaAppNameLabel: "label friendly name",
			}),
			expected: "label friendly name",
		},
		{
			name: "friendly group name (uses description)",
			resource: newGroup(t, "friendly", "friendly name", map[string]string{
				OriginLabel: OriginOkta,
			}),
			expected: "friendly name",
		},
		{
			name: "friendly group name (uses label)",
			resource: newGroup(t, "friendly", "friendly name", map[string]string{
				OriginLabel:        OriginOkta,
				OktaGroupNameLabel: "label friendly name",
			}),
			expected: "label friendly name",
		},
		{
			name: "friendly role name (uses label)",
			resource: newRole(t, "friendly", map[string]string{
				OriginLabel:       OriginOkta,
				OktaRoleNameLabel: "label friendly name",
			}),
			expected: "label friendly name",
		},
		{
			name:     "friendly node name",
			resource: node,
			expected: "friendly hostname",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, FriendlyName(test.resource))
		})
	}
}

func TestMetadataIsEqual(t *testing.T) {
	newMetadata := func(changeFns ...func(*Metadata)) *Metadata {
		metadata := &Metadata{
			Name:        "name",
			Namespace:   "namespace",
			Description: "description",
			Labels:      map[string]string{"label1": "value1"},
			Expires:     &time.Time{},
			Revision:    "aaaa",
		}

		for _, fn := range changeFns {
			fn(metadata)
		}

		return metadata
	}
	tests := []struct {
		name     string
		m1       *Metadata
		m2       *Metadata
		expected bool
	}{
		{
			name:     "empty equals",
			m1:       &Metadata{},
			m2:       &Metadata{},
			expected: true,
		},
		{
			name:     "nil equals",
			m1:       nil,
			m2:       (*Metadata)(nil),
			expected: true,
		},
		{
			name:     "one is nil",
			m1:       &Metadata{},
			m2:       (*Metadata)(nil),
			expected: false,
		},
		{
			name:     "populated equals",
			m1:       newMetadata(),
			m2:       newMetadata(),
			expected: true,
		},
		{
			name: "id and revision have no effect",
			m1:   newMetadata(),
			m2: newMetadata(func(m *Metadata) {
				m.Revision = "bbbb"
			}),
			expected: true,
		},
		{
			name: "name is different",
			m1:   newMetadata(),
			m2: newMetadata(func(m *Metadata) {
				m.Name = "different-name"
			}),
			expected: false,
		},
		{
			name: "namespace is different",
			m1:   newMetadata(),
			m2: newMetadata(func(m *Metadata) {
				m.Namespace = "different-namespace"
			}),
			expected: false,
		},
		{
			name: "description is different",
			m1:   newMetadata(),
			m2: newMetadata(func(m *Metadata) {
				m.Description = "different-description"
			}),
			expected: false,
		},
		{
			name: "labels is different",
			m1:   newMetadata(),
			m2: newMetadata(func(m *Metadata) {
				m.Labels = map[string]string{"label2": "value2"}
			}),
			expected: false,
		},
		{
			name: "expires is different",
			m1:   newMetadata(),
			m2: newMetadata(func(m *Metadata) {
				newTime := time.Date(1, 2, 3, 4, 5, 6, 7, time.UTC)
				m.Expires = &newTime
			}),
			expected: false,
		},
		{
			name:     "expires both nil",
			m1:       newMetadata(func(m *Metadata) { m.Expires = nil }),
			m2:       newMetadata(func(m *Metadata) { m.Expires = nil }),
			expected: true,
		},
		{
			name:     "expires m1 nil",
			m1:       newMetadata(func(m *Metadata) { m.Expires = nil }),
			m2:       newMetadata(),
			expected: false,
		},
		{
			name:     "expires m2 nil",
			m1:       newMetadata(),
			m2:       newMetadata(func(m *Metadata) { m.Expires = nil }),
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, test.m1.IsEqual(test.m2))
		})
	}
}

func TestResourceHeaderIsEqual(t *testing.T) {
	newHeader := func(changeFns ...func(*ResourceHeader)) *ResourceHeader {
		header := &ResourceHeader{
			Kind:    "kind",
			SubKind: "subkind",
			Version: "v1",
			Metadata: Metadata{
				Name:        "name",
				Namespace:   "namespace",
				Description: "description",
				Labels:      map[string]string{"label1": "value1"},
				Expires:     &time.Time{},
				Revision:    "aaaa",
			},
		}

		for _, fn := range changeFns {
			fn(header)
		}

		return header
	}
	tests := []struct {
		name     string
		h1       *ResourceHeader
		h2       *ResourceHeader
		expected bool
	}{
		{
			name:     "empty equals",
			h1:       &ResourceHeader{},
			h2:       &ResourceHeader{},
			expected: true,
		},
		{
			name:     "nil equals",
			h1:       nil,
			h2:       (*ResourceHeader)(nil),
			expected: true,
		},
		{
			name:     "one is nil",
			h1:       &ResourceHeader{},
			h2:       (*ResourceHeader)(nil),
			expected: false,
		},
		{
			name:     "populated equals",
			h1:       newHeader(),
			h2:       newHeader(),
			expected: true,
		},
		{
			name: "kind is different",
			h1:   newHeader(),
			h2: newHeader(func(h *ResourceHeader) {
				h.Kind = "different-kind"
			}),
			expected: false,
		},
		{
			name: "subkind is different",
			h1:   newHeader(),
			h2: newHeader(func(h *ResourceHeader) {
				h.SubKind = "different-subkind"
			}),
			expected: false,
		},
		{
			name: "metadata is different",
			h1:   newHeader(),
			h2: newHeader(func(h *ResourceHeader) {
				h.Metadata = Metadata{}
			}),
			expected: false,
		},
		{
			name: "version is different",
			h1:   newHeader(),
			h2: newHeader(func(h *ResourceHeader) {
				h.Version = "different-version"
			}),
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, test.h1.IsEqual(test.h2))
		})
	}
}

func TestResourceNames(t *testing.T) {
	var apps Apps
	var expectedNames []string
	for i := 0; i < 10; i++ {
		app, err := NewAppV3(Metadata{
			Name: fmt.Sprintf("app-%d", i),
		}, AppSpecV3{
			URI: "tcp://localhost:1111",
		})
		require.NoError(t, err)
		apps = append(apps, app)
		expectedNames = append(expectedNames, app.GetName())
	}

	require.Equal(t, expectedNames, slices.Collect(ResourceNames(apps)))
}

func newAppServer(t *testing.T, name string) AppServer {
	t.Helper()
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
