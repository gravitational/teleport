/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package services

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go/aws"
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
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch server := kubeServer.(type) {
	case *types.KubernetesServerV3:
		if err := server.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoResourceID(cfg.PreserveResourceID, server))
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
		if cfg.Revision != "" {
			s.SetRevision(cfg.Revision)
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
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch cluster := kubeCluster.(type) {
	case *types.KubernetesClusterV3:
		if err := cluster.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoResourceID(cfg.PreserveResourceID, cluster))
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
		if cfg.Revision != "" {
			s.SetRevision(cfg.Revision)
		}
		if !cfg.Expires.IsZero() {
			s.SetExpiry(cfg.Expires)
		}
		return &s, nil
	}
	return nil, trace.BadParameter("unsupported kube cluster resource version %q", h.Version)
}

// setAWSKubeName modifies the types.Metadata in place, overriding the first
// part if the kube cluster override label for AWS is present, and setting the
// kube cluster name.
func setAWSKubeName(meta types.Metadata, firstNamePart string, extraNameParts ...string) types.Metadata {
	return setResourceName(types.AWSKubeClusterNameOverrideLabels, meta, firstNamePart, extraNameParts...)
}

// setAzureKubeName modifies the types.Metadata in place, overriding the first
// part if the AKS kube cluster override label is present, and setting the kube
// cluster name.
func setAzureKubeName(meta types.Metadata, firstNamePart string, extraNameParts ...string) types.Metadata {
	return setResourceName([]string{types.AzureKubeClusterNameOverrideLabel}, meta, firstNamePart, extraNameParts...)
}

// setGCPKubeName modifies the types.Metadata in place, overriding the first
// part if the GKE kube cluster override label is present, and setting the kube
// cluster name.
func setGCPKubeName(meta types.Metadata, firstNamePart string, extraNameParts ...string) types.Metadata {
	return setResourceName([]string{types.GCPKubeClusterNameOverrideLabel}, meta, firstNamePart, extraNameParts...)
}

// NewKubeClusterFromAzureAKS creates a kube_cluster resource from an AKSCluster.
func NewKubeClusterFromAzureAKS(cluster *azure.AKSCluster) (types.KubeCluster, error) {
	labels := labelsFromAzureKubeCluster(cluster)
	return types.NewKubernetesClusterV3(
		setAzureKubeName(types.Metadata{
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
	labels[types.CloudLabel] = types.CloudAzure
	labels[types.DiscoveryLabelRegion] = cluster.Location

	labels[types.DiscoveryLabelAzureResourceGroup] = cluster.GroupName
	labels[types.DiscoveryLabelAzureSubscriptionID] = cluster.SubscriptionID
	return labels
}

// NewKubeClusterFromGCPGKE creates a kube_cluster resource from an GKE cluster.
func NewKubeClusterFromGCPGKE(cluster gcp.GKECluster) (types.KubeCluster, error) {
	return types.NewKubernetesClusterV3(
		setGCPKubeName(types.Metadata{
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
	labels := make(map[string]string)
	maps.Copy(labels, cluster.Labels)
	labels[types.CloudLabel] = types.CloudGCP
	labels[types.DiscoveryLabelGCPLocation] = cluster.Location

	labels[types.DiscoveryLabelGCPProjectID] = cluster.ProjectID
	return labels
}

// NewKubeClusterFromAWSEKS creates a kube_cluster resource from an EKS cluster.
func NewKubeClusterFromAWSEKS(clusterName, clusterArn string, tags map[string]*string) (types.KubeCluster, error) {
	parsedARN, err := arn.Parse(clusterArn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	labels := labelsFromAWSKubeClusterTags(tags, parsedARN)

	return types.NewKubernetesClusterV3(
		setAWSKubeName(types.Metadata{
			Description: fmt.Sprintf("AWS EKS cluster %q in %s",
				clusterName,
				parsedARN.Region),
			Labels: labels,
		}, clusterName),
		types.KubernetesClusterSpecV3{
			AWS: types.KubeAWS{
				Name:      clusterName,
				AccountID: parsedARN.AccountID,
				Region:    parsedARN.Region,
			},
		})
}

// labelsFromAWSKubeClusterTags creates kube cluster labels.
func labelsFromAWSKubeClusterTags(tags map[string]*string, parsedARN arn.ARN) map[string]string {
	labels := awsEKSTagsToLabels(tags)
	labels[types.CloudLabel] = types.CloudAWS
	labels[types.DiscoveryLabelRegion] = parsedARN.Region
	labels[types.DiscoveryLabelAccountID] = parsedARN.AccountID
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
