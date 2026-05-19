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
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/google/go-cmp/cmp"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/gravitational/teleport/lib/integrations/awsoidc/internal/kubeagent"
)

const (
	chartPath  = "../../../../../../../examples/chart/teleport-kube-agent"
	valuesPath = "../../testdata/values.yaml"
)

func TestGeneratedSourceIsCurrent(t *testing.T) {
	var out bytes.Buffer
	if err := run(
		[]string{
			"-chart", chartPath,
			"-values", valuesPath,
			"-out", "-",
		},
		&out,
		io.Discard,
	); err != nil {
		t.Fatalf("run generator: %v", err)
	}

	want, err := os.ReadFile("../../zz_generated.go")
	if err != nil {
		t.Fatalf("read checked-in generated output: %v", err)
	}

	if !bytes.Equal(out.Bytes(), want) {
		t.Fatalf("zz_generated.go is stale; run go generate ./lib/integrations/awsoidc/internal/kubeagent")
	}
}

func TestManifestsMatch(t *testing.T) {
	chart, err := loader.Load(chartPath)
	if err != nil {
		t.Fatalf("load chart: %v", err)
	}

	for _, variant := range allChartVariants {
		t.Run(variant.suffix(), func(t *testing.T) {
			rendered, err := renderTemplates(chart, renderOptions{
				valuesFile: valuesPath,
				roles:      strings.Split(variant.roles, ","),
				updater:    variant.updater,
				ha:         variant.ha,
				enterprise: variant.enterprise,
				skipHooks:  true,
			})
			if err != nil {
				t.Fatalf("render helm templates: %v", err)
			}

			_, manifests, err := releaseutil.SortManifests(rendered, nil, releaseutil.InstallOrder)
			if err != nil {
				t.Fatalf("sort helm manifests: %v", err)
			}

			runtimeObjects, err := kubeagent.Manifests(variant.Options())
			if err != nil {
				t.Fatalf("build runtime manifests: %v", err)
			}

			want := manifestMap(t, manifests)
			got := objectMap(t, runtimeObjects)
			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatalf("runtime manifests do not match rendered Helm manifests (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHelmInterop(t *testing.T) {
	objs, err := kubeagent.Manifests(kubeagent.ChartOptions{
		Namespace:        "teleport-agent",
		ProxyAddr:        "proxy.example.com:443",
		AuthToken:        "join-token",
		KubeClusterName:  "test-cluster",
		Roles:            kubeagent.RoleKubeAppDiscovery,
		Updater:          true,
		UpdaterChannel:   "stable/cloud",
		HighAvailability: true,
		Labels:           map[string]string{"env": "prod"},
	})
	if err != nil {
		t.Fatalf("build runtime objects: %v", err)
	}

	kubeClient := &recordingKubeClient{}
	cfg := &action.Configuration{
		Releases:     storage.Init(driver.NewMemory()),
		KubeClient:   kubeClient,
		Capabilities: chartutil.DefaultCapabilities,
		Log:          func(string, ...any) {},
	}

	rel := releaseFromRuntimeObjects(t, objs)
	if err := cfg.Releases.Create(rel); err != nil {
		t.Fatalf("seed helm storage: %v", err)
	}

	if _, err := action.NewStatus(cfg).Run(releaseName); err != nil {
		t.Fatalf("helm status action rejected generated release: %v", err)
	}

	values, err := action.NewGetValues(cfg).Run(releaseName)
	if err != nil {
		t.Fatalf("helm get values action rejected generated release: %v", err)
	}

	labels, ok := values["labels"].(map[string]any)
	if !ok || labels["env"] != "prod" {
		t.Fatalf("helm values lost labels: %#v", values["labels"])
	}

	allValues := action.NewGetValues(cfg)
	allValues.AllValues = true
	if _, err := allValues.Run(releaseName); err != nil {
		t.Fatalf("helm get values --all action rejected generated release: %v", err)
	}

	chart, err := loader.Load(chartPath)
	if err != nil {
		t.Fatalf("load chart: %v", err)
	}

	upgrade := action.NewUpgrade(cfg)
	upgrade.Namespace = "teleport-agent"
	upgrade.ReuseValues = true
	upgrade.DryRun = true
	upgraded, err := upgrade.Run(releaseName, chart, nil)
	if err != nil {
		t.Fatalf("helm upgrade --reuse-values action rejected generated release: %v", err)
	}

	upgradedLabels, ok := upgraded.Config["labels"].(map[string]any)
	if !ok || upgradedLabels["env"] != "prod" {
		t.Fatalf("helm upgrade --reuse-values lost labels: %#v", upgraded.Config["labels"])
	}

	if _, err := action.NewUninstall(cfg).Run(releaseName); err != nil {
		t.Fatalf("helm uninstall action rejected generated release: %v", err)
	}

	var postDeleteHookBuilds int
	for _, build := range kubeClient.builds {
		if build.validate && strings.Contains(build.manifest, `args: ["kube-state", "delete"]`) {
			postDeleteHookBuilds++
		}
	}

	if postDeleteHookBuilds != 1 {
		t.Fatalf("expected helm uninstall to execute post-delete cleanup Job once, got %d hook builds", postDeleteHookBuilds)
	}
}

func TestReleaseValues(t *testing.T) {
	objs, err := kubeagent.Manifests(kubeagent.ChartOptions{
		Namespace:       "teleport-agent",
		ProxyAddr:       "proxy.example.com:443",
		AuthToken:       "join-token",
		KubeClusterName: "test-cluster",
		Roles:           kubeagent.RoleKubeAppDiscovery,
		Labels:          map[string]string{"env": "prod"},
	})
	if err != nil {
		t.Fatalf("build runtime objects: %v", err)
	}

	cfg := &action.Configuration{
		Releases:     storage.Init(driver.NewMemory()),
		KubeClient:   &recordingKubeClient{},
		Capabilities: chartutil.DefaultCapabilities,
		Log:          func(string, ...any) {},
	}
	if err := cfg.Releases.Create(releaseFromRuntimeObjects(t, objs)); err != nil {
		t.Fatalf("seed helm storage: %v", err)
	}

	values, err := action.NewGetValues(cfg).Run(releaseName)
	if err != nil {
		t.Fatalf("helm get values rejected generated release: %v", err)
	}

	if _, ok := values["updater"]; ok {
		t.Fatalf("updater should be absent when updater was disabled: %#v", values)
	}
	if _, ok := values["highAvailability"]; ok {
		t.Fatalf("highAvailability should be absent when updater was disabled: %#v", values)
	}
	if _, ok := values["podSecurityPolicy"]; ok {
		t.Fatalf("podSecurityPolicy should be absent: %#v", values)
	}

	chart, err := loader.Load(chartPath)
	if err != nil {
		t.Fatalf("load chart: %v", err)
	}

	upgrade := action.NewUpgrade(cfg)
	upgrade.Namespace = "teleport-agent"
	upgrade.ReuseValues = true
	upgrade.DryRun = true
	upgraded, err := upgrade.Run(releaseName, chart, map[string]any{
		"updater": map[string]any{"enabled": true},
	})
	if err != nil {
		t.Fatalf("helm upgrade --reuse-values rejected generated release: %v", err)
	}

	if !strings.Contains(upgraded.Manifest, "--version-channel=stable/cloud") {
		t.Fatalf("upgraded manifest did not render chart default release channel")
	}
}

func TestHooksMatch(t *testing.T) {
	chart, err := loader.Load(chartPath)
	if err != nil {
		t.Fatalf("load chart: %v", err)
	}

	for _, variant := range allHookVariants {
		t.Run(fmt.Sprintf("enterprise=%t", variant.enterprise), func(t *testing.T) {
			rendered, err := renderTemplates(chart, renderOptions{
				valuesFile: valuesPath,
				roles:      strings.Split(RoleKubeAppDiscovery, ","),
				updater:    true,
				ha:         true,
				enterprise: variant.enterprise,
				skipHooks:  false,
			})
			if err != nil {
				t.Fatalf("render helm templates: %v", err)
			}

			wantHooks, _, err := releaseutil.SortManifests(rendered, nil, releaseutil.InstallOrder)
			if err != nil {
				t.Fatalf("sort helm hooks: %v", err)
			}

			runtimeObjects, err := kubeagent.Manifests(kubeagent.ChartOptions{
				Namespace:        placeholderNamespace,
				ProxyAddr:        "proxy-placeholder:443",
				AuthToken:        "token-placeholder",
				KubeClusterName:  "cluster-placeholder",
				Roles:            kubeagent.RoleKubeAppDiscovery,
				Enterprise:       variant.enterprise,
				Updater:          true,
				UpdaterChannel:   "channel-placeholder",
				HighAvailability: true,
				RequestedVersion: semver.New(placeholderVersion),
			})
			if err != nil {
				t.Fatalf("build runtime manifests: %v", err)
			}

			gotHooks := releaseFromRuntimeObjects(t, runtimeObjects).Hooks
			if diff := cmp.Diff(hookMap(wantHooks), hookMap(gotHooks)); diff != "" {
				t.Fatalf("runtime hooks do not match rendered Helm hooks (-want +got):\n%s", diff)
			}
		})
	}
}

type comparableHook struct {
	Name           string
	Kind           string
	Path           string
	Manifest       string
	Events         []string
	Weight         int
	DeletePolicies []string
}

func hookMap(hooks []*release.Hook) map[string]comparableHook {
	out := make(map[string]comparableHook, len(hooks))
	for _, hook := range hooks {
		key := strings.Join([]string{
			hook.Path,
			hook.Kind,
			hook.Name,
			strconv.Itoa(hook.Weight),
		}, "/")

		out[key] = comparableHook{
			Name:           hook.Name,
			Kind:           hook.Kind,
			Path:           hook.Path,
			Manifest:       hook.Manifest,
			Events:         hookEventsToStrings(hook.Events),
			Weight:         hook.Weight,
			DeletePolicies: hookDeletePoliciesToStrings(hook.DeletePolicies),
		}
	}

	return out
}
func hookEventsToStrings(events []release.HookEvent) []string {
	out := make([]string, 0, len(events))
	for _, event := range events {
		out = append(out, string(event))
	}
	return out
}

func hookDeletePoliciesToStrings(policies []release.HookDeletePolicy) []string {
	out := make([]string, 0, len(policies))
	for _, policy := range policies {
		out = append(out, string(policy))
	}
	return out
}

type recordingKubeClient struct {
	kube.Interface
	builds []buildCall
}

type buildCall struct {
	manifest string
	validate bool
}

func (r *recordingKubeClient) Create(resources kube.ResourceList) (*kube.Result, error) {
	return &kube.Result{Created: resources}, nil
}

func (r *recordingKubeClient) Delete(resources kube.ResourceList) (*kube.Result, []error) {
	return &kube.Result{Deleted: resources}, nil
}

func (r *recordingKubeClient) WatchUntilReady(kube.ResourceList, time.Duration) error {
	return nil
}

func (r *recordingKubeClient) Update(_, target kube.ResourceList, _ bool) (*kube.Result, error) {
	return &kube.Result{Updated: target}, nil
}

func (r *recordingKubeClient) Build(reader io.Reader, validate bool) (kube.ResourceList, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	r.builds = append(r.builds, buildCall{manifest: string(raw), validate: validate})
	return []*resource.Info{}, nil
}

func (r *recordingKubeClient) WaitAndGetCompletedPodPhase(string, time.Duration) (corev1.PodPhase, error) {
	return corev1.PodSucceeded, nil
}

func (r *recordingKubeClient) IsReachable() error {
	return nil
}

func releaseFromRuntimeObjects(t *testing.T, objs []client.Object) *release.Release {
	t.Helper()
	for _, obj := range objs {
		secret, ok := obj.(*corev1.Secret)
		if !ok || secret.Type != "helm.sh/release.v1" {
			continue
		}
		return decodeReleaseSecret(t, secret)
	}

	t.Fatal("generated runtime objects did not include helm release secret")
	return nil
}

func decodeReleaseSecret(t *testing.T, secret *corev1.Secret) *release.Release {
	t.Helper()
	encoded := secret.Data["release"]
	if len(encoded) == 0 {
		t.Fatal("release secret is missing data[release]")
	}

	compressed, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		t.Fatalf("decode release: %v", err)
	}

	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("open release gzip: %v", err)
	}

	defer gz.Close()
	raw, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("read release gzip: %v", err)
	}

	var rel release.Release
	if err := json.Unmarshal(raw, &rel); err != nil {
		t.Fatalf("unmarshal helm release: %v", err)
	}

	return &rel
}

func manifestMap(t *testing.T, manifests []releaseutil.Manifest) map[string]map[string]any {
	t.Helper()

	var b bytes.Buffer
	for i, manifest := range manifests {
		if i > 0 {
			b.WriteString("---\n")
		}

		b.WriteString(manifest.Content)
		if !strings.HasSuffix(manifest.Content, "\n") {
			b.WriteByte('\n')
		}
	}

	reader := utilyaml.NewYAMLReader(bufio.NewReader(&b))
	out := make(map[string]map[string]any, len(manifests))
	for {
		doc, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatalf("read manifest doc: %v", err)
		}

		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}

		obj := manifestToUnstructured(t, doc)
		normalizeManifestObject(t, obj)
		out[manifestObjectKey(obj)] = obj.Object
	}
}

func objectMap(t *testing.T, objs []client.Object) map[string]map[string]any {
	t.Helper()

	out := make(map[string]map[string]any, len(objs))
	for _, obj := range objs {
		if secret, ok := obj.(*corev1.Secret); ok && secret.Type == "helm.sh/release.v1" {
			continue
		}

		u, ok := obj.(*unstructured.Unstructured)
		if !ok {
			t.Fatalf("unexpected object type %T", obj)
		}

		u = u.DeepCopy()
		normalizeManifestObject(t, u)
		out[manifestObjectKey(u)] = u.Object
	}

	return out
}

func manifestToUnstructured(t *testing.T, content []byte) *unstructured.Unstructured {
	t.Helper()

	raw, err := yaml.YAMLToJSON(content)
	if err != nil {
		t.Fatalf("convert YAML to JSON: %v", err)
	}

	obj := &unstructured.Unstructured{}
	if err := json.Unmarshal(raw, &obj.Object); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	return obj
}

func manifestObjectKey(obj *unstructured.Unstructured) string {
	return strings.Join([]string{
		obj.GetAPIVersion(),
		obj.GetKind(),
		obj.GetNamespace(),
		obj.GetName(),
	}, "/")
}

func normalizeManifestObject(t *testing.T, obj *unstructured.Unstructured) {
	t.Helper()

	removeMetadataStringMapKey(t, obj, "labels", "app.kubernetes.io/managed-by")
	removeMetadataStringMapKey(t, obj, "labels", "helm.sh/chart")
	removeMetadataStringMapKey(t, obj, "annotations", "meta.helm.sh/release-name")
	removeMetadataStringMapKey(t, obj, "annotations", "meta.helm.sh/release-namespace")
}

func removeMetadataStringMapKey(t *testing.T, obj *unstructured.Unstructured, field, key string) {
	t.Helper()

	values, found, err := unstructured.NestedStringMap(obj.Object, "metadata", field)
	if err != nil {
		t.Fatalf("read metadata.%s from %s: %v", field, manifestObjectKey(obj), err)
	}

	if !found {
		return
	}

	delete(values, key)
	if len(values) == 0 {
		unstructured.RemoveNestedField(obj.Object, "metadata", field)
		return
	}

	if err := unstructured.SetNestedStringMap(obj.Object, values, "metadata", field); err != nil {
		t.Fatalf("write metadata.%s to %s: %v", field, manifestObjectKey(obj), err)
	}
}
