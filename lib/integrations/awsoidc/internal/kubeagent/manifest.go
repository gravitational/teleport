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

// Package kubeagent applies the teleport-kube-agent chart output without a
// runtime dependency on Helm. The chart is rendered at build time by
// internal/gen; runtime code selects one compressed manifest variant, patches
// known install-time values, and writes a Helm-compatible release Secret.
package kubeagent

//go:generate go run -C ./internal/gen . -chart ../../../../../../../examples/chart/teleport-kube-agent -values ../../testdata/values.yaml -out ../../zz_generated.go

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils/teleportassets"
)

// TeleportSystemRoles defines the Teleport services enabled for the agent.
type TeleportSystemRoles string

const (
	// RoleKube indicates only the Kubernetes role is enabled.
	RoleKube TeleportSystemRoles = "kube"
	// RoleKubeAppDiscovery indicates Kubernetes, App, and Discovery roles are enabled.
	RoleKubeAppDiscovery TeleportSystemRoles = "kube,app,discovery"
)

const (
	placeholderNamespace = "ns-placeholder"
	placeholderProxy     = "proxy-placeholder:443"
	placeholderToken     = "token-placeholder"
	placeholderCluster   = "cluster-placeholder"
	placeholderChannel   = "channel-placeholder"
)

// ChartOptions configures the generated teleport-kube-agent install.
type ChartOptions struct {
	Namespace       string
	ProxyAddr       string
	AuthToken       string
	KubeClusterName string
	Roles           TeleportSystemRoles

	Enterprise bool

	Updater        bool
	UpdaterChannel string

	HighAvailability bool

	// RequestedVersion controls the image tag only. The chart structure is
	// fixed to the Teleport binary build.
	RequestedVersion *semver.Version

	Labels map[string]string
}

func (o ChartOptions) validate() error {
	switch {
	case o.Namespace == "":
		return trace.BadParameter("Namespace is required")
	case strings.ContainsAny(o.Namespace, "\r\n"):
		return trace.BadParameter("Namespace must not contain newlines")
	case o.ProxyAddr == "":
		return trace.BadParameter("ProxyAddr is required")
	case strings.ContainsAny(o.ProxyAddr, "\r\n"):
		return trace.BadParameter("ProxyAddr must not contain newlines")
	case o.AuthToken == "":
		return trace.BadParameter("AuthToken is required")
	case strings.ContainsAny(o.AuthToken, "\r\n"):
		return trace.BadParameter("AuthToken must not contain newlines")
	case o.KubeClusterName == "":
		return trace.BadParameter("KubeClusterName is required")
	case strings.ContainsAny(o.KubeClusterName, "\r\n"):
		return trace.BadParameter("KubeClusterName must not contain newlines")
	case o.Roles != RoleKube && o.Roles != RoleKubeAppDiscovery:
		return trace.BadParameter("Roles must be %q or %q, got %q", RoleKube, RoleKubeAppDiscovery, o.Roles)
	case o.Updater && o.UpdaterChannel == "":
		return trace.BadParameter("UpdaterChannel is required when Updater is true")
	case strings.ContainsAny(o.UpdaterChannel, "\r\n"):
		return trace.BadParameter("UpdaterChannel must not contain newlines")
	}
	return nil
}

// Apply creates the namespace, rendered chart objects, and Helm release Secret.
func Apply(ctx context.Context, clientGetter genericclioptions.RESTClientGetter, opts ChartOptions) error {
	if err := opts.validate(); err != nil {
		return trace.Wrap(err)
	}

	kubeClient, err := newKubeClient(clientGetter)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := kubeClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: opts.Namespace, Namespace: ""}}); err != nil && !apierrors.IsAlreadyExists(err) {
		return trace.Wrap(err, "creating namespace %q", opts.Namespace)
	}

	objs, err := Manifests(opts)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, obj := range objs {
		if err := createReleaseObject(ctx, kubeClient, obj, opts.Namespace); err != nil {
			return trace.Wrap(err, "creating %s %s/%s", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetNamespace(), obj.GetName())
		}
	}

	return nil
}

func createReleaseObject(ctx context.Context, kubeClient client.Client, desired client.Object, releaseNamespace string) error {
	err := kubeClient.Create(ctx, desired)
	if err == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	if secret, ok := desired.(*corev1.Secret); ok && secret.Type == helmSecretType {
		return trace.Wrap(err)
	}

	existing, ok := desired.DeepCopyObject().(client.Object)
	if !ok {
		return trace.BadParameter("expected %T, got %T", desired, existing)
	}

	if err := kubeClient.Get(ctx, client.ObjectKeyFromObject(desired), existing); err != nil {
		return trace.Wrap(err)
	}

	if err := validateExistingReleaseObject(existing, releaseNamespace); err != nil {
		return trace.Wrap(err)
	}

	desired.SetResourceVersion(existing.GetResourceVersion())
	desired.SetUID(existing.GetUID())
	return trace.Wrap(kubeClient.Update(ctx, desired))
}

func validateExistingReleaseObject(existing client.Object, releaseNamespace string) error {
	labels, annotations := helmMetadata(releaseNamespace)
	if existing.GetLabels()["app.kubernetes.io/managed-by"] != labels["app.kubernetes.io/managed-by"] ||
		existing.GetAnnotations()["meta.helm.sh/release-name"] != annotations["meta.helm.sh/release-name"] ||
		existing.GetAnnotations()["meta.helm.sh/release-namespace"] != annotations["meta.helm.sh/release-namespace"] {
		return trace.AlreadyExists("%s %s/%s already exists and is not owned by release %q", existing.GetObjectKind().GroupVersionKind().Kind, existing.GetNamespace(), existing.GetName(), helmReleaseName)
	}

	return nil
}

// IsInstalled reports whether a deployed Helm release secret exists.
func IsInstalled(ctx context.Context, clientGetter genericclioptions.RESTClientGetter, namespace string) (bool, error) {
	kubeClient, err := newKubeClient(clientGetter)
	if err != nil {
		return false, trace.Wrap(err)
	}

	var secrets corev1.SecretList
	if err := kubeClient.List(ctx, &secrets,
		client.InNamespace(namespace),
		client.MatchingLabels{"owner": "helm", "name": helmReleaseName, "status": helmReleaseStatus},
	); err != nil {
		return false, trace.Wrap(err)
	}

	return len(secrets.Items) > 0, nil
}

func newKubeClient(clientGetter genericclioptions.RESTClientGetter) (client.Client, error) {
	restConfig, err := clientGetter.ToRESTConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.New(restConfig, client.Options{Scheme: scheme.Scheme})
}

// Manifests returns the Kubernetes objects for opts, including the Helm release
// Secret. The namespace object is created separately and is not part of the
// release, matching Helm's CreateNamespace behavior.
func Manifests(opts ChartOptions) ([]client.Object, error) {
	if err := opts.validate(); err != nil {
		return nil, trace.Wrap(err)
	}

	compressed, ok := compressedManifest(opts)
	if !ok {
		return nil, trace.BadParameter("no rendered manifest variant for roles=%q updater=%v highAvailability=%v", opts.Roles, opts.Updater, opts.HighAvailability)
	}

	manifest, err := decompressString(compressed)
	if err != nil {
		return nil, trace.Wrap(err, "decompressing manifest")
	}

	manifest = applyTextSubstitutions(manifest, opts)

	unstructuredObjs, err := decodeManifestObjects(manifest)
	if err != nil {
		return nil, trace.Wrap(err, "decoding manifest")
	}

	for _, u := range unstructuredObjs {
		if err := patchObject(u, opts); err != nil {
			return nil, trace.Wrap(err, "patching %s/%s", u.GetKind(), u.GetName())
		}
		stampHelmMetadata(u, opts.Namespace)
	}

	releaseSecret, err := buildReleaseSecret(unstructuredObjs, opts)
	if err != nil {
		return nil, trace.Wrap(err, "building helm release secret")
	}

	objs := make([]client.Object, 0, len(unstructuredObjs)+1)
	for _, obj := range unstructuredObjs {
		objs = append(objs, obj)
	}
	objs = append(objs, releaseSecret)

	return objs, nil
}

func buildReleaseSecret(objs []*unstructured.Unstructured, opts ChartOptions) (*corev1.Secret, error) {
	manifestYAML, err := joinAsManifestYAML(objs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hooks, err := buildHooks(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	now := time.Now().UTC()
	return helmReleaseSecret(helmRelease{
		Name:      helmReleaseName,
		Namespace: opts.Namespace,
		Version:   helmReleaseRev,
		Info: helmReleaseInfo{
			FirstDeployed: now,
			LastDeployed:  now,
			Description:   "Install complete",
			Status:        helmReleaseStatus,
		},
		Chart:    json.RawMessage(chartJSON),
		Config:   helmReleaseConfig(opts),
		Manifest: manifestYAML,
		Hooks:    hooks,
	})
}

func helmReleaseConfig(opts ChartOptions) map[string]any {
	config := map[string]any{
		"roles":           string(opts.Roles),
		"proxyAddr":       opts.ProxyAddr,
		"authToken":       opts.AuthToken,
		"kubeClusterName": opts.KubeClusterName,
		"labels":          opts.Labels,
	}

	if opts.Enterprise {
		config["enterprise"] = true
	}

	if opts.Updater {
		config["updater"] = map[string]any{"enabled": true, "releaseChannel": opts.UpdaterChannel}
	}

	if opts.HighAvailability {
		config["highAvailability"] = map[string]any{
			"replicaCount":        2,
			"podDisruptionBudget": map[string]any{"enabled": true, "minAvailable": 1},
		}
	}

	return config
}

type generatedHook struct {
	Name           string
	Kind           string
	Path           string
	Manifest       string
	Events         []string
	Weight         int
	DeletePolicies []string
}

func buildHooks(opts ChartOptions) ([]*helmHook, error) {
	generated := generatedHooksOSS
	if opts.Enterprise {
		generated = generatedHooksEnterprise
	}

	hooks := make([]*helmHook, 0, len(generated))
	for _, h := range generated {
		manifest, err := decompressString(h.Manifest)
		if err != nil {
			return nil, trace.Wrap(err, "decompressing hook %q", h.Name)
		}

		manifest = applyTextSubstitutions(manifest, opts)
		hooks = append(hooks, &helmHook{
			Name:           h.Name,
			Kind:           h.Kind,
			Path:           h.Path,
			Manifest:       manifest,
			Events:         h.Events,
			Weight:         h.Weight,
			DeletePolicies: h.DeletePolicies,
		})
	}

	return hooks, nil
}

func applyTextSubstitutions(manifest string, opts ChartOptions) string {
	const (
		generatedImageRepo            = "public.ecr.aws/gravitational"
		generatedEnterpriseAgentImage = generatedImageRepo + "/teleport-ent-distroless:" + generatedVersion
		generatedOSSAgentImage        = generatedImageRepo + "/teleport-distroless:" + generatedVersion
		generatedEnterpriseAgentBase  = generatedImageRepo + "/teleport-ent-distroless"
		generatedOSSAgentBase         = generatedImageRepo + "/teleport-distroless"
		generatedUpdaterImage         = generatedImageRepo + "/teleport-kube-agent-updater:" + generatedVersion
	)

	generatedAgentImage := generatedOSSAgentImage
	generatedAgentBase := generatedOSSAgentBase
	buildType := modules.BuildOSS
	if opts.Enterprise {
		generatedAgentImage = generatedEnterpriseAgentImage
		generatedAgentBase = generatedEnterpriseAgentBase
		buildType = modules.BuildEnterprise
	}

	desiredVersion := api.Version
	agentImage := generatedAgentImage
	agentBase := generatedAgentBase
	updaterImage := generatedUpdaterImage
	if opts.RequestedVersion != nil {
		desiredVersion = opts.RequestedVersion.String()
		agentImage = teleportassets.DistrolessImage(*opts.RequestedVersion, buildType)
		agentBase = trimImageTag(agentImage)
		updaterImage = teleportassets.KubeAgentUpdaterImage(*opts.RequestedVersion)
	}

	return strings.NewReplacer(
		placeholderNamespace, opts.Namespace,
		placeholderProxy, opts.ProxyAddr,
		placeholderToken, opts.AuthToken,
		placeholderCluster, opts.KubeClusterName,
		placeholderChannel, opts.UpdaterChannel,
		generatedAgentImage, agentImage,
		"--base-image="+generatedAgentBase, "--base-image="+agentBase,
		generatedUpdaterImage, updaterImage,
		generatedVersion, desiredVersion,
	).Replace(manifest)
}

func patchObject(obj *unstructured.Unstructured, opts ChartOptions) error {
	if obj.GetNamespace() != "" {
		obj.SetNamespace(opts.Namespace)
	}

	if err := patchSubjectNamespaces(obj, opts.Namespace); err != nil {
		return err
	}

	if obj.GetKind() == "ConfigMap" && obj.GetName() == helmReleaseName {
		return patchTeleportConfig(obj, opts)
	}

	return nil
}

func patchSubjectNamespaces(obj *unstructured.Unstructured, namespace string) error {
	subjects, found, err := unstructured.NestedSlice(obj.Object, "subjects")
	if err != nil {
		return trace.Wrap(err, "reading Subjects data")
	}

	if !found {
		return nil
	}

	for i := range subjects {
		subject, ok := subjects[i].(map[string]any)
		if !ok {
			continue
		}

		if subject["kind"] == "ServiceAccount" {
			subject["namespace"] = namespace
		}
	}

	if err := unstructured.SetNestedSlice(obj.Object, subjects, "subjects"); err != nil {
		return err
	}

	return nil
}

func patchTeleportConfig(obj *unstructured.Unstructured, opts ChartOptions) error {
	data, found, err := unstructured.NestedStringMap(obj.Object, "data")
	if err != nil || !found {
		return trace.Wrap(err, "reading ConfigMap data")
	}

	payload := data["teleport.yaml"]
	if len(opts.Labels) > 0 {
		fileConfig, err := config.ReadConfig(strings.NewReader(payload))
		if err != nil {
			return trace.Wrap(err, "parsing teleport.yaml")
		}

		if !fileConfig.Kube.Enabled() {
			return trace.BadParameter("teleport.yaml is missing kubernetes_service")
		}

		fileConfig.Kube.StaticLabels = opts.Labels

		payload, err = fileConfig.YAMLString()
		if err != nil {
			return trace.Wrap(err, "serializing teleport.yaml")
		}
	}

	data["teleport.yaml"] = payload

	return trace.Wrap(unstructured.SetNestedStringMap(obj.Object, data, "data"))
}

func decodeManifestObjects(manifest string) ([]*unstructured.Unstructured, error) {
	reader := utilyaml.NewYAMLReader(bufio.NewReader(strings.NewReader(manifest)))
	var out []*unstructured.Unstructured
	for i := 0; ; i++ {
		doc, err := reader.Read()
		switch {
		case errors.Is(err, io.EOF):
			return out, nil
		case err != nil:
			return nil, trace.Wrap(err, "reading doc %d", i)
		case len(bytes.TrimSpace(doc)) == 0:
			continue
		}

		jsonDoc, err := yaml.YAMLToJSON(doc)
		if err != nil {
			return nil, trace.Wrap(err, "converting doc %d to JSON", i)
		}

		obj := &unstructured.Unstructured{}
		if err := json.Unmarshal(jsonDoc, &obj.Object); err != nil {
			return nil, trace.Wrap(err, "unmarshaling doc %d", i)
		}

		out = append(out, obj)
	}
}

func joinAsManifestYAML(objs []*unstructured.Unstructured) (string, error) {
	var b strings.Builder
	for i, obj := range objs {
		out, err := yaml.Marshal(obj.Object)
		if err != nil {
			return "", trace.Wrap(err, "marshaling %s %s/%s", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetNamespace(), obj.GetName())
		}

		if i > 0 {
			b.WriteString("---\n")
		}

		b.Write(out)
	}

	return b.String(), nil
}

func stampHelmMetadata(obj *unstructured.Unstructured, releaseNamespace string) {
	labels, annotations := helmMetadata(releaseNamespace)

	objLabels := obj.GetLabels()
	if objLabels == nil {
		objLabels = map[string]string{}
	}

	maps.Copy(objLabels, labels)
	obj.SetLabels(objLabels)

	objAnnotations := obj.GetAnnotations()
	if objAnnotations == nil {
		objAnnotations = map[string]string{}
	}

	maps.Copy(objAnnotations, annotations)
	obj.SetAnnotations(objAnnotations)
}

func helmMetadata(releaseNamespace string) (map[string]string, map[string]string) {
	return map[string]string{
			"app.kubernetes.io/managed-by": "Helm",
			"helm.sh/chart":                "teleport-kube-agent-" + strings.ReplaceAll(api.Version, "+", "_"),
		}, map[string]string{
			"meta.helm.sh/release-name":      helmReleaseName,
			"meta.helm.sh/release-namespace": releaseNamespace,
		}
}

func decompressString(encoded string) (string, error) {
	compressed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", trace.Wrap(err, "decoding base64")
	}

	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return "", trace.Wrap(err, "creating gzip reader")
	}
	defer gz.Close()

	out, err := io.ReadAll(gz)
	return string(out), trace.Wrap(err)
}

func trimImageTag(image string) string {
	i := strings.LastIndex(image, ":")
	if i < 0 {
		return image
	}
	return image[:i]
}

const (
	helmStorageVersion = "v1"
	helmSecretType     = "helm.sh/release.v1"
	helmReleaseStatus  = "deployed"
	helmReleaseRev     = 1
	helmReleaseName    = "teleport-kube-agent"
)

// helmRelease mirrors the JSON shape of helm's release.Release for
// release-storage. It intentionally declares only the fields Helm reads
// from storage for list/status/get-values/upgrade/uninstall interop.
type helmRelease struct {
	Name      string          `json:"name,omitempty"`
	Info      helmReleaseInfo `json:"info,omitempty"`
	Chart     json.RawMessage `json:"chart,omitempty"`
	Config    map[string]any  `json:"config,omitempty"`
	Manifest  string          `json:"manifest,omitempty"`
	Hooks     []*helmHook     `json:"hooks,omitempty"`
	Version   int             `json:"version,omitempty"`
	Namespace string          `json:"namespace,omitempty"`
}

type helmReleaseInfo struct {
	FirstDeployed time.Time `json:"first_deployed,omitempty"`
	LastDeployed  time.Time `json:"last_deployed,omitempty"`
	Deleted       time.Time `json:"deleted,omitempty"`
	Description   string    `json:"description,omitempty"`
	Status        string    `json:"status,omitempty"`
}

type helmHook struct {
	Name           string   `json:"name,omitempty"`
	Kind           string   `json:"kind,omitempty"`
	Path           string   `json:"path,omitempty"`
	Manifest       string   `json:"manifest,omitempty"`
	Events         []string `json:"events,omitempty"`
	Weight         int      `json:"weight,omitempty"`
	DeletePolicies []string `json:"delete_policies,omitempty"`
}

func helmReleaseSecret(release helmRelease) (*corev1.Secret, error) {
	encoded, err := encodeHelmRelease(release)
	if err != nil {
		return nil, trace.Wrap(err, "encoding helm release")
	}

	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("sh.helm.release.%s.%s.v%d", helmStorageVersion, release.Name, release.Version),
			Namespace: release.Namespace,
			Labels: map[string]string{
				"owner":      "helm",
				"name":       release.Name,
				"version":    strconv.Itoa(release.Version),
				"status":     helmReleaseStatus,
				"modifiedAt": strconv.FormatInt(release.Info.LastDeployed.Unix(), 10),
			},
		},
		Type: helmSecretType,
		Data: map[string][]byte{"release": []byte(encoded)},
	}, nil
}

func encodeHelmRelease(r helmRelease) (string, error) {
	raw, err := json.Marshal(r)
	if err != nil {
		return "", trace.Wrap(err, "marshaling release JSON")
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)

	if _, err := gz.Write(raw); err != nil {
		return "", trace.Wrap(err, "compressing release JSON")
	}

	if err := gz.Close(); err != nil {
		return "", trace.Wrap(err, "closing gzip writer")
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
