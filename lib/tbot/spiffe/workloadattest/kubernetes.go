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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
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
	Kubelet KubeletClientConfig `yaml:"kubelet"`
}

func (c *KubernetesAttestorConfig) CheckAndSetDefaults() error {
	if !c.Enabled {
		return nil
	}
	return trace.Wrap(c.Kubelet.CheckAndSetDefaults(), "validating kubelet")
}

// KubernetesAttestor attests a workload to a Kubernetes pod.
//
// It requires:
//
// - `hostPID: true` so we can view the /proc of other pods.
// - `TELEPORT_MY_NODE_NAME` to be set to the node name of the current node.
// - A service account that allows it to query the Kubelet API.
//
// It roughly takes the following steps:
//  1. From the PID, determine the container ID and pod ID from the
//     /proc/<pid>/mountinfo file.
//  2. Makes a request to the Kubelet API to list all pods on the node.
//  3. Find the pod and container with the matching ID.
//  4. Convert the pod information to a KubernetesAttestation.
type KubernetesAttestor struct {
	kubeletClient *kubeletClient
	log           *slog.Logger
	// rootPath specifies the location of `/`. This allows overriding for tests.
	rootPath string
}

// NewKubernetesAttestor creates a new KubernetesAttestor.
func NewKubernetesAttestor(cfg KubernetesAttestorConfig, log *slog.Logger) *KubernetesAttestor {
	kubeletClient := newKubeletClient(cfg.Kubelet)
	return &KubernetesAttestor{
		kubeletClient: kubeletClient,
		log:           log,
	}
}

// Attest resolves the Kubernetes pod information from the
// PID of the workload.
func (a *KubernetesAttestor) Attest(ctx context.Context, pid int) (KubernetesAttestation, error) {
	podID, containerID, err := a.getContainerAndPodID(pid)
	if err != nil {
		return KubernetesAttestation{}, trace.Wrap(err, "determining pod and container ID")
	}
	a.log.DebugContext(ctx, "Found pod and container ID", "pod_id", podID, "container_id", containerID)

	pod, err := a.getPodForID(ctx, podID)
	if err != nil {
		return KubernetesAttestation{}, trace.Wrap(err, "finding pod by ID")
	}
	a.log.DebugContext(ctx, "Found pod", "pod_name", pod.Name)

	return KubernetesAttestation{
		Attested:       true,
		Namespace:      pod.Namespace,
		ServiceAccount: pod.Spec.ServiceAccountName,
		PodName:        pod.Name,
		PodUID:         string(pod.UID),
		Labels:         pod.Labels,
	}, nil
}

// getContainerAndPodID retrieves the container ID and pod ID for the provided
// PID.
func (a *KubernetesAttestor) getContainerAndPodID(pid int) (podID string, containerID string, err error) {
	info, err := mount.ParseMountInfo(
		path.Join(a.rootPath, "/proc", strconv.Itoa(pid), "mountinfo"),
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
		cgroupMount.Root,
	)
	if err != nil {
		return "", "", trace.Wrap(
			err, "parsing cgroup mount (root: %q)", cgroupMount.Root,
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
func (a *KubernetesAttestor) getPodForID(ctx context.Context, podID string) (*v1.Pod, error) {
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
	ReadOnlyPort int `yaml:"read_only_port"`
	// SecurePort specifies the secure port on which the Kubelet API is exposed.
	// If unspecified, this defaults to `10250`. This is mutually exclusive
	// with ReadOnlyPort.
	SecurePort int `yaml:"secure_port"`

	// TokenPath is the path to the token file used to authenticate with the
	// Kubelet API when using the secure port.
	// Defaults to `/var/run/secrets/kubernetes.io/serviceaccount/token`.
	TokenPath string `yaml:"token_path"`
	// CAPath is the path to the CA file used to verify the certificate
	// presented by Kubelet when using the secure port.
	// Defaults to `/var/run/secrets/kubernetes.io/serviceaccount/ca.crt`.
	CAPath string `yaml:"ca_path"`
	// SkipVerify is used to skip verification of the Kubelet's certificate when
	// using the secure port. If set, CAPath will be ignored.
	//
	// This is useful in scenarios where Kubelet has not been configured with a
	// valid certificate signed by the cluster CA. This is more common than
	// you'd think.
	SkipVerify bool `yaml:"skip_verify"`
	// Anonymous is used to indicate that no authentication should be used
	// when connecting to the secure Kubelet API. If set, TokenPath will be
	// ignored.
	Anonymous bool `yaml:"anonymous"`
}

func (c KubeletClientConfig) CheckAndSetDefaults() error {
	if c.ReadOnlyPort != 0 && c.SecurePort != 0 {
		return trace.BadParameter("readOnlyPort and securePort are mutually exclusive")
	}
	return nil
}

// kubeletClient is a HTTP client for the Kubelet API
type kubeletClient struct {
	cfg KubeletClientConfig
}

func newKubeletClient(cfg KubeletClientConfig) *kubeletClient {
	return &kubeletClient{
		cfg: cfg,
	}
}

type roundTripperFn func(req *http.Request) (*http.Response, error)

func (f roundTripperFn) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func (c *kubeletClient) httpClient() (url.URL, *http.Client, error) {
	host := os.Getenv(nodeNameEnv)

	if c.cfg.ReadOnlyPort != 0 {
		return url.URL{
			Scheme: "http",
			Host:   net.JoinHostPort(host, strconv.Itoa(c.cfg.ReadOnlyPort)),
		}, &http.Client{}, nil
	}

	port := cmp.Or(c.cfg.SecurePort, defaultSecurePort)

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{},
	}

	switch {
	case c.cfg.SkipVerify:
		transport.TLSClientConfig.InsecureSkipVerify = true
	default:
		caPath := cmp.Or(c.cfg.CAPath, defaultCAPath)
		certPool := x509.NewCertPool()
		caPEM, err := os.ReadFile(caPath)
		if err != nil {
			return url.URL{}, nil, trace.Wrap(err, "reading CA file %q", caPath)
		}
		if !certPool.AppendCertsFromPEM(caPEM) {
			return url.URL{}, nil, trace.BadParameter("failed to append CA cert from %q", caPath)
		}
		transport.TLSClientConfig.RootCAs = certPool
	}

	client := &http.Client{
		Transport: transport,
	}

	switch {
	case c.cfg.Anonymous:
	// Nothing to do
	case c.cfg.TokenPath != "":
		fallthrough
	default:
		tokenPath := cmp.Or(c.cfg.TokenPath, defaultServiceAccountTokenPath)
		token, err := os.ReadFile(tokenPath)
		if err != nil {
			return url.URL{}, nil, trace.Wrap(err, "reading token file %q", tokenPath)
		}
		client.Transport = roundTripperFn(func(req *http.Request) (*http.Response, error) {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
			return transport.RoundTrip(req)
		})
	}

	return url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
	}, client, nil
}

func (c *kubeletClient) ListAllPods(ctx context.Context) (*v1.PodList, error) {
	reqUrl, client, err := c.httpClient()
	if err != nil {
		return nil, trace.Wrap(err, "creating HTTP client")
	}
	reqUrl.Path = "/pods"

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
