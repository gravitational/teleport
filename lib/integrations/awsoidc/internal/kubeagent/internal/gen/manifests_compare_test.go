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

package main

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/gravitational/teleport/lib/integrations/awsoidc/internal/kubeagent"
)

// TestManifestsMatchesChart renders the helm chart in-process for each
// scenario's Options and asserts the output is equivalent to kubeagent.Manifests(opts).
// Catches drift between the chart's output and the runtime substitutions.
func TestManifestsMatchesChart(t *testing.T) {
	t.Parallel()

	const chartPath = "../../../../../../../examples/chart/teleport-kube-agent"
	ch, err := loader.Load(chartPath)
	require.NoError(t, err)

	tests := []struct {
		name string
		opts kubeagent.Options
	}{
		{
			name: "oss-kube-no-updater",
			opts: kubeagent.Options{
				Roles:           kubeagent.RoleKube,
				Namespace:       "teleport-agent",
				ProxyAddr:       "example.teleport.sh:443",
				AuthToken:       "join-token-value",
				KubeClusterName: "my-eks-cluster",
			},
		},
		{
			name: "ent-kube-no-updater",
			opts: kubeagent.Options{
				Roles:           kubeagent.RoleKube,
				Namespace:       "teleport-agent",
				ProxyAddr:       "example.teleport.sh:443",
				AuthToken:       "join-token-value",
				KubeClusterName: "my-eks-cluster",
				Enterprise:      true,
			},
		},
		{
			name: "ent-kube-updater-ha",
			opts: kubeagent.Options{
				Roles:            kubeagent.RoleKube,
				Namespace:        "teleport-agent",
				ProxyAddr:        "example.teleport.sh:443",
				AuthToken:        "join-token-value",
				KubeClusterName:  "my-eks-cluster",
				Enterprise:       true,
				Updater:          true,
				UpdaterChannel:   "stable/cloud",
				HighAvailability: true,
			},
		},
		{
			name: "ent-discovery-updater-ha",
			opts: kubeagent.Options{
				Roles:            kubeagent.RoleKubeAppDiscovery,
				Namespace:        "teleport-agent",
				ProxyAddr:        "example.teleport.sh:443",
				AuthToken:        "join-token-value",
				KubeClusterName:  "my-eks-cluster",
				Enterprise:       true,
				Updater:          true,
				UpdaterChannel:   "stable/cloud",
				HighAvailability: true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			chartObjs := renderChartForOpts(t, ch, test.opts)
			composerObjs, err := kubeagent.Manifests(test.opts)
			require.NoError(t, err)
			assertSameResources(t, chartObjs, composerObjs)
		})
	}
}

// renderChartForOpts renders the helm chart in-process with values
// derived from opts and returns the decoded objects keyed by Kind and Name.
func renderChartForOpts(t *testing.T, ch *chart.Chart, opts kubeagent.Options) map[resourceID]runtime.Object {
	t.Helper()

	replicas := 1
	if opts.HighAvailability {
		replicas = 2
	}

	vals := chartutil.Values{
		"roles":           string(opts.Roles),
		"proxyAddr":       opts.ProxyAddr,
		"authToken":       opts.AuthToken,
		"kubeClusterName": opts.KubeClusterName,
		"enterprise":      opts.Enterprise,
		"updater": map[string]any{
			"enabled":        opts.Updater,
			"releaseChannel": opts.UpdaterChannel,
		},
		"highAvailability": map[string]any{
			"replicaCount":        replicas,
			"podDisruptionBudget": map[string]any{"enabled": opts.HighAvailability, "minAvailable": 1},
		},
		"skipHooks":         true,
		"podSecurityPolicy": map[string]any{"enabled": false},
	}

	relOpts := chartutil.ReleaseOptions{
		Name:      "teleport-kube-agent",
		Namespace: opts.Namespace,
		Revision:  1,
		IsInstall: true,
	}
	rv, err := chartutil.ToRenderValues(ch, vals, relOpts, chartutil.DefaultCapabilities)
	require.NoError(t, err)

	rendered, err := engine.Render(ch, rv)
	require.NoError(t, err)

	out := map[resourceID]runtime.Object{}
	for path, content := range rendered {
		if strings.TrimSpace(content) == "" ||
			!strings.Contains(path, "/templates/") ||
			!strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			continue
		}
		objs, err := decodeObjects(content)
		require.NoError(t, err)
		for _, obj := range objs {
			if sts, ok := obj.(*appsv1.StatefulSet); ok {
				// Same scrub renderHelmChart does for the gen path.
				// The composer overrides ConfigMap content at runtime,
				// so this hash would always be stale.
				delete(sts.Spec.Template.Annotations, "checksum/config")
			}
			out[idOfObj(obj)] = obj
		}
	}
	return out
}

func idOfObj(obj runtime.Object) resourceID {
	var id resourceID
	if gvks, _, err := scheme.Scheme.ObjectKinds(obj); err == nil && len(gvks) > 0 {
		id.Kind = gvks[0].Kind
	}
	if mo, ok := obj.(interface{ GetName() string }); ok {
		id.Name = mo.GetName()
	}
	return id
}

// assertSameResources fails the test if the chart-rendered and composer-produced
// object sets differ. Both sides are canonicalized to YAML so diffs are readable.
func assertSameResources(t *testing.T, chart map[resourceID]runtime.Object, composer []client.Object) {
	t.Helper()

	composerByID := map[resourceID]runtime.Object{}
	for _, o := range composer {
		id := idOfObj(o)
		// Skip the helm release-storage Secret. It's fabricated to make
		// `helm uninstall`/`upgrade` work. The chart templates don't emit it.
		if id.Kind == "Secret" && strings.HasPrefix(id.Name, "sh.helm.release.") {
			continue
		}

		stripHelmInstallMetadata(o)
		composerByID[id] = o
	}

	chartYAML := canonicalYAMLMap(t, chart)
	composerYAML := canonicalYAMLMap(t, composerByID)

	require.Empty(t, cmp.Diff(chartYAML, composerYAML, cmpopts.SortMaps(func(a, b string) bool { return a < b })))
	for k := range chartYAML {
		require.Equal(t, chartYAML[k], composerYAML[k])
	}
}

// stripHelmInstallMetadata removes the labels and annotations helm itself
// injects at install time. The chart's templates don't emit these, they
// are added by helm during install/upgrade.
func stripHelmInstallMetadata(o client.Object) {
	if labels := o.GetLabels(); labels != nil {
		delete(labels, "app.kubernetes.io/managed-by")
		delete(labels, "helm.sh/chart")
		if len(labels) == 0 {
			o.SetLabels(nil)
		}
	}
	if annotations := o.GetAnnotations(); annotations != nil {
		delete(annotations, "meta.helm.sh/release-name")
		delete(annotations, "meta.helm.sh/release-namespace")
		if len(annotations) == 0 {
			o.SetAnnotations(nil)
		}
	}
}

func canonicalYAMLMap(t *testing.T, objs map[resourceID]runtime.Object) map[string]string {
	t.Helper()
	out := map[string]string{}
	for id, obj := range objs {
		// Strip TypeMeta because the helm-decoder sets it but the composer
		// doesn't, so apiVersion/kind would otherwise diverge.
		obj.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{})
		data, err := sigsyaml.Marshal(obj)
		require.NoError(t, err)
		out[id.Kind+"/"+id.Name] = string(data)
	}
	return out
}
