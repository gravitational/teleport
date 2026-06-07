// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package kubeagent

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/lib/config"
	kubeserver "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
)

func TestManifestsReleaseSecretInteropShape(t *testing.T) {
	version, err := semver.NewVersion("19.0.0")
	require.NoError(t, err)
	opts := ChartOptions{
		Namespace:        "teleport-agent",
		ProxyAddr:        "proxy.example.com:443",
		AuthToken:        "join-token",
		KubeClusterName:  "cluster-name",
		Roles:            RoleKubeAppDiscovery,
		Enterprise:       true,
		Updater:          true,
		UpdaterChannel:   "stable/cloud",
		HighAvailability: true,
		RequestedVersion: version,
		Labels: map[string]string{
			"env": "prod",
		},
	}

	objs, err := Manifests(opts)
	require.NoError(t, err)

	release := decodeReleaseSecret(t, findReleaseSecret(t, objs))
	require.Len(t, release.Hooks, 4)
	require.Equal(t, map[string]any{"env": "prod"}, release.Config["labels"])

	var chart map[string]any
	require.NoError(t, json.Unmarshal(release.Chart, &chart))
	require.Contains(t, chart, "metadata")
	require.Contains(t, chart, "values")
	require.NotContains(t, chart, "templates")
	require.NotContains(t, chart, "files")
}

func TestManifestsReleaseSecret(t *testing.T) {
	version, err := semver.NewVersion("19.0.0")
	require.NoError(t, err)

	tests := []struct {
		name string
		opts ChartOptions
		want map[string]any
	}{
		{
			name: "oss updater disabled",
			opts: ChartOptions{
				Namespace:        "teleport-agent",
				ProxyAddr:        "proxy.example.com:443",
				AuthToken:        "join-token",
				KubeClusterName:  "cluster-name",
				Roles:            RoleKubeAppDiscovery,
				UpdaterChannel:   "stable/cloud",
				RequestedVersion: version,
				Labels:           map[string]string{"env": "prod"},
			},
			want: map[string]any{
				"proxyAddr":       "proxy.example.com:443",
				"roles":           string(RoleKubeAppDiscovery),
				"authToken":       "join-token",
				"kubeClusterName": "cluster-name",
				"labels":          map[string]any{"env": "prod"},
			},
		},
		{
			name: "enterprise updater disabled",
			opts: ChartOptions{
				Namespace:        "teleport-agent",
				ProxyAddr:        "proxy.example.com:443",
				AuthToken:        "join-token",
				KubeClusterName:  "cluster-name",
				Roles:            RoleKubeAppDiscovery,
				UpdaterChannel:   "stable/cloud",
				RequestedVersion: version,
				Labels:           map[string]string{"env": "prod"},
				Enterprise:       true,
			},
			want: map[string]any{
				"proxyAddr":       "proxy.example.com:443",
				"roles":           string(RoleKubeAppDiscovery),
				"authToken":       "join-token",
				"kubeClusterName": "cluster-name",
				"labels":          map[string]any{"env": "prod"},
				"enterprise":      true,
			},
		},
		{
			name: "enterprise updater enabled",
			opts: ChartOptions{
				Namespace:        "teleport-agent",
				ProxyAddr:        "proxy.example.com:443",
				AuthToken:        "join-token",
				KubeClusterName:  "cluster-name",
				Roles:            RoleKubeAppDiscovery,
				UpdaterChannel:   "stable/cloud",
				RequestedVersion: version,
				Labels:           map[string]string{"env": "prod"},
				Enterprise:       true,
				Updater:          true,
				HighAvailability: true,
			},
			want: map[string]any{
				"proxyAddr":       "proxy.example.com:443",
				"roles":           string(RoleKubeAppDiscovery),
				"authToken":       "join-token",
				"kubeClusterName": "cluster-name",
				"labels":          map[string]any{"env": "prod"},
				"enterprise":      true,
				"updater": map[string]any{
					"enabled":        true,
					"releaseChannel": "stable/cloud",
				},
				"highAvailability": map[string]any{
					"replicaCount": float64(2),
					"podDisruptionBudget": map[string]any{
						"enabled":      true,
						"minAvailable": float64(1),
					},
				},
			},
		},
		{
			name: "enterprise updater enabled high availability disabled",
			opts: ChartOptions{
				Namespace:        "teleport-agent",
				ProxyAddr:        "proxy.example.com:443",
				AuthToken:        "join-token",
				KubeClusterName:  "cluster-name",
				Roles:            RoleKubeAppDiscovery,
				UpdaterChannel:   "stable/cloud",
				RequestedVersion: version,
				Labels:           map[string]string{"env": "prod"},
				Enterprise:       true,
				Updater:          true,
			},
			want: map[string]any{
				"proxyAddr":       "proxy.example.com:443",
				"roles":           string(RoleKubeAppDiscovery),
				"authToken":       "join-token",
				"kubeClusterName": "cluster-name",
				"labels":          map[string]any{"env": "prod"},
				"enterprise":      true,
				"updater": map[string]any{
					"enabled":        true,
					"releaseChannel": "stable/cloud",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs, err := Manifests(tt.opts)
			require.NoError(t, err)

			release := decodeReleaseSecret(t, findReleaseSecret(t, objs))
			require.Equal(t, tt.want, release.Config)
			require.NotContains(t, release.Config, "podSecurityPolicy")
		})
	}
}

func TestManifestsToggles(t *testing.T) {
	tests := []struct {
		name     string
		options  ChartOptions
		contains []string
	}{
		{
			name: "enterprise image substitution",
			options: ChartOptions{
				Namespace:       "teleport-agent",
				ProxyAddr:       "proxy.example.com:443",
				AuthToken:       "join-token",
				KubeClusterName: "cluster-name",
				Roles:           RoleKubeAppDiscovery,
				Enterprise:      true,
				Updater:         true,
				UpdaterChannel:  "stable/cloud",
				RequestedVersion: &semver.Version{
					Major: 18,
					Minor: 1,
					Patch: 2,
				},
			},
			contains: []string{
				"public.ecr.aws/gravitational/teleport-ent-distroless:18.1.2",
				"public.ecr.aws/gravitational/teleport-kube-agent-updater:18.1.2",
				"--base-image=public.ecr.aws/gravitational/teleport-ent-distroless",
			},
		},
		{
			name: "prerelease image substitution",
			options: ChartOptions{
				Namespace:       "teleport-agent",
				ProxyAddr:       "proxy.example.com:443",
				AuthToken:       "join-token",
				KubeClusterName: "cluster-name",
				Roles:           RoleKubeAppDiscovery,
				Enterprise:      true,
				Updater:         true,
				UpdaterChannel:  "stable/cloud",
				RequestedVersion: &semver.Version{
					Major:      18,
					Minor:      1,
					Patch:      2,
					PreRelease: "alpha.2",
				},
			},
			contains: []string{
				"public.ecr.aws/gravitational-staging/teleport-ent-distroless:18.1.2-alpha.2",
				"public.ecr.aws/gravitational-staging/teleport-kube-agent-updater:18.1.2-alpha.2",
				"--base-image=public.ecr.aws/gravitational-staging/teleport-ent-distroless",
			},
		},
		{
			name: "oss image substitution",
			options: ChartOptions{
				Namespace:       "teleport-agent",
				ProxyAddr:       "proxy.example.com:443",
				AuthToken:       "join-token",
				KubeClusterName: "cluster-name",
				Roles:           RoleKube,
				RequestedVersion: &semver.Version{
					Major:      18,
					Minor:      1,
					Patch:      2,
					PreRelease: "alpha.2",
				},
			},
			contains: []string{
				"public.ecr.aws/gravitational-staging/teleport-distroless:18.1.2-alpha.2",
			},
		},
		{
			name: "oss and updater image substitution",
			options: ChartOptions{
				Namespace:       "teleport-agent",
				ProxyAddr:       "proxy.example.com:443",
				AuthToken:       "join-token",
				KubeClusterName: "cluster-name",
				Roles:           RoleKube,
				RequestedVersion: &semver.Version{
					Major: 18,
					Minor: 1,
					Patch: 2,
				},
				UpdaterChannel: "stable/cloud",
				Updater:        true,
			},
			contains: []string{
				"public.ecr.aws/gravitational/teleport-distroless:18.1.2",
				"--base-image=public.ecr.aws/gravitational/teleport-distroless",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			objs, err := Manifests(test.options)
			require.NoError(t, err)

			var sb strings.Builder
			for _, obj := range objs {
				u, ok := obj.(*unstructured.Unstructured)
				if !ok {
					continue
				}

				if u.GetKind() == "Deployment" {
					raw, err := json.Marshal(u)
					require.NoError(t, err)
					sb.Write(raw)
					sb.WriteString("\n")
				}

				if u.GetKind() == "StatefulSet" {
					raw, err := json.Marshal(u)
					require.NoError(t, err)
					sb.Write(raw)
					sb.WriteString("\n")
				}
			}

			raw := sb.String()
			assert.NotContains(t, raw, "teleport-ent-distroless:"+api.Version)

			for _, contains := range test.contains {
				assert.Contains(t, raw, contains, "%s not found", contains)
			}
		})
	}
}

func TestBuildHooks(t *testing.T) {
	tests := []struct {
		name         string
		enterprise   bool
		version      *semver.Version
		wantImage    string
		notWantImage string
	}{
		{
			name:       "oss hooks",
			enterprise: false,
			version: &semver.Version{
				Major: 18,
				Minor: 1,
				Patch: 2,
			},
			wantImage:    "public.ecr.aws/gravitational/teleport-distroless:18.1.2",
			notWantImage: "teleport-ent-distroless",
		},
		{
			name:       "prerelease oss hooks",
			enterprise: false,
			version: &semver.Version{
				Major:      18,
				Minor:      1,
				Patch:      2,
				PreRelease: "alpha.2",
			},
			wantImage:    "public.ecr.aws/gravitational-staging/teleport-distroless:18.1.2-alpha.2",
			notWantImage: "teleport-ent-distroless",
		},
		{
			name:       "enterprise hooks",
			enterprise: true,
			version: &semver.Version{
				Major: 18,
				Minor: 1,
				Patch: 2,
			},
			wantImage:    "public.ecr.aws/gravitational/teleport-ent-distroless:18.1.2",
			notWantImage: "public.ecr.aws/gravitational/teleport-distroless",
		},
		{
			name:       "prerelease enterprise hooks",
			enterprise: true,
			version: &semver.Version{
				Major:      18,
				Minor:      1,
				Patch:      2,
				PreRelease: "alpha.2",
			},
			wantImage:    "public.ecr.aws/gravitational-staging/teleport-ent-distroless:18.1.2-alpha.2",
			notWantImage: "public.ecr.aws/gravitational-staging/teleport-distroless",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hooks, err := buildHooks(ChartOptions{
				Namespace:        "teleport-agent",
				ProxyAddr:        "proxy.example.com:443",
				AuthToken:        "join-token",
				KubeClusterName:  "cluster-name",
				Roles:            RoleKube,
				UpdaterChannel:   "stable/cloud",
				RequestedVersion: test.version,
				Enterprise:       test.enterprise,
			})
			require.NoError(t, err)

			var manifests strings.Builder
			for _, hook := range hooks {
				manifests.WriteString(hook.Manifest)
			}

			raw := manifests.String()
			require.Contains(t, raw, test.wantImage)
			require.NotContains(t, raw, test.notWantImage)
		})
	}
}

func FuzzManifestsChartOptions(f *testing.F) {
	f.Add(
		"teleport-agent",
		"proxy.example.com:443",
		"join-token",
		"cluster-name",
		"stable/cloud",
		"prod",
		"us-west-2",
		true,
		true,
		true,
	)
	f.Add(
		"teleport-agent",
		"https://proxy.example.com:443",
		"join-token.with-special_chars",
		"cluster-name",
		"stable/cloud",
		"dev",
		"value-with-dashes",
		false,
		false,
		false,
	)

	f.Fuzz(func(
		t *testing.T,
		namespace string,
		proxyAddr string,
		authToken string,
		clusterName string,
		updaterChannel string,
		labelValue string,
		secondLabelValue string,
		enterprise bool,
		updater bool,
		highAvailability bool,
	) {
		opts := ChartOptions{
			Namespace:        namespace,
			ProxyAddr:        proxyAddr,
			AuthToken:        authToken,
			KubeClusterName:  clusterName,
			Roles:            RoleKubeAppDiscovery,
			Enterprise:       enterprise,
			Updater:          updater,
			UpdaterChannel:   updaterChannel,
			HighAvailability: highAvailability,
			RequestedVersion: &semver.Version{
				Major:      19,
				Minor:      1,
				Patch:      2,
				PreRelease: "alpha.1",
			},
			Labels: map[string]string{
				"env":    labelValue,
				"region": secondLabelValue,
			},
		}

		if err := opts.validate(); err != nil {
			t.Skip()
		}

		objs, err := Manifests(opts)
		require.NoError(t, err)
		require.NotEmpty(t, objs)

		rawManifest := objectsAsYAML(t, objs)
		for _, placeholder := range []string{
			placeholderNamespace,
			placeholderProxy,
			placeholderToken,
			placeholderCluster,
			placeholderChannel,
		} {
			require.NotContains(t, rawManifest, placeholder)
		}

		release := decodeReleaseSecret(t, findReleaseSecret(t, objs))
		require.Equal(t, namespace, release.Namespace)
		require.Equal(t, proxyAddr, release.Config["proxyAddr"])
		require.Equal(t, authToken, release.Config["authToken"])
		require.Equal(t, clusterName, release.Config["kubeClusterName"])
		require.Equal(t, string(RoleKubeAppDiscovery), release.Config["roles"])
		require.Equal(t, map[string]any{
			"env":    labelValue,
			"region": secondLabelValue,
		}, release.Config["labels"])

		cfg := readRenderedTeleportConfig(t, objs)
		require.True(t, cfg.Kube.Enabled())
		require.Equal(t, opts.Labels, cfg.Kube.StaticLabels)
	})
}

func TestPatchTeleportConfig(t *testing.T) {
	tests := []struct {
		name                  string
		initialTeleportConfig string
		labels                map[string]string
		expectedErrorMessage  string
	}{
		{
			name: "labels added to kubernetes service",
			initialTeleportConfig: `
teleport:
  auth_token: token-placeholder
kubernetes_service:
  enabled: true
`,
			labels: map[string]string{"env": "prod"},
		},
		{
			name:                 "missing kubernetes service",
			expectedErrorMessage: "teleport.yaml is missing kubernetes_service",
			initialTeleportConfig: `
teleport:
  auth_token: token-placeholder
`,
			labels: map[string]string{"env": "prod"},
		},
		{
			name: "no labels to patch",
			initialTeleportConfig: `
teleport:
  auth_token: token-placeholder
kubernetes_service:
  enabled: true`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			obj := teleportConfigMapObject(test.initialTeleportConfig)
			err := patchTeleportConfig(obj, ChartOptions{Labels: test.labels})
			if test.expectedErrorMessage != "" {
				require.ErrorContains(t, err, test.expectedErrorMessage)
				return
			}

			data, found, err := unstructured.NestedStringMap(obj.Object, "data")
			require.NoError(t, err)
			require.True(t, found)

			cfg, err := config.ReadConfig(strings.NewReader(data["teleport.yaml"]))
			require.NoError(t, err)

			require.True(t, cfg.Kube.Enabled())
			require.Equal(t, test.labels, cfg.Kube.StaticLabels)
		})
	}
}

func TestApply(t *testing.T) {
	clientGetter := newTestClientGetter(t)
	opts := ChartOptions{
		Namespace:       "teleport-agent",
		ProxyAddr:       "proxy.example.com:443",
		AuthToken:       "join-token",
		KubeClusterName: "cluster-name",
		Roles:           RoleKube,
		RequestedVersion: &semver.Version{
			Major: 19,
			Minor: 1,
			Patch: 2,
		},
	}

	installed, err := IsInstalled(t.Context(), clientGetter, opts.Namespace)
	require.NoError(t, err)
	require.False(t, installed)

	require.NoError(t, Apply(t.Context(), clientGetter, opts))

	installed, err = IsInstalled(t.Context(), clientGetter, opts.Namespace)
	require.NoError(t, err)
	require.True(t, installed)
}

func TestApplyNamespaceAlreadyExists(t *testing.T) {
	clientGetter := newTestClientGetter(t)
	opts := ChartOptions{
		Namespace:       "teleport-agent",
		ProxyAddr:       "proxy.example.com:443",
		AuthToken:       "join-token",
		KubeClusterName: "cluster-name",
		Roles:           RoleKube,
		RequestedVersion: &semver.Version{
			Major: 19,
			Minor: 1,
			Patch: 2,
		},
	}

	restConfig, err := clientGetter.ToRESTConfig()
	require.NoError(t, err)

	clt, err := client.New(restConfig, client.Options{Scheme: scheme.Scheme})
	require.NoError(t, err)

	require.NoError(t, clt.Create(t.Context(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: opts.Namespace}}))

	require.NoError(t, Apply(t.Context(), clientGetter, opts))

	installed, err := IsInstalled(t.Context(), clientGetter, opts.Namespace)
	require.NoError(t, err)
	require.True(t, installed)
}

func TestApplyFails(t *testing.T) {
	clientGetter := newTestClientGetter(t)
	opts := ChartOptions{
		Namespace:       "teleport-agent",
		ProxyAddr:       "proxy.example.com:443",
		AuthToken:       "join-token",
		KubeClusterName: "cluster-name",
		Roles:           RoleKube,
		RequestedVersion: &semver.Version{
			Major: 19,
			Minor: 1,
			Patch: 2,
		},
	}

	restConfig, err := clientGetter.ToRESTConfig()
	require.NoError(t, err)

	clt, err := client.New(restConfig, client.Options{Scheme: scheme.Scheme})
	require.NoError(t, err)

	objs, err := Manifests(opts)
	require.NoError(t, err)

	var found bool
	for _, obj := range objs {
		u, ok := obj.(*unstructured.Unstructured)
		if ok {
			existing := u.DeepCopy()
			existing.SetLabels(nil)
			existing.SetAnnotations(nil)
			require.NoError(t, clt.Create(t.Context(), existing))
			found = true
		}
	}
	require.True(t, found)

	require.Error(t, Apply(t.Context(), clientGetter, opts))

	installed, err := IsInstalled(t.Context(), clientGetter, opts.Namespace)
	require.NoError(t, err)
	require.False(t, installed)
}

func TestApplyIgnoresExistingResource(t *testing.T) {
	clientGetter := newTestClientGetter(t)
	opts := ChartOptions{
		Namespace:       "teleport-agent",
		ProxyAddr:       "proxy.example.com:443",
		AuthToken:       "join-token",
		KubeClusterName: "cluster-name",
		Roles:           RoleKube,
		RequestedVersion: &semver.Version{
			Major: 19,
			Minor: 1,
			Patch: 2,
		},
	}

	restConfig, err := clientGetter.ToRESTConfig()
	require.NoError(t, err)
	clt, err := client.New(restConfig, client.Options{Scheme: scheme.Scheme})
	require.NoError(t, err)

	objs, err := Manifests(opts)
	require.NoError(t, err)

	var precreated *unstructured.Unstructured
	for _, obj := range objs {
		if u, ok := obj.(*unstructured.Unstructured); ok {
			precreated = u.DeepCopy()
			break
		}
	}
	require.NotNil(t, precreated)

	labels := precreated.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels["partial-install/stale"] = "true"
	precreated.SetLabels(labels)
	require.NoError(t, clt.Create(t.Context(), precreated))

	require.NoError(t, Apply(t.Context(), clientGetter, opts))

	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(precreated.GroupVersionKind())
	require.NoError(t, clt.Get(t.Context(), client.ObjectKeyFromObject(precreated), got))
	require.NotContains(t, got.GetLabels(), "partial-install/stale")

	installed, err := IsInstalled(t.Context(), clientGetter, opts.Namespace)
	require.NoError(t, err)
	require.True(t, installed)
}

func TestIsInstalled(t *testing.T) {
	tests := []struct {
		name   string
		secret *corev1.Secret
		want   bool
	}{
		{
			name: "deployed helm release",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "teleport-agent",
					Name:      "release",
					Labels: map[string]string{
						"owner": "helm", "name": helmReleaseName, "status": helmReleaseStatus,
					},
				},
			},
			want: true,
		},
		{
			name: "failed release is ignored",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "teleport-agent",
					Name:      "release",
					Labels: map[string]string{
						"owner": "helm", "name": helmReleaseName, "status": "failed",
					},
				},
			},
		},
		{
			name: "non helm secret is ignored",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "teleport-agent",
					Name:      "release",
					Labels: map[string]string{
						"name": helmReleaseName, "status": helmReleaseStatus,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getter := newTestClientGetter(t)

			restConfig, err := getter.ToRESTConfig()
			require.NoError(t, err)

			clt, err := client.New(restConfig, client.Options{Scheme: scheme.Scheme})
			require.NoError(t, err)

			require.NoError(t, clt.Create(t.Context(), tt.secret))

			got, err := IsInstalled(t.Context(), getter, "teleport-agent")
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestApplyReconcilesPartialInstall(t *testing.T) {
	clientGetter := newTestClientGetter(t)
	restConfig, err := clientGetter.ToRESTConfig()
	require.NoError(t, err)
	clt, err := client.New(restConfig, client.Options{Scheme: scheme.Scheme})
	require.NoError(t, err)

	initial := ChartOptions{
		Namespace:       "teleport-agent",
		ProxyAddr:       "proxy.example.com:443",
		AuthToken:       "old-token",
		KubeClusterName: "cluster-name",
		Roles:           RoleKube,
		RequestedVersion: &semver.Version{
			Major: 19,
			Minor: 1,
			Patch: 2,
		},
	}

	// Simulate a partial install: every rendered chart object is in the
	// cluster, but the helm release Secret was never written.
	objs, err := Manifests(initial)
	require.NoError(t, err)
	for _, obj := range objs {
		if s, ok := obj.(*corev1.Secret); ok && s.Type == helmSecretType {
			continue
		}
		require.NoError(t, clt.Create(t.Context(), obj))
	}

	installed, err := IsInstalled(t.Context(), clientGetter, initial.Namespace)
	require.NoError(t, err)
	require.False(t, installed)

	retry := initial
	retry.AuthToken = "new-token"
	require.NoError(t, Apply(t.Context(), clientGetter, retry))

	installed, err = IsInstalled(t.Context(), clientGetter, retry.Namespace)
	require.NoError(t, err)
	require.True(t, installed)

	var secrets corev1.SecretList
	require.NoError(t, clt.List(t.Context(), &secrets, client.InNamespace(retry.Namespace)))

	var joinToken *corev1.Secret
	for i := range secrets.Items {
		s := &secrets.Items[i]
		if s.Type != helmSecretType && s.StringData["auth-token"] != "" {
			joinToken = s
			break
		}
	}
	require.NotNil(t, joinToken)
	require.Equal(t, "new-token", strings.TrimSpace(joinToken.StringData["auth-token"]))
}

func TestApplyRejectsForeignJoinTokenSecret(t *testing.T) {
	clientGetter := newTestClientGetter(t)
	restConfig, err := clientGetter.ToRESTConfig()
	require.NoError(t, err)
	clt, err := client.New(restConfig, client.Options{Scheme: scheme.Scheme})
	require.NoError(t, err)

	opts := ChartOptions{
		Namespace:       "teleport-agent",
		ProxyAddr:       "proxy.example.com:443",
		AuthToken:       "join-token",
		KubeClusterName: "cluster-name",
		Roles:           RoleKube,
		RequestedVersion: &semver.Version{
			Major: 19,
			Minor: 1,
			Patch: 2,
		},
	}

	objs, err := Manifests(opts)
	require.NoError(t, err)

	var foreign *unstructured.Unstructured
	for _, obj := range objs {
		u, ok := obj.(*unstructured.Unstructured)
		if !ok {
			continue
		}
		if u.GetKind() == "Secret" {
			foreign = u.DeepCopy()
			break
		}
	}
	require.NotNil(t, foreign)

	foreign.SetLabels(nil)
	foreign.SetAnnotations(nil)
	require.NoError(t, clt.Create(t.Context(), foreign))

	err = Apply(t.Context(), clientGetter, opts)
	require.Error(t, err)
	require.True(t, trace.IsAlreadyExists(err))

	installed, err := IsInstalled(t.Context(), clientGetter, opts.Namespace)
	require.NoError(t, err)
	require.False(t, installed)
}

func teleportConfigMapObject(payload string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name": helmReleaseName,
		},
		"data": map[string]any{
			"teleport.yaml": payload,
		},
	}}
}

func findReleaseSecret(t *testing.T, objs []client.Object) *corev1.Secret {
	t.Helper()

	for _, obj := range objs {
		secret, ok := obj.(*corev1.Secret)
		if ok && secret.Type == helmSecretType {
			return secret
		}
	}

	t.Fatal("release secret not found")
	return nil
}

func decodeReleaseSecret(t *testing.T, secret *corev1.Secret) helmRelease {
	t.Helper()

	compressed, err := base64.StdEncoding.DecodeString(string(secret.Data["release"]))
	require.NoError(t, err)

	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	require.NoError(t, err)
	defer gz.Close()

	raw, err := io.ReadAll(gz)
	require.NoError(t, err)

	var release helmRelease
	require.NoError(t, json.Unmarshal(raw, &release))
	return release
}

func objectsAsYAML(t *testing.T, objs []client.Object) string {
	t.Helper()

	var out strings.Builder
	for _, obj := range objs {
		raw, err := json.Marshal(obj)
		require.NoError(t, err)
		out.Write(raw)
		out.WriteString("\n")
	}
	return out.String()
}

func readRenderedTeleportConfig(t *testing.T, objs []client.Object) *config.FileConfig {
	t.Helper()

	for _, obj := range objs {
		u, ok := obj.(*unstructured.Unstructured)
		if !ok {
			continue
		}
		if u.GetKind() != "ConfigMap" || u.GetName() != helmReleaseName {
			continue
		}

		data, found, err := unstructured.NestedStringMap(u.Object, "data")
		require.NoError(t, err)
		require.True(t, found)

		payload := data["teleport.yaml"]
		require.NotEmpty(t, payload)

		cfg, err := config.ReadConfig(strings.NewReader(payload))
		require.NoError(t, err)
		return cfg
	}

	t.Fatal("teleport config ConfigMap not found")
	return nil
}

func newTestClientGetter(t *testing.T) genericclioptions.RESTClientGetter {
	t.Helper()
	srv, err := kubeserver.NewKubeAPIMock()
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, srv.Close()) })

	cfg := &rest.Config{
		Host: srv.URL,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}
	flags := genericclioptions.NewConfigFlags(false)
	flags.WithWrapConfigFn(func(*rest.Config) *rest.Config {
		return rest.CopyConfig(cfg)
	})
	return flags
}
