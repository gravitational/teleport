/*
Copyright 2021 Gravitational, Inc.

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

package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// TestApplicationUnmarshal verifies an app resource can be unmarshaled.
func TestApplicationUnmarshal(t *testing.T) {
	expected, err := types.NewAppV3(types.Metadata{
		Name:        "test-app",
		Description: "Test description",
		Labels:      map[string]string{"env": "dev"},
	}, types.AppSpecV3{
		URI: "http://localhost:8080",
	})
	require.NoError(t, err)
	data, err := utils.ToJSON([]byte(appYAML))
	require.NoError(t, err)
	actual, err := UnmarshalApp(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

// TestApplicationMarshal verifies a marshaled app resource can be unmarshaled back.
func TestApplicationMarshal(t *testing.T) {
	expected, err := types.NewAppV3(types.Metadata{
		Name:        "test-app",
		Description: "Test description",
		Labels:      map[string]string{"env": "dev"},
	}, types.AppSpecV3{
		URI: "http://localhost:8080",
	})
	require.NoError(t, err)
	data, err := MarshalApp(expected)
	require.NoError(t, err)
	actual, err := UnmarshalApp(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestCompareAppServers(t *testing.T) {
	tests := []struct {
		name      string
		appServer types.AppServer
		want      bool
	}{
		{
			name:      "equal",
			appServer: appServerWithModification(nil),
			want:      true,
		},
		{
			name:      "kind",
			appServer: appServerWithModification(func(as *types.AppServerV3) { as.Kind = "diff" }),
		},
		{
			name:      "subkind",
			appServer: appServerWithModification(func(as *types.AppServerV3) { as.SetSubKind("diff") }),
		},
		{
			name:      "version",
			appServer: appServerWithModification(func(as *types.AppServerV3) { as.Version = "diff" }),
		},
		{
			name:      "spec-version",
			appServer: appServerWithModification(func(as *types.AppServerV3) { as.Spec.Version = "diff" }),
		},
		{
			name:      "host-id",
			appServer: appServerWithModification(func(as *types.AppServerV3) { as.Spec.HostID = "diff" }),
		},
		{
			name:      "hostname",
			appServer: appServerWithModification(func(as *types.AppServerV3) { as.Spec.Hostname = "diff" }),
		},
		{
			name:      "proxy ids",
			appServer: appServerWithModification(func(as *types.AppServerV3) { as.Spec.ProxyIDs = []string{"diff"} }),
		},
		{
			name:      "rotation",
			appServer: appServerWithModification(func(as *types.AppServerV3) { as.Spec.Rotation = types.Rotation{State: "diff"} }),
		},
		{
			name: "app",
			appServer: appServerWithModification(func(as *types.AppServerV3) {
				as.Spec.App = appWithModification(func(a *types.AppV3) { a.Kind = "diff" })
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			appServer := appServerWithModification(nil)
			require.Equal(t, test.want, CompareAppServers(appServer, test.appServer))
		})
	}
}

func TestCompareApps(t *testing.T) {
	tests := []struct {
		name string
		app1 types.Application
		app2 types.Application
		want bool
	}{
		{
			name: "equal",
			app1: appWithModification(nil),
			app2: appWithModification(nil),
			want: true,
		},
		{
			name: "kind",
			app1: appWithModification(nil),
			app2: appWithModification(func(a *types.AppV3) { a.Kind = "diff" }),
		},
		{
			name: "subkind",
			app1: appWithModification(nil),
			app2: appWithModification(func(a *types.AppV3) { a.SetSubKind("diff") }),
		},
		{
			name: "version",
			app1: appWithModification(nil),
			app2: appWithModification(func(a *types.AppV3) { a.Version = "diff" }),
		},
		{
			name: "metadata",
			app1: appWithModification(nil),
			app2: appWithModification(func(a *types.AppV3) { a.SetName("diff") }),
		},
		{
			name: "uri",
			app1: appWithModification(nil),
			app2: appWithModification(func(a *types.AppV3) { a.SetURI("diff") }),
		},
		{
			name: "public addr",
			app1: appWithModification(nil),
			app2: appWithModification(func(a *types.AppV3) { a.Spec.PublicAddr = "diff" }),
		},
		{
			name: "dynamic labels",
			app1: appWithModification(nil),
			app2: appWithModification(func(a *types.AppV3) {
				a.SetDynamicLabels(map[string]types.CommandLabel{
					"diff": &types.CommandLabelV2{Result: "diff"},
				})
			}),
		},
		{
			name: "insecure skip verify",
			app1: appWithModification(nil),
			app2: appWithModification(func(a *types.AppV3) { a.Spec.InsecureSkipVerify = false }),
		},
		{
			name: "rewrite",
			app1: appWithModification(nil),
			app2: appWithModification(func(a *types.AppV3) {
				a.Spec.Rewrite = &types.Rewrite{
					Headers: []*types.Header{
						{
							Name:  "diff",
							Value: "diff",
						},
					},
				}
			}),
		},
		{
			name: "aws account ID",
			app1: appWithModification(nil),
			app2: appWithModification(func(a *types.AppV3) { a.Metadata.Labels[constants.AWSAccountIDLabel] = "123456" }),
		},
		{
			name: "aws external ID",
			app1: appWithModification(nil),
			app2: appWithModification(func(a *types.AppV3) { a.Spec.AWS.ExternalID = "diff" }),
		},
		{
			name: "gcp",
			app1: appWithModification(func(a *types.AppV3) { a.Spec.URI = "diff"; a.Spec.Cloud = types.CloudGCP }),
			app2: appWithModification(func(a *types.AppV3) { a.Spec.URI = "diff"; a.Spec.Cloud = types.CloudAzure }),
		},
		{
			name: "tcp",
			app1: appWithModification(nil),
			app2: appWithModification(func(a *types.AppV3) { a.Spec.URI = "tcp://blah" }),
		},
		{
			name: "protocol",
			app1: appWithModification(nil),
			app2: appWithModification(func(a *types.AppV3) { a.Spec.URI = "https://blah" }),
		},
		{
			name: "azure",
			app1: appWithModification(func(a *types.AppV3) { a.Spec.URI = "diff"; a.Spec.Cloud = types.CloudAzure }),
			app2: appWithModification(func(a *types.AppV3) { a.Spec.URI = "diff"; a.Spec.Cloud = types.CloudGCP }),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, CompareApps(test.app1, test.app2))
		})
	}
}

// appServerWithModification returns an app server with modifications performed by the modFn function.
func appServerWithModification(modFn func(*types.AppServerV3)) types.AppServer {
	appServer := &types.AppServerV3{
		Kind:     "kind",
		SubKind:  "subkind",
		Version:  "version",
		Metadata: metadataWithModification(nil),
		Spec: types.AppServerSpecV3{
			Version:  "version",
			Hostname: "hostname",
			Rotation: types.Rotation{
				State: "state",
			},
			App:      appWithModification(nil),
			ProxyIDs: []string{"proxy-id"},
		},
	}

	if modFn != nil {
		modFn(appServer)
	}

	return appServer
}

// appWithModification returns an application with modifications performed by the modFn function.
func appWithModification(modFn func(*types.AppV3)) *types.AppV3 {
	app := &types.AppV3{
		Kind:     "kind",
		SubKind:  "subkind",
		Version:  "version",
		Metadata: metadataWithModification(nil),
		Spec: types.AppSpecV3{
			URI:        constants.AWSConsoleURL,
			PublicAddr: "public-addr",
			DynamicLabels: map[string]types.CommandLabelV2{
				"cmd1": {
					Period:  types.Duration(time.Second),
					Command: []string{"command", "parts"},
					Result:  "result",
				},
			},
			InsecureSkipVerify: true,
			Rewrite: &types.Rewrite{
				Headers: []*types.Header{
					{
						Name:  "name",
						Value: "value",
					},
				},
			},
			AWS: &types.AppAWS{
				ExternalID: "external-id",
			},
			Cloud: types.CloudAWS,
		},
	}

	if modFn != nil {
		modFn(app)
	}

	return app
}

var appYAML = `kind: app
version: v3
metadata:
  name: test-app
  description: "Test description"
  labels:
    env: dev
spec:
  uri: "http://localhost:8080"`
