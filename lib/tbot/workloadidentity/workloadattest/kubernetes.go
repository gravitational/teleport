/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package workloadattest

import (
	"log/slog"

	"github.com/gravitational/trace"
)

// KubernetesAttestation holds the Kubernetes pod information retrieved from
// the workload attestation process.
type KubernetesAttestation struct {
	// Attested is true if the PID was successfully attested to a Kubernetes
	// pod. This indicates the validity of the rest of the fields.
	Attested bool
	// Namespace is the namespace of the pod.
	Namespace string
	// ServiceAccount is the service account of the pod.
	ServiceAccount string
	// PodName is the name of the pod.
	PodName string
	// PodUID is the UID of the pod.
	PodUID string
	// Labels is a map of labels on the pod.
	Labels map[string]string
}

// LogValue implements slog.LogValue to provide a nicely formatted set of
// log keys for a given attestation.
func (a KubernetesAttestation) LogValue() slog.Value {
	values := []slog.Attr{
		slog.Bool("attested", a.Attested),
	}
	if a.Attested {
		labels := []slog.Attr{}
		for k, v := range a.Labels {
			labels = append(labels, slog.String(k, v))
		}
		values = append(values,
			slog.String("namespace", a.Namespace),
			slog.String("service_account", a.ServiceAccount),
			slog.String("pod_name", a.PodName),
			slog.String("pod_uid", a.PodUID),
			slog.Attr{
				Key:   "labels",
				Value: slog.GroupValue(labels...),
			},
		)
	}
	return slog.GroupValue(values...)
}

// KubernetesAttestorConfig holds the configuration for the KubernetesAttestor.
type KubernetesAttestorConfig struct {
	// Enabled is true if the KubernetesAttestor is enabled. If false,
	// Kubernetes attestation will not be attempted.
	Enabled bool                `yaml:"enabled"`
	Kubelet KubeletClientConfig `yaml:"kubelet,omitempty"`
}

func (c *KubernetesAttestorConfig) CheckAndSetDefaults() error {
	if !c.Enabled {
		return nil
	}
	return trace.Wrap(c.Kubelet.CheckAndSetDefaults(), "validating kubelet")
}

const (
	// nodeNameEnv is used to inject the current nodes name via the downward API.
	// This provides a hostname for the kubelet client to use.
	nodeNameEnv                    = "TELEPORT_NODE_NAME"
	defaultServiceAccountTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	defaultCAPath                  = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	defaultSecurePort              = 10250
)

// KubeletClientConfig holds the configuration for the Kubelet client
// used to query the Kubelet API for workload attestation.
type KubeletClientConfig struct {
	// ReadOnlyPort is the port on which the Kubelet API is exposed for
	// read-only operations. This is mutually exclusive with SecurePort.
	// This is primarily left for legacy support - since Kubernetes 1.16, the
	// read-only port is disabled by default.
	ReadOnlyPort int `yaml:"read_only_port,omitempty"`
	// SecurePort specifies the secure port on which the Kubelet API is exposed.
	// If unspecified, this defaults to `10250`. This is mutually exclusive
	// with ReadOnlyPort.
	SecurePort int `yaml:"secure_port,omitempty"`

	// TokenPath is the path to the token file used to authenticate with the
	// Kubelet API when using the secure port.
	// Defaults to `/var/run/secrets/kubernetes.io/serviceaccount/token`.
	TokenPath string `yaml:"token_path,omitempty"`
	// CAPath is the path to the CA file used to verify the certificate
	// presented by Kubelet when using the secure port.
	// Defaults to `/var/run/secrets/kubernetes.io/serviceaccount/ca.crt`.
	CAPath string `yaml:"ca_path,omitempty"`
	// SkipVerify is used to skip verification of the Kubelet's certificate when
	// using the secure port. If set, CAPath will be ignored.
	//
	// This is useful in scenarios where Kubelet has not been configured with a
	// valid certificate signed by the cluster CA. This is more common than
	// you'd think.
	SkipVerify bool `yaml:"skip_verify,omitempty"`
	// Anonymous is used to indicate that no authentication should be used
	// when connecting to the secure Kubelet API. If set, TokenPath will be
	// ignored.
	Anonymous bool `yaml:"anonymous,omitempty"`
}

// CheckAndSetDefaults checks the KubeletClientConfig for any invalid values
// and sets defaults where necessary.
func (c KubeletClientConfig) CheckAndSetDefaults() error {
	if c.ReadOnlyPort != 0 && c.SecurePort != 0 {
		return trace.BadParameter("readOnlyPort and securePort are mutually exclusive")
	}
	return nil
}
