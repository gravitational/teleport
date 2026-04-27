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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

// TestHelmReleaseSecretRoundTrip confirms our manually-built release
// Secret exists in the same format helm's storage driver expects,
// and that key fields (name, namespace, version, status, manifest)
// survive the encoding.
func TestHelmReleaseSecretRoundTrip(t *testing.T) {
	t.Parallel()

	const manifestYAML = "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n"
	sec, err := helmReleaseSecret(helmRelease{
		Name:      "teleport-kube-agent",
		Namespace: "teleport-agent",
		Version:   1,
		Info:      helmReleaseInfo{Status: helmReleaseStatus, Description: "Install complete"},
		Manifest:  manifestYAML,
	})
	require.NoError(t, err)

	require.Equal(t, "sh.helm.release.v1.teleport-kube-agent.v1", sec.Name)
	require.Equal(t, "teleport-agent", sec.Namespace)
	require.Equal(t, corev1.SecretType("helm.sh/release.v1"), sec.Type)
	require.Equal(t, "helm", sec.Labels["owner"])
	require.Equal(t, "deployed", sec.Labels["status"])

	// data["release"] is base64(gzip(JSON(release))). Decode it the same
	// way helm's storage driver does.
	encoded := sec.Data["release"]
	require.NotEmpty(t, encoded)

	gzipped, err := base64.StdEncoding.DecodeString(string(encoded))
	require.NoError(t, err)

	gz, err := gzip.NewReader(bytes.NewReader(gzipped))
	require.NoError(t, err)

	var decoded helmRelease
	require.NoError(t, json.NewDecoder(gz).Decode(&decoded))

	require.Equal(t, "teleport-kube-agent", decoded.Name)
	require.Equal(t, "teleport-agent", decoded.Namespace)
	require.Equal(t, 1, decoded.Version)
	require.Equal(t, "deployed", decoded.Info.Status)
	require.Equal(t, manifestYAML, decoded.Manifest)
}

// TestManifestsEmitsHelmInterop confirms the composer's output includes
// the helm release-storage Secret and that every Kubernetes resource carries
// the helm-ownership labels and annotations.
func TestManifestsEmitsHelmInterop(t *testing.T) {
	t.Parallel()

	objs, err := Manifests(baseOpts())
	require.NoError(t, err)

	// Find the helm release Secret.
	var releaseSec *corev1.Secret
	for _, o := range objs {
		s, ok := o.(*corev1.Secret)
		if !ok {
			continue
		}
		if strings.HasPrefix(s.Name, "sh.helm.release.") {
			releaseSec = s
			break
		}
	}
	require.NotNil(t, releaseSec)
	require.Equal(t, "sh.helm.release.v1.teleport-kube-agent.v1", releaseSec.Name)
	require.Equal(t, corev1.SecretType("helm.sh/release.v1"), releaseSec.Type)

	// Every other object should carry the helm ownership metadata.
	for _, o := range objs {
		if o == releaseSec {
			continue
		}
		require.Equal(t, "Helm", o.GetLabels()["app.kubernetes.io/managed-by"])
		require.Equal(t, "teleport-kube-agent", o.GetAnnotations()["meta.helm.sh/release-name"])
		require.Equal(t, baseOpts().Namespace, o.GetAnnotations()["meta.helm.sh/release-namespace"])
	}
}

// TestReleaseSecretCarriesChart confirms the embedded chart bytes survive
// the encode/decode round-trip, with non-empty Templates and Metadata.
func TestReleaseSecretCarriesChart(t *testing.T) {
	t.Parallel()

	objs, err := Manifests(baseOpts())
	require.NoError(t, err)

	var releaseSec *corev1.Secret
	for _, o := range objs {
		s, ok := o.(*corev1.Secret)
		if ok && strings.HasPrefix(s.Name, "sh.helm.release.") {
			releaseSec = s
			break
		}
	}
	require.NotNil(t, releaseSec)

	gzipped, err := base64.StdEncoding.DecodeString(string(releaseSec.Data["release"]))
	require.NoError(t, err)
	gz, err := gzip.NewReader(bytes.NewReader(gzipped))
	require.NoError(t, err)

	// Decode partially. Chart stays raw so we can inspect it via a
	// looser-typed shape.
	var decoded struct {
		Chart struct {
			Metadata struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"metadata"`
			Templates []struct {
				Name string `json:"name"`
				Data []byte `json:"data"`
			} `json:"templates"`
		} `json:"chart"`
	}
	require.NoError(t, json.NewDecoder(gz).Decode(&decoded))

	require.Equal(t, "teleport-kube-agent", decoded.Chart.Metadata.Name)
	require.NotEmpty(t, decoded.Chart.Metadata.Version)
	require.NotEmpty(t, decoded.Chart.Templates)

	var found bool
	for _, tmpl := range decoded.Chart.Templates {
		if strings.HasSuffix(tmpl.Name, "templates/statefulset.yaml") {
			found = true
			require.NotEmpty(t, tmpl.Data)
			break
		}
	}
	require.True(t, found)
}
