/*
Copyright 2026 Gravitational, Inc.

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

package discoveryconfig

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
)

const sentinelJoinToken = "sentinel"

var installerParamsType = reflect.TypeFor[*types.InstallerParams]()

func TestNewStaticSnapshotDiscoveryConfig(t *testing.T) {
	const serverID = "00000000-0000-0000-0000-000000000001"
	input := Spec{DiscoveryGroup: "group", AWS: []types.AWSMatcher{{
		Types: []string{types.AWSMatcherEC2}, Regions: []string{"us-east-1"},
	}}}
	dc, err := NewStaticSnapshotDiscoveryConfig(serverID, input)
	require.NoError(t, err)
	require.True(t, dc.IsStaticSnapshot())
	require.Equal(t, StaticSnapshotName(serverID), dc.GetName())
	require.Equal(t, types.OriginConfigFile, dc.Origin())
	require.Equal(t, "group", dc.GetDiscoveryGroup())
	require.Len(t, dc.Spec.AWS, 1)
	// Validation runs against a throwaway copy, so the stored inventory
	// never gains installer settings.
	require.Nil(t, dc.Spec.AWS[0].Params)

	// The spec is copied on construction: mutating the input afterward must
	// not reach the resource.
	input.AWS[0].Regions[0] = "mutated"
	require.Equal(t, "us-east-1", dc.Spec.AWS[0].Regions[0])

	// Publication is fail-closed: unsanitized installer params are rejected
	// rather than silently stripped.
	_, err = NewStaticSnapshotDiscoveryConfig(serverID, Spec{AWS: []types.AWSMatcher{{
		Types: []string{types.AWSMatcherEC2}, Regions: []string{"us-east-1"},
		Params: &types.InstallerParams{JoinToken: "secret"},
	}}})
	requireBadParameter(t, err)

	// A service with no discovery group and no matchers still publishes a
	// valid (empty) snapshot.
	empty, err := NewStaticSnapshotDiscoveryConfig(serverID, Spec{})
	require.NoError(t, err)
	require.Empty(t, empty.GetDiscoveryGroup())
	require.NoError(t, CheckStaticSnapshotDiscoveryConfig(empty, serverID))

	_, err = NewStaticSnapshotDiscoveryConfig("", Spec{})
	requireBadParameter(t, err)
}

// TestStaticSnapshotPreservesNonInstallerMatcherFields pins the inventory
// contract: snapshot validation must not enrich stored matchers with values
// derived by ordinary matcher defaulting. A publisher sending raw file
// configuration (no ssm document, no installer params) must get exactly that
// stored; deriving the absent document from a sanitized, params-less matcher
// would pick the agentless installer document where the publishing service's
// own defaulting picks the agent one, misrepresenting the effective config.
func TestStaticSnapshotPreservesNonInstallerMatcherFields(t *testing.T) {
	// Snapshot validation of the integration matcher below must not depend
	// on the Auth host's environment: pin EICE explicitly disabled.
	t.Setenv(constants.UnstableEnableEICEEnvVar, "")
	const serverID = "00000000-0000-0000-0000-000000000001"
	input := types.AWSMatcher{
		Types:       []string{types.AWSMatcherEC2},
		Regions:     []string{"us-east-1"},
		Integration: "integration",
	}

	dc, err := NewStaticSnapshotDiscoveryConfig(serverID, Spec{AWS: []types.AWSMatcher{input}})
	require.NoError(t, err)
	require.Nil(t, dc.Spec.AWS[0].SSM, "an absent SSM document must stay absent, not be derived")
	require.Equal(t, input, dc.Spec.AWS[0], "snapshot construction must store the matcher exactly as published")

	// A publisher-supplied document name passes through unchanged.
	custom := input
	custom.SSM = &types.AWSSSM{DocumentName: "custom-installer-document"}
	dc, err = NewStaticSnapshotDiscoveryConfig(serverID, Spec{AWS: []types.AWSMatcher{custom}})
	require.NoError(t, err)
	require.Equal(t, custom, dc.Spec.AWS[0])

	// Re-running validation on the stored form (backend reads, the marshal
	// path) must leave it unchanged as well.
	require.NoError(t, dc.CheckAndSetDefaults())
	require.Equal(t, custom, dc.Spec.AWS[0])

	// Validation still runs against the throwaway copy: an invalid matcher
	// is rejected even though nothing is mutated.
	_, err = NewStaticSnapshotDiscoveryConfig(serverID, Spec{AWS: []types.AWSMatcher{{
		Types:   []string{"not-a-matcher-type"},
		Regions: []string{"us-east-1"},
	}}})
	requireBadParameter(t, err)

	// Azure, GCP, and Kube matchers get the same treatment: validation must
	// not stamp the wildcard scoping their defaulting derives (regions,
	// resource groups, locations, namespaces, labels) into the inventory.
	multi := Spec{
		Azure: []types.AzureMatcher{{Types: []string{types.AzureMatcherVM}, Subscriptions: []string{"sub-1"}}},
		GCP:   []types.GCPMatcher{{Types: []string{types.GCPMatcherCompute}, ProjectIDs: []string{"project-1"}}},
		Kube:  []types.KubernetesMatcher{{Types: []string{types.KubernetesMatchersApp}}},
	}
	dcMulti, err := NewStaticSnapshotDiscoveryConfig(serverID, multi)
	require.NoError(t, err)
	require.Empty(t, dcMulti.Spec.Azure[0].Regions, "absent Azure regions must stay absent, not default to the wildcard")
	require.Empty(t, dcMulti.Spec.Azure[0].ResourceGroups)
	require.Empty(t, dcMulti.Spec.Azure[0].ResourceTags)
	require.Equal(t, []string{"sub-1"}, dcMulti.Spec.Azure[0].Subscriptions)
	require.Nil(t, dcMulti.Spec.Azure[0].Params)
	require.Empty(t, dcMulti.Spec.GCP[0].Locations, "absent GCP locations must stay absent, not default to the wildcard")
	require.Empty(t, dcMulti.Spec.GCP[0].Labels)
	require.Nil(t, dcMulti.Spec.GCP[0].Params)
	require.Empty(t, dcMulti.Spec.Kube[0].Namespaces, "absent Kube namespaces must stay absent, not default to the wildcard")
	require.Empty(t, dcMulti.Spec.Kube[0].Labels)
}

// TestStaticSnapshotMatchesRegularDefaulting pins the publisher contract end
// to end: a matcher that went through ordinary file-config defaulting (as
// the running service applies it) and was sanitized of installer params is
// stored by snapshot construction exactly as sent. Every non-installer field
// the service derived, especially SSM.DocumentName, is what the snapshot
// reports; the bug this guards against stored AWSAgentlessInstallerDocument
// where the service runs with AWSInstallerDocument.
func TestStaticSnapshotMatchesRegularDefaulting(t *testing.T) {
	t.Setenv(constants.UnstableEnableEICEEnvVar, "true")
	const serverID = "00000000-0000-0000-0000-000000000001"

	tests := map[string]struct {
		matcher    types.AWSMatcher
		wantSSMDoc string
	}{
		"default EC2 script enrollment": {
			matcher: types.AWSMatcher{
				Types: []string{types.AWSMatcherEC2}, Regions: []string{"us-east-1"},
			},
			wantSSMDoc: types.AWSInstallerDocument,
		},
		"integration-backed EC2": {
			matcher: types.AWSMatcher{
				Types: []string{types.AWSMatcherEC2}, Regions: []string{"us-east-1"},
				Integration: "integration",
			},
			wantSSMDoc: types.AWSInstallerDocument,
		},
		"custom SSM document": {
			matcher: types.AWSMatcher{
				Types: []string{types.AWSMatcherEC2}, Regions: []string{"us-east-1"},
				SSM: &types.AWSSSM{DocumentName: "custom-installer-document"},
			},
			wantSSMDoc: "custom-installer-document",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Ordinary file-config defaulting, then the publisher-side
			// sanitization that strips installer params before publication.
			spec := Spec{DiscoveryGroup: "group", AWS: []types.AWSMatcher{tc.matcher}}
			require.NoError(t, spec.AWS[0].CheckAndSetDefaults())
			SanitizeStaticSnapshotSpec(&spec)

			var sourceBefore types.AWSMatcher
			require.NoError(t, utils.StrictObjectToStruct(&spec.AWS[0], &sourceBefore))

			dc, err := NewStaticSnapshotDiscoveryConfig(serverID, spec)
			require.NoError(t, err)
			stored := dc.Spec.AWS[0]
			require.Nil(t, stored.Params, "installer params must be absent from the snapshot")
			require.Equal(t, sourceBefore, stored,
				"the snapshot must store the service's effective matcher exactly")
			require.NotNil(t, stored.SSM)
			require.Equal(t, tc.wantSSMDoc, stored.SSM.DocumentName,
				"the snapshot must report the SSM document the service actually runs with")
			require.Equal(t, sourceBefore, spec.AWS[0],
				"construction must not modify the caller's spec")
		})
	}
}

func TestCheckStaticSnapshotDiscoveryConfig(t *testing.T) {
	const serverID = "00000000-0000-0000-0000-000000000001"
	snapshot, err := NewStaticSnapshotDiscoveryConfig(serverID, Spec{DiscoveryGroup: "group"})
	require.NoError(t, err)
	require.NoError(t, CheckStaticSnapshotDiscoveryConfig(snapshot, serverID))

	for name, mutate := range map[string]func(*DiscoveryConfig){
		"wrong subkind": func(dc *DiscoveryConfig) { dc.SetSubKind("") },
		"foreign name":  func(dc *DiscoveryConfig) { dc.SetName(StaticSnapshotName("00000000-0000-0000-0000-000000000002")) },
		"wrong origin":  func(dc *DiscoveryConfig) { dc.SetOrigin(types.OriginDynamic) },
		"installer params": func(dc *DiscoveryConfig) {
			dc.Spec.AWS = []types.AWSMatcher{{Params: &types.InstallerParams{JoinToken: "secret"}}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			dc := snapshot.Clone()
			mutate(dc)
			requireBadParameter(t, CheckStaticSnapshotDiscoveryConfig(dc, serverID))
		})
	}

	requireBadParameter(t, CheckStaticSnapshotDiscoveryConfig(snapshot, ""))
	requireBadParameter(t, CheckStaticSnapshotDiscoveryConfig(nil, serverID))
}

// TestEachInstallerParamsCoversAllFamilies enforces the eachInstallerParams
// contract by reflection: every Spec matcher family whose elements carry
// installer params must be visited, so static snapshot sanitization and
// validation cannot silently miss a family added later.
func TestEachInstallerParamsCoversAllFamilies(t *testing.T) {
	var spec Spec
	want := populateSentinelInstallerParams(&spec)
	require.NotZero(t, want, "expected at least one matcher family with installer params")

	visited := 0
	spec.eachInstallerParams(func(p **types.InstallerParams) {
		if *p != nil && (*p).JoinToken == sentinelJoinToken {
			visited++
		}
	})
	require.Equal(t, want, visited,
		"eachInstallerParams must visit every matcher family carrying installer params; update it for the new family")

	SanitizeStaticSnapshotSpec(&spec)
	require.Empty(t, familiesWithInstallerParams(&spec), "sanitization left installer params behind")
}

func populateSentinelInstallerParams(spec *Spec) int {
	specValue := reflect.ValueOf(spec).Elem()
	specType := specValue.Type()
	populated := 0
	for i := range specType.NumField() {
		field := specType.Field(i)
		params, ok := installerParamsField(field.Type)
		if !ok {
			continue
		}
		entry := newMatcherValue(field.Type)
		matcher := entry
		if matcher.Kind() == reflect.Pointer {
			matcher = matcher.Elem()
		}
		matcher.FieldByIndex(params.Index).Set(reflect.ValueOf(&types.InstallerParams{JoinToken: sentinelJoinToken}))
		specValue.Field(i).Set(reflect.Append(reflect.MakeSlice(field.Type, 0, 1), entry))
		populated++
	}
	return populated
}

func familiesWithInstallerParams(spec *Spec) []string {
	var found []string
	specValue := reflect.ValueOf(spec).Elem()
	specType := specValue.Type()
	for i := range specType.NumField() {
		field := specType.Field(i)
		params, ok := installerParamsField(field.Type)
		if !ok {
			continue
		}
		matchers := specValue.Field(i)
		for j := range matchers.Len() {
			matcher := matchers.Index(j)
			if matcher.Kind() == reflect.Pointer {
				if matcher.IsNil() {
					continue
				}
				matcher = matcher.Elem()
			}
			if !matcher.FieldByIndex(params.Index).IsNil() {
				found = append(found, field.Name)
			}
		}
	}
	return found
}

func installerParamsField(fieldType reflect.Type) (reflect.StructField, bool) {
	if fieldType.Kind() != reflect.Slice {
		return reflect.StructField{}, false
	}
	elementType := fieldType.Elem()
	if elementType.Kind() == reflect.Pointer {
		elementType = elementType.Elem()
	}
	if elementType.Kind() != reflect.Struct {
		return reflect.StructField{}, false
	}
	for field := range elementType.Fields() {
		if field.Type == installerParamsType {
			return field, true
		}
	}
	return reflect.StructField{}, false
}

func newMatcherValue(sliceType reflect.Type) reflect.Value {
	elementType := sliceType.Elem()
	if elementType.Kind() == reflect.Pointer {
		return reflect.New(elementType.Elem())
	}
	return reflect.New(elementType).Elem()
}

func TestStaticSnapshotNames(t *testing.T) {
	uuid := "00000000-0000-0000-0000-000000000001"
	require.Equal(t, "static-snapshot-"+uuid, StaticSnapshotName(uuid))
	require.True(t, IsReservedStaticSnapshotName(StaticSnapshotName(uuid)))
	hashed := StaticSnapshotName("legacy-server")
	require.True(t, strings.HasPrefix(hashed, staticSnapshotHashedNamePrefix))
	require.Equal(t, hashed, StaticSnapshotName("legacy-server"))
	require.True(t, IsReservedStaticSnapshotName(hashed))
	require.False(t, IsReservedStaticSnapshotName("static-snapshot-aws-prod"))
}
