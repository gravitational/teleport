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
	"cmp"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/mount"
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
	// ContainerName is the individual container that the PID resolved to within
	// the pod.
	ContainerName string
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
			slog.String("container_name", a.ContainerName),
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
	Kubelet KubeletClientConfig `yaml:"kubelet"`
}

// KubernetesAttestor attests a workload to a Kubernetes pod.
type KubernetesAttestor struct {
	kubeletClient *kubeletClient
}

// NewKubernetesAttestor creates a new KubernetesAttestor.
func NewKubernetesAttestor(cfg KubernetesAttestorConfig) *KubernetesAttestor {
	return &KubernetesAttestor{
		kubeletClient: &kubeletClient{
			cfg: cfg.Kubelet,
		},
	}
}

// Attest resolves the Kubernetes pod information from the
// PID of the workload.
//
// From what I can tell, there's two common ways of doing this:
// - /proc/<pid>/mountinfo
// - /proc/<pid>/cgroup
//
// This implementation leverages /proc/<pid>/mountinfo
//
// Requires `hostPID: true` so we can see the /proc of other pods.
//
// We can then query the kubelet api to find the pod that this corresponds to.
func (a *KubernetesAttestor) Attest(ctx context.Context, pid int) (KubernetesAttestation, error) {
	podID, containerID, err := a.getContainerAndPodID(pid)
	if err != nil {
		return KubernetesAttestation{}, trace.Wrap(err, "determining pod and container ID")
	}

	pod, err := a.getPodForID(ctx, podID)
	if err != nil {
		return KubernetesAttestation{}, trace.Wrap(err, "finding pod by ID")
	}

	var container *v1.ContainerStatus
	for _, c := range pod.Status.ContainerStatuses {
		if c.ContainerID == containerID {
			container = &c
			break
		}
	}
	if container == nil {
		for _, c := range pod.Status.InitContainerStatuses {
			if c.ContainerID == containerID {
				container = &c
				break
			}
		}
	}
	if container == nil {
		return KubernetesAttestation{}, trace.BadParameter("container %q not found in pod %q", containerID, pod.Name)
	}

	return KubernetesAttestation{
		Attested:       true,
		Namespace:      pod.Namespace,
		ServiceAccount: pod.Spec.ServiceAccountName,
		ContainerName:  container.Name,
		PodName:        pod.Name,
		PodUID:         string(pod.UID),
		Labels:         pod.Labels,
	}, nil
}

// getContainerAndPodID retrieves the container ID and pod ID for the provided
// PID.
func (a *KubernetesAttestor) getContainerAndPodID(pid int) (podID string, containerID string, err error) {
	info, err := mount.ParseMountInfo(
		path.Join("/proc", strconv.Itoa(pid), "mountinfo"),
	)
	if err != nil {
		return "", "", trace.Wrap(
			err, "parsing mountinfo",
		)
	}

	// Find the cgroup or cgroupv2 mount
	// TODO(noah): Is it possible for there to be multiple cgroup mounts?
	// If so, how should we handle.
	// I imagine with cgroup v1, we get one mount per controller, but all should
	// be fairly equivelant.
	var cgroupMount mount.MountInfo
	for _, m := range info {
		if m.FsType == "cgroup" || m.FsType == "cgroup2" {
			cgroupMount = m
			break
		}
	}

	podID, containerID, err = mountpointSourceToContainerAndPodID(
		cgroupMount.Source,
	)
	if err != nil {
		return "", "", trace.Wrap(
			err, "parsing cgroup mount (source: %q)", cgroupMount.Source,
		)
	}
	return podID, containerID, nil
}

var (
	// TODO: This is a super naive implementation that may only work in my
	// cluster. This needs revisiting before merging.

	// A container ID is usually a 64 character hex string, so this regex just
	// selects for that.
	containerIDRegex = regexp.MustCompile(`(?P<containerID>[[:xdigit:]]{64})`)
	// A pod ID is usually a UUID prefaced with "pod".
	// There are two main cgroup drivers:
	// - systemd , the dashes are replaced with underscores
	// - cgroupfs, the dashes are kept.
	podIDRegex = regexp.MustCompile(`pod(?P<podID>[[:xdigit:]]{8}[_-][[:xdigit:]]{4}[_-][[:xdigit:]]{4}[_-][[:xdigit:]]{4}[_-][[:xdigit:]]{12})`)
)

// mountpointSourceToContainerAndPodID takes the source of the cgroup mountpoint
// and extracts the container ID and pod ID from it.
// TODO: This is a super naive implementation that may only work in my
// cluster. This needs revisiting before merging.
func mountpointSourceToContainerAndPodID(source string) (podID string, containerID string, err error) {
	// From the mount, we need to extract the container ID and pod ID.
	// Unfortunately this process can be a little fragile, as the format of
	// the mountpoint varies across Kubernetes implementations.
	// The format of the mountpoint varies, but can look something like:
	// /kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod30e5e887_5bea_42fb_a256_ec9d6a76efc7.slice/cri-containerd-22985f2d7e6472530eabf5ed449b0c84899f38f60e778cbee5c1642f1b24cda6.scope

	matches := containerIDRegex.FindStringSubmatch(source)
	if len(matches) != 2 {
		return "", "", trace.BadParameter(
			"expected 2 matches searching for container ID but found %d",
			len(matches),
		)
	}
	containerID = matches[1]
	if containerID == "" {
		return "", "", trace.BadParameter(
			"source does not contain container ID",
		)
	}

	matches = podIDRegex.FindStringSubmatch(source)
	if len(matches) != 2 {
		return "", "", trace.BadParameter(
			"expected 2 matches searching for pod ID but found %d",
			len(matches),
		)
	}
	podID = matches[1]
	if podID == "" {
		return "", "", trace.BadParameter(
			"source does not contain pod ID",
		)
	}

	// When using the `systemd` cgroup driver, the dashes are replaced with
	// underscores. So let's correct that.
	podID = strings.ReplaceAll(podID, "_", "-")

	return podID, containerID, nil
}

// getPodForID retrieves the pod information for the provided pod ID.
// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/server/server.go#L371
//
// Internally, this may retry a few times if not found. This accounts for any
// potential raciness.
func (a *KubernetesAttestor) getPodForID(ctx context.Context, podID string) (*v1.Pod, error) {
	// TODO: Retry w/ short backoff if not found.
	pods, err := a.kubeletClient.ListAllPods(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "listing all pods")
	}
	for _, pod := range pods.Items {
		if string(pod.UID) == podID {
			return &pod, nil
		}
	}
	return nil, trace.NotFound("pod %q not found", podID)
}

const (
	// nodeNameEnv is used to inject the current nodes name via the downward API.
	// This provides a hostname for the kubelet client to use.
	nodeNameEnv                = "TELEPORT_NODE_NAME"
	defaultServiceAccountToken = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

// KubeletClientConfig holds the configuration for the Kubelet client
// used to query the Kubelet API for workload attestation.
type KubeletClientConfig struct {
	// ReadOnlyPort is the port on which the Kubelet API is exposed for
	// read-only operations. This is mutually exclusive with SecurePort.
	// This is primarily left for legacy support - since Kubernetes 1.16, the
	// read-only port is disabled by default.
	ReadOnlyPort int `yaml:"read_only_port"`
	// SecurePort specifies the secure port on which the Kubelet API is exposed.
	// If unspecified, this defaults to `10250`. This is mutually exclusive
	// with ReadOnlyPort.
	SecurePort int `yaml:"secure_port"`

	// TokenPath is the path to the token file used to authenticate with the
	// Kubelet API. Defaults to `/var/run/secrets/kubernetes.io/serviceaccount/token`.
	TokenPath string `yaml:"token_path"`
}

// kubeletClient is a HTTP client for the Kubelet API
type kubeletClient struct {
	cfg KubeletClientConfig
}

func (c *kubeletClient) ListAllPods(ctx context.Context) (*v1.PodList, error) {
	host := os.Getenv(nodeNameEnv)
	port := cmp.Or(c.cfg.SecurePort, 10250)
	client := &http.Client{
		Transport: http.DefaultTransport,
	}
	reqUrl := url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
		Path:   "/pods",
	}

	// TODO: Support for read only port...

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqUrl.String(), nil)
	if err != nil {
		return nil, trace.Wrap(err, "creating request")
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, trace.Wrap(err, "performing request")
	}
	defer res.Body.Close()

	out := &v1.PodList{}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return nil, trace.Wrap(err, "decoding response")
	}
	return out, nil
}
