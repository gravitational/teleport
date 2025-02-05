//go:build unix

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
	"time"

	"github.com/gravitational/trace"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/mount"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

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
func (a *KubernetesAttestor) Attest(ctx context.Context, pid int) (*workloadidentityv1pb.WorkloadAttrsKubernetes, error) {
	a.log.DebugContext(ctx, "Starting Kubernetes workload attestation", "pid", pid)

	podID, containerID, err := a.getContainerAndPodID(pid)
	if err != nil {
		return nil, trace.Wrap(err, "determining pod and container ID")
	}
	a.log.DebugContext(ctx, "Found pod and container ID", "pod_id", podID, "container_id", containerID)

	pod, err := a.getPodForID(ctx, podID)
	if err != nil {
		return nil, trace.Wrap(err, "finding pod by ID")
	}
	a.log.DebugContext(ctx, "Found pod", "pod_name", pod.Name)

	att := &workloadidentityv1pb.WorkloadAttrsKubernetes{
		Attested:       true,
		Namespace:      pod.Namespace,
		ServiceAccount: pod.Spec.ServiceAccountName,
		PodName:        pod.Name,
		PodUid:         string(pod.UID),
		Labels:         pod.Labels,
	}
	a.log.DebugContext(ctx, "Finished Kubernetes workload attestation", "attestation", att)
	return att, nil
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
	// For cgroup v2, we expect a single mount. But for cgroup v1, there will
	// be one mount per subsystem, but regardless, they will all contain the
	// same container ID/pod ID.
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
//
// Note: this is a fairly naive implementation, we may need to make further
// improvements to account for other distributions of Kubernetes.
func mountpointSourceToContainerAndPodID(source string) (podID string, containerID string, err error) {
	// From the mount, we need to extract the container ID and pod ID.
	// Unfortunately this process can be a little fragile, as the format of
	// the mountpoint varies across Kubernetes implementations.
	// There's a collection of real world mountfiles in testdata/mountfile.

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

// kubeletClient is a HTTP client for the Kubelet API
type kubeletClient struct {
	cfg    KubeletClientConfig
	getEnv func(string) string
}

func newKubeletClient(cfg KubeletClientConfig) *kubeletClient {
	return &kubeletClient{
		cfg:    cfg,
		getEnv: os.Getenv,
	}
}

type roundTripperFn func(req *http.Request) (*http.Response, error)

func (f roundTripperFn) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func (c *kubeletClient) httpClient() (url.URL, *http.Client, error) {
	host := c.getEnv(nodeNameEnv)

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
		// 10 seconds is fairly generous given that we're expecting to talk to
		// kubelet on the same physical machine.
		Timeout: 10 * time.Second,
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
