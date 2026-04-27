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
	"fmt"
	"strconv"
	"time"

	"github.com/gravitational/trace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	helmStorageVersion = "v1"
	helmSecretType     = "helm.sh/release.v1"
	helmReleaseStatus  = "deployed"
	helmReleaseRev     = 1
)

// helmRelease mirrors the JSON shape of helm's release.Release for
// release-storage. Importing helm is intentionally avoided to allow
// DCE. The helmRelease declares only the fields the storage driver serializes.
//
// Chart is a json.RawMessage so the pre-serialized chart blob (emitted
// by the generator into chartJSON) lands in the output verbatim,
// without marshaling again at runtime.
type helmRelease struct {
	Name      string          `json:"name"`
	Info      helmReleaseInfo `json:"info"`
	Chart     json.RawMessage `json:"chart,omitempty"`
	Config    map[string]any  `json:"config,omitempty"`
	Manifest  string          `json:"manifest"`
	Version   int             `json:"version"`
	Namespace string          `json:"namespace"`
}

type helmReleaseInfo struct {
	FirstDeployed time.Time `json:"first_deployed"`
	LastDeployed  time.Time `json:"last_deployed"`
	Deleted       time.Time `json:"deleted"`
	Description   string    `json:"description"`
	Status        string    `json:"status"`
}

// helmReleaseSecret builds the release-storage Secret helm reads on
// uninstall/upgrade/list. The Secret must live in the release namespace.
func helmReleaseSecret(release helmRelease) (*corev1.Secret, error) {
	encoded, err := encodeHelmRelease(release)
	if err != nil {
		return nil, trace.Wrap(err, "encoding helm release")
	}

	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
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

// encodeHelmRelease produces the base64(gzip(JSON(release))) blob the
// helm storage driver expects in Secret.Data["release"].
func encodeHelmRelease(r helmRelease) (string, error) {
	raw, err := json.Marshal(r)
	if err != nil {
		return "", trace.Wrap(err, "marshaling release JSON")
	}

	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return "", trace.Wrap(err, "creating gzip writer")
	}

	if _, err := gz.Write(raw); err != nil {
		return "", trace.Wrap(err, "compressing release JSON")
	}

	if err := gz.Close(); err != nil {
		return "", trace.Wrap(err, "closing gzip writer")
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// stampHelmMetadata adds the labels and annotations helm uses to
// recognize a resource as belonging to a release. Without these, `helm
// upgrade` refuses to adopt the resource.
func stampHelmMetadata(obj metav1.Object, releaseName, releaseNamespace, chartLabel string) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}

	labels["app.kubernetes.io/managed-by"] = "Helm"
	labels["helm.sh/chart"] = chartLabel
	obj.SetLabels(labels)

	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	annotations["meta.helm.sh/release-name"] = releaseName
	annotations["meta.helm.sh/release-namespace"] = releaseNamespace
	obj.SetAnnotations(annotations)
}
