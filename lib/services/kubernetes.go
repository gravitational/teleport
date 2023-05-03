/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/utils"
)

// KubernetesClusterGetter defines interface for fetching kubernetes cluster resources.
type KubernetesClusterGetter interface {
	// GetKubernetesClusters returns all kubernetes cluster resources.
	GetKubernetesClusters(context.Context) ([]types.KubeCluster, error)
	// GetKubernetesCluster returns the specified kubernetes cluster resource.
	GetKubernetesCluster(ctx context.Context, name string) (types.KubeCluster, error)
}

// KubernetesServerGetter defines interface for fetching kubernetes server resources.
type KubernetesServerGetter interface {
	// GetKubernetesServers returns all kubernetes server resources.
	GetKubernetesServers(context.Context) ([]types.KubeServer, error)
}

// Kubernetes defines an interface for managing kubernetes clusters resources.
type Kubernetes interface {
	// KubernetesGetter provides methods for fetching kubernetes resources.
	KubernetesClusterGetter
	// CreateKubernetesCluster creates a new kubernetes cluster resource.
	CreateKubernetesCluster(context.Context, types.KubeCluster) error
	// UpdateKubernetesCluster updates an existing kubernetes cluster resource.
	UpdateKubernetesCluster(context.Context, types.KubeCluster) error
	// DeleteKubernetesCluster removes the specified kubernetes cluster resource.
	DeleteKubernetesCluster(ctx context.Context, name string) error
	// DeleteAllKubernetesClusters removes all kubernetes resources.
	DeleteAllKubernetesClusters(context.Context) error
}

// MarshalKubeServer marshals the KubeServer resource to JSON.
func MarshalKubeServer(kubeServer types.KubeServer, opts ...MarshalOption) ([]byte, error) {
	if err := kubeServer.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch server := kubeServer.(type) {
	case *types.KubernetesServerV3:
		if !cfg.PreserveResourceID {
			copy := *server
			copy.SetResourceID(0)
			server = &copy
		}
		return utils.FastMarshal(server)
	default:
		return nil, trace.BadParameter("unsupported kube server resource %T", server)
	}
}

// UnmarshalKubeServer unmarshals KubeServer resource from JSON.
func UnmarshalKubeServer(data []byte, opts ...MarshalOption) (types.KubeServer, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing kube server data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V3:
		var s types.KubernetesServerV3
		if err := utils.FastUnmarshal(data, &s); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			s.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			s.SetExpiry(cfg.Expires)
		}
		return &s, nil
	}
	return nil, trace.BadParameter("unsupported kube server resource version %q", h.Version)
}

// MarshalKubeCluster marshals the KubeCluster resource to JSON.
func MarshalKubeCluster(kubeCluster types.KubeCluster, opts ...MarshalOption) ([]byte, error) {
	if err := kubeCluster.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch cluster := kubeCluster.(type) {
	case *types.KubernetesClusterV3:
		if !cfg.PreserveResourceID {
			copy := *cluster
			copy.SetResourceID(0)
			cluster = &copy
		}
		return utils.FastMarshal(cluster)
	default:
		return nil, trace.BadParameter("unsupported kube cluster resource %T", cluster)
	}
}

// UnmarshalKubeCluster unmarshals KubeCluster resource from JSON.
func UnmarshalKubeCluster(data []byte, opts ...MarshalOption) (types.KubeCluster, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing kube cluster data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V3:
		var s types.KubernetesClusterV3
		if err := utils.FastUnmarshal(data, &s); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			s.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			s.SetExpiry(cfg.Expires)
		}
		return &s, nil
	}
	return nil, trace.BadParameter("unsupported kube cluster resource version %q", h.Version)
}

const (
	// labelTeleportKubeClusterName is the label key containing the kubernetes cluster name override.
	labelTeleportKubeClusterName = types.TeleportNamespace + "/kubernetes-name"
)

// setKubeName modifies the types.Metadata argument in place, setting the kubernetes cluster name.
// The name is calculated based on nameParts arguments which are joined by hyphens "-".
// If the kube_cluster name override label is present (setKubeName), it will replace the *first* name part.
func setKubeName(meta types.Metadata, firstNamePart string, extraNameParts ...string) types.Metadata {
	nameParts := append([]string{firstNamePart}, extraNameParts...)

	// apply override
	if override, found := meta.Labels[labelTeleportKubeClusterName]; found && override != "" {
		nameParts[0] = override
	}

	meta.Name = strings.Join(nameParts, "-")

	return meta
}

// NewKubeClusterFromAzureAKS creates a kube_cluster resource from an AKSCluster.
func NewKubeClusterFromAzureAKS(cluster *azure.AKSCluster) (types.KubeCluster, error) {
	labels := labelsFromAzureKubeCluster(cluster)
	return types.NewKubernetesClusterV3(
		setKubeName(types.Metadata{
			Description: fmt.Sprintf("Azure AKS cluster %q in %v",
				cluster.Name,
				cluster.Location),
			Labels: labels,
		}, cluster.Name),
		types.KubernetesClusterSpecV3{
			Azure: types.KubeAzure{
				ResourceName:   cluster.Name,
				ResourceGroup:  cluster.GroupName,
				TenantID:       cluster.TenantID,
				SubscriptionID: cluster.SubscriptionID,
			},
		})
}

// labelsFromAzureKubeCluster creates kube cluster labels.
func labelsFromAzureKubeCluster(cluster *azure.AKSCluster) map[string]string {
	labels := azureTagsToLabels(cluster.Tags)
	labels[types.OriginLabel] = types.OriginCloud
	labels[types.CloudLabel] = types.CloudAzure
	labels[labelRegion] = cluster.Location

	labels[labelResourceGroup] = cluster.GroupName
	labels[labelSubscriptionID] = cluster.SubscriptionID
	return labels
}

// NewKubeClusterFromGCPGKE creates a kube_cluster resource from an GKE cluster.
func NewKubeClusterFromGCPGKE(cluster gcp.GKECluster) (types.KubeCluster, error) {
	return types.NewKubernetesClusterV3(
		setKubeName(types.Metadata{
			Description: getOrSetDefaultGCPDescription(cluster),
			Labels:      labelsFromGCPKubeCluster(cluster),
		}, cluster.Name),
		types.KubernetesClusterSpecV3{
			GCP: types.KubeGCP{
				Name:      cluster.Name,
				ProjectID: cluster.ProjectID,
				Location:  cluster.Location,
			},
		})
}

// getOrSetDefaultGCPDescription gets the default GKE cluster description if available,
// otherwise returns a default one.
func getOrSetDefaultGCPDescription(cluster gcp.GKECluster) string {
	if len(cluster.Description) > 0 {
		return cluster.Description
	}
	return fmt.Sprintf("GKE cluster %q in %s",
		cluster.Name,
		cluster.Location)
}

// labelsFromGCPKubeCluster creates kube cluster labels.
func labelsFromGCPKubeCluster(cluster gcp.GKECluster) map[string]string {
	labels := maps.Clone(cluster.Labels)
	labels[types.OriginLabel] = types.OriginCloud
	labels[types.CloudLabel] = types.CloudGCP
	labels[labelLocation] = cluster.Location

	labels[labelProjectID] = cluster.ProjectID
	return labels
}

// NewKubeClusterFromAWSEKS creates a kube_cluster resource from an EKS cluster.
func NewKubeClusterFromAWSEKS(cluster *eks.Cluster) (types.KubeCluster, error) {
	parsedARN, err := arn.Parse(aws.StringValue(cluster.Arn))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	labels := labelsFromAWSKubeCluster(cluster, parsedARN)

	return types.NewKubernetesClusterV3(
		setKubeName(types.Metadata{
			Description: fmt.Sprintf("AWS EKS cluster %q in %s",
				aws.StringValue(cluster.Name),
				parsedARN.Region),
			Labels: labels,
		}, aws.StringValue(cluster.Name)),
		types.KubernetesClusterSpecV3{
			AWS: types.KubeAWS{
				Name:      aws.StringValue(cluster.Name),
				AccountID: parsedARN.AccountID,
				Region:    parsedARN.Region,
			},
		})
}

// labelsFromAWSKubeCluster creates kube cluster labels.
func labelsFromAWSKubeCluster(cluster *eks.Cluster, parsedARN arn.ARN) map[string]string {
	labels := awsEKSTagsToLabels(cluster.Tags)
	labels[types.OriginLabel] = types.OriginCloud
	labels[types.CloudLabel] = types.CloudAWS
	labels[labelRegion] = parsedARN.Region

	labels[labelAccountID] = parsedARN.AccountID
	return labels
}

// awsEKSTagsToLabels converts AWS tags to a labels map.
func awsEKSTagsToLabels(tags map[string]*string) map[string]string {
	labels := make(map[string]string)
	for key, val := range tags {
		if types.IsValidLabelKey(key) {
			labels[key] = aws.StringValue(val)
		} else {
			log.Debugf("Skipping EKS tag %q, not a valid label key.", key)
		}
	}
	return labels
}

const (
	// labelProjectID is the label key for GCP project ID.
	labelProjectID = "project-id"
	// labelLocation is the label key for GCP location.
	labelLocation = "location"
)
