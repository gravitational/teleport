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
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/gravitational/teleport/lib/config"
)

func baseOpts() Options {
	return Options{
		Namespace:       "teleport-agent",
		ProxyAddr:       "example.teleport.sh:443",
		AuthToken:       "join-token-value",
		KubeClusterName: "my-eks-cluster",
		Roles:           RoleKube,
	}
}

func TestManifests_MinimalOSS(t *testing.T) {
	t.Parallel()

	opts := baseOpts()
	objs, err := Manifests(opts)
	require.NoError(t, err)

	kinds := kindsByType(objs)
	require.Equal(t, 1, kinds["StatefulSet"])
	require.Equal(t, 0, kinds["Deployment"])
	require.Equal(t, 0, kinds["PodDisruptionBudget"])
	require.Equal(t, 1, kinds["ServiceAccount"])
	require.Equal(t, 1, kinds["Role"])
	require.Equal(t, 1, kinds["RoleBinding"])
	require.Equal(t, 1, kinds["ClusterRole"])
	require.Equal(t, 1, kinds["ClusterRoleBinding"])
	require.Equal(t, 1, kinds["ConfigMap"])
	require.Equal(t, 2, kinds["Secret"])

	sts := getObject[*appsv1.StatefulSet](t, objs)
	require.EqualValues(t, 1, *sts.Spec.Replicas)
	require.Contains(t, sts.Spec.Template.Spec.Containers[0].Image, "teleport-distroless:")
	require.NotContains(t, sts.Spec.Template.Spec.Containers[0].Image, "teleport-ent-distroless:")

	c := sts.Spec.Template.Spec.Containers[0]
	for _, e := range c.Env {
		require.NotContains(t, []string{
			"TELEPORT_EXT_UPGRADER",
			"TELEPORT_EXT_UPGRADER_VERSION",
			"TELEPORT_UPDATE_CONFIG_FILE",
		}, e.Name)
	}
	for _, m := range c.VolumeMounts {
		require.NotEqual(t, "updater-config", m.Name)
	}
	for _, v := range sts.Spec.Template.Spec.Volumes {
		require.NotEqual(t, "updater-config", v.Name)
	}

	cm := getObject[*corev1.ConfigMap](t, objs)
	require.Equal(t, baseOpts().Namespace, cm.Namespace)

	cfg, err := config.ReadConfig(strings.NewReader(cm.Data["teleport.yaml"]))
	require.NoError(t, err)

	require.Equal(t, opts.KubeClusterName, cfg.Kube.KubeClusterName)
	require.True(t, cfg.Kube.Enabled())
	require.False(t, cfg.Apps.Enabled())
}

func TestManifests_EnterpriseImage(t *testing.T) {
	t.Parallel()

	opts := baseOpts()
	opts.Enterprise = true

	objs, err := Manifests(opts)
	require.NoError(t, err)

	sts := getObject[*appsv1.StatefulSet](t, objs)
	require.Contains(t, sts.Spec.Template.Spec.Containers[0].Image, "teleport-ent-distroless:")
}

func TestManifests_AgentVersion(t *testing.T) {
	t.Parallel()

	// Prerelease version: exercises the staging-ECR routing in
	// teleportassets for both the agent and updater images.
	const tag = "99.0.0-alpha.1"
	version := semver.New(tag)

	opts := baseOpts()
	opts.Enterprise = false // ensure the OSS swap branch is the alternative
	opts.Updater = true     // forces TELEPORT_EXT_UPGRADER_VERSION to be present
	opts.UpdaterChannel = "stable/cloud"
	opts.RequestedVersion = version

	objs, err := Manifests(opts)
	require.NoError(t, err)

	sts := getObject[*appsv1.StatefulSet](t, objs)
	c := sts.Spec.Template.Spec.Containers[0]

	// Agent image points at the staging registry (prerelease) with the
	// override tag, and was not subjected to the OSS substring swap.
	require.Contains(t, c.Image, "gravitational-staging/")
	require.True(t, strings.HasSuffix(c.Image, ":"+tag))

	// Env var carrying the version is rewritten to the override's tag.
	var found bool
	for _, e := range c.Env {
		if e.Name == "TELEPORT_EXT_UPGRADER_VERSION" {
			require.Equal(t, tag, e.Value)
			found = true
			break
		}
	}
	require.True(t, found)

	// Updater image is derived from the same version and also routes to staging.
	updater := getObject[*appsv1.Deployment](t, objs)
	uc := updater.Spec.Template.Spec.Containers[0]
	require.Contains(t, uc.Image, "gravitational-staging/")
	require.Contains(t, uc.Image, "teleport-kube-agent-updater:"+tag)

	// --base-image arg gets the agent image's repo.
	var baseImage string
	for _, a := range uc.Args {
		if strings.HasPrefix(a, "--base-image=") {
			baseImage = strings.TrimPrefix(a, "--base-image=")
			break
		}
	}
	require.NotEmpty(t, baseImage)
	require.Equal(t, strings.TrimSuffix(c.Image, ":"+tag), baseImage)
}

func TestManifests_HighAvailability(t *testing.T) {
	t.Parallel()

	opts := baseOpts()
	opts.HighAvailability = true

	objs, err := Manifests(opts)
	require.NoError(t, err)

	sts := getObject[*appsv1.StatefulSet](t, objs)
	require.EqualValues(t, 2, *sts.Spec.Replicas)

	pdb := getObject[*policyv1.PodDisruptionBudget](t, objs)
	require.NotNil(t, pdb.Spec.MinAvailable)
	require.Equal(t, 1, pdb.Spec.MinAvailable.IntValue())
}

func TestManifests_Updater(t *testing.T) {
	t.Parallel()

	opts := baseOpts()
	opts.Updater = true
	opts.UpdaterChannel = "stable/cloud"

	objs, err := Manifests(opts)
	require.NoError(t, err)

	kinds := kindsByType(objs)
	require.Equal(t, 1, kinds["Deployment"])
	require.Equal(t, 2, kinds["ServiceAccount"])
	require.Equal(t, 2, kinds["Role"])
	require.Equal(t, 2, kinds["RoleBinding"])
}

func TestManifests_KubeAppDisc_ConfigPayload(t *testing.T) {
	t.Parallel()

	opts := baseOpts()
	opts.Roles = RoleKubeAppDiscovery

	objs, err := Manifests(opts)
	require.NoError(t, err)

	cm := getObject[*corev1.ConfigMap](t, objs)
	cfg, err := config.ReadConfig(strings.NewReader(cm.Data["teleport.yaml"]))
	require.NoError(t, err)
	require.True(t, cfg.Discovery.Enabled())
	require.True(t, cfg.Apps.Enabled())
	require.True(t, cfg.Kube.Enabled())
	require.False(t, cfg.Auth.Enabled())
	require.False(t, cfg.Databases.Enabled())

	kubeOnly, err := Manifests(baseOpts())
	require.NoError(t, err)
	kubeCM := getObject[*corev1.ConfigMap](t, kubeOnly)
	cfg, err = config.ReadConfig(strings.NewReader(kubeCM.Data["teleport.yaml"]))
	require.NoError(t, err)
	require.True(t, cfg.Kube.Enabled())
	require.False(t, cfg.Apps.Enabled())
	require.False(t, cfg.Discovery.Enabled())
	require.False(t, cfg.Auth.Enabled())
	require.False(t, cfg.Databases.Enabled())

	// Discovery requires cluster-wide list on Services. The chart gates
	// this rule on the discovery role. Verify the composer adds it for
	// RoleKubeAppDiscovery and omits it for RoleKube.
	require.True(t, hasServicesListRule(getObject[*rbacv1.ClusterRole](t, objs)))
	require.False(t, hasServicesListRule(getObject[*rbacv1.ClusterRole](t, kubeOnly)))
}

func hasServicesListRule(cr *rbacv1.ClusterRole) bool {
	for _, r := range cr.Rules {
		if slices.Contains(r.APIGroups, "") &&
			slices.Contains(r.Resources, "services") &&
			slices.Contains(r.Verbs, "list") {
			return true
		}
	}
	return false
}

// TestManifests_Labels verifies that Options.Labels are routed into the
// rendered teleport.yaml's kubernetes_service.labels.
func TestManifests_Labels(t *testing.T) {
	t.Parallel()

	opts := baseOpts()
	opts.Labels = map[string]string{
		"teleport.internal/resource-id": "abc-123",
		"region":                        "us-east-1",
		// Mirrors a real EKS-tag-derived label that fails k8s pod-label
		// validation. Confirms we don't try to route it through pod meta.
		"aws:cloudformation:stack-name": "eksctl-cluster",
	}

	objs, err := Manifests(opts)
	require.NoError(t, err)

	// Pod template labels must NOT contain the user-supplied labels.
	sts := getObject[*appsv1.StatefulSet](t, objs)
	for k := range opts.Labels {
		_, present := sts.Spec.Template.Labels[k]
		require.False(t, present)
	}

	// They MUST appear under kubernetes_service.labels in the rendered config.
	cm := getObject[*corev1.ConfigMap](t, objs)

	cfg, err := config.ReadConfig(strings.NewReader(cm.Data["teleport.yaml"]))
	require.NoError(t, err)
	require.True(t, cfg.Kube.Enabled())

	for k, want := range opts.Labels {
		got, present := cfg.Kube.StaticLabels[k]
		require.True(t, present)
		require.Equal(t, want, got)
	}
}

func TestManifests_NoPlaceholderLeaks(t *testing.T) {
	t.Parallel()

	opts := baseOpts()
	opts.Enterprise = true
	opts.HighAvailability = true
	opts.Updater = true
	opts.UpdaterChannel = "stable/cloud"
	opts.Labels = map[string]string{"region": "us-east-1"}

	objs, err := Manifests(opts)
	require.NoError(t, err)

	placeholders := []string{
		placeholderNamespace,
		placeholderProxy,
		placeholderToken,
		placeholderCluster,
		placeholderChannel,
	}
	for _, o := range objs {
		content, err := yaml.Marshal(o)
		require.NoError(t, err)
		for _, p := range placeholders {
			require.NotContainsf(t, string(content), p, "placeholder %q leaked into %T %s/%s", p, o, o.GetNamespace(), o.GetName())
		}
	}
}

func kindsByType(objs []client.Object) map[string]int {
	out := map[string]int{}
	for _, o := range objs {
		out[reflect.TypeOf(o).Elem().Name()]++
	}
	return out
}

// getObject finds exactly one object of type T in objs, failing the test if
// there are zero or more than one matches.
func getObject[T client.Object](t *testing.T, objs []client.Object) T {
	t.Helper()
	var matches []T
	for _, o := range objs {
		if cast, ok := o.(T); ok {
			matches = append(matches, cast)
		}
	}
	require.Len(t, matches, 1)
	return matches[0]
}
