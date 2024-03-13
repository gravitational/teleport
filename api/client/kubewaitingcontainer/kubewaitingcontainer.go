// Copyright 2023 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubewaitingcontainer

import (
	"context"

	"github.com/gravitational/trace"

	kubewaitingcontainerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	"github.com/gravitational/teleport/api/types/kubewaitingcontainer"
	convert "github.com/gravitational/teleport/api/types/kubewaitingcontainer/convert/v1"
)

// Client is a KubeWaitingContainers client.
type Client struct {
	grpcClient kubewaitingcontainerpb.KubeWaitingContainersServiceClient
}

// NewClient creates a new KubeWaitingContainers client.
func NewClient(grpcClient kubewaitingcontainerpb.KubeWaitingContainersServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// KubeWaitingContainerRequest is a request for a specific Kubernetes
// ephemeral container that is waiting to be created until moderated
// session conditions are met.
type KubeWaitingContainerRequest struct {
	// Username is the Teleport user that attempted to create the container
	Username string `json:"username" yaml:"username"`
	// Cluster is the Kubernetes cluster of the container
	Cluster string `json:"cluster" yaml:"cluster"`
	// Namespace is the Kubernetes namespace of the container
	Namespace string `json:"namespace" yaml:"namespace"`
	// PodName is the parent pod of the container
	PodName string `json:"pod_name" yaml:"pod_name"`
	// ContainerName is the name of the container
	ContainerName string `json:"container_name" yaml:"container_name"`
}

// ListKubernetesWaitingContainers lists Kubernetes ephemeral
// containers that are waiting to be created until moderated
// session conditions are met.
func (c *Client) ListKubernetesWaitingContainers(ctx context.Context, pageSize int, pageToken string) ([]*kubewaitingcontainer.KubeWaitingContainer, string, error) {
	resp, err := c.grpcClient.ListKubernetesWaitingContainers(ctx, &kubewaitingcontainerpb.ListKubernetesWaitingContainersRequest{
		PageSize:  int32(pageSize),
		PageToken: pageToken,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	converted := make([]*kubewaitingcontainer.KubeWaitingContainer, len(resp.WaitingContainers))
	for i, msg := range resp.WaitingContainers {
		out, err := convert.FromProto(msg)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		converted[i] = out
	}

	return converted, resp.NextPageToken, nil
}

// GetKubernetesWaitingContainer returns a Kubernetes ephemeral
// container that is waiting to be created until moderated
// session conditions are met.
func (c *Client) GetKubernetesWaitingContainer(ctx context.Context, req KubeWaitingContainerRequest) (*kubewaitingcontainer.KubeWaitingContainer, error) {
	if req.Username == "" {
		return nil, trace.BadParameter("missing Username")
	}
	if req.Cluster == "" {
		return nil, trace.BadParameter("missing Cluster")
	}
	if req.Namespace == "" {
		return nil, trace.BadParameter("missing Namespace")
	}
	if req.PodName == "" {
		return nil, trace.BadParameter("missing PodName")
	}
	if req.ContainerName == "" {
		return nil, trace.BadParameter("missing ContainerName")
	}

	resp, err := c.grpcClient.GetKubernetesWaitingContainer(ctx, &kubewaitingcontainerpb.GetKubernetesWaitingContainerRequest{
		Username:      req.Username,
		Cluster:       req.Cluster,
		Namespace:     req.Namespace,
		PodName:       req.PodName,
		ContainerName: req.ContainerName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := convert.FromProto(resp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

// CreateKubernetesWaitingContainer creates a Kubernetes ephemeral
// container that is waiting to be created until moderated
// session conditions are met.
func (c *Client) CreateKubernetesWaitingContainer(ctx context.Context, waitingPod *kubewaitingcontainer.KubeWaitingContainer) (*kubewaitingcontainer.KubeWaitingContainer, error) {
	resp, err := c.grpcClient.CreateKubernetesWaitingContainer(ctx, &kubewaitingcontainerpb.CreateKubernetesWaitingContainerRequest{
		WaitingContainer: convert.ToProto(waitingPod),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := convert.FromProto(resp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

// DeleteKubernetesWaitingContainer returns a Kubernetes ephemeral
// container that is waiting to be created until moderated
// session conditions are met.
func (c *Client) DeleteKubernetesWaitingContainer(ctx context.Context, req KubeWaitingContainerRequest) error {
	if req.Username == "" {
		return trace.BadParameter("missing Username")
	}
	if req.Cluster == "" {
		return trace.BadParameter("missing Cluster")
	}
	if req.Namespace == "" {
		return trace.BadParameter("missing Namespace")
	}
	if req.PodName == "" {
		return trace.BadParameter("missing PodName")
	}
	if req.ContainerName == "" {
		return trace.BadParameter("missing ContainerName")
	}

	_, err := c.grpcClient.DeleteKubernetesWaitingContainer(ctx, &kubewaitingcontainerpb.DeleteKubernetesWaitingContainerRequest{
		Username:      req.Username,
		Cluster:       req.Cluster,
		Namespace:     req.Namespace,
		PodName:       req.PodName,
		ContainerName: req.ContainerName,
	})

	return trace.Wrap(err)
}
