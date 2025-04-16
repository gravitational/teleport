// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package common

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/gcp"
)

// setAWSKubeName modifies the types.Metadata in place, overriding the first
// part if the kube cluster override label for AWS is present, and setting the
// kube cluster name.
func setAWSKubeName(meta types.Metadata, firstNamePart string, extraNameParts ...string) types.Metadata {
	return setResourceName(types.AWSKubeClusterNameOverrideLabels, meta, firstNamePart, extraNameParts...)
}

// NewKubeClusterFromAWSEKS creates a kube_cluster resource from an EKS cluster.
func NewKubeClusterFromAWSEKS(clusterName, clusterArn string, tags map[string]string) (types.KubeCluster, error) {
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
func labelsFromAWSKubeClusterTags(tags map[string]string, parsedARN arn.ARN) map[string]string {
	labels := awsEKSTagsToLabels(tags)
	labels[types.CloudLabel] = types.CloudAWS
	labels[types.DiscoveryLabelRegion] = parsedARN.Region
	labels[types.DiscoveryLabelAccountID] = parsedARN.AccountID
	labels[types.DiscoveryLabelAWSArn] = parsedARN.String()
	return labels
}

// awsEKSTagsToLabels converts AWS tags to a labels map.
func awsEKSTagsToLabels(tags map[string]string) map[string]string {
	labels := make(map[string]string)
	for key, val := range tags {
		if types.IsValidLabelKey(key) {
			labels[key] = val
		} else {
			slog.DebugContext(context.Background(), "Skipping EKS tag that is not a valid label key", "tag", key)
		}
	}
	return labels
}

// setAzureKubeName modifies the types.Metadata in place, overriding the first
// part if the AKS kube cluster override label is present, and setting the kube
// cluster name.
func setAzureKubeName(meta types.Metadata, firstNamePart string, extraNameParts ...string) types.Metadata {
	return setResourceName([]string{types.AzureKubeClusterNameOverrideLabel}, meta, firstNamePart, extraNameParts...)
}

func addLabels(labels map[string]string, moreLabels map[string]string) map[string]string {
	for key, value := range moreLabels {
		if types.IsValidLabelKey(key) {
			labels[key] = value
		} else {
			slog.DebugContext(context.Background(), "Skipping label that is not a valid label key", "label", key)
		}
	}
	return labels
}

// azureTagsToLabels converts Azure tags to a labels map.
func azureTagsToLabels(tags map[string]string) map[string]string {
	labels := make(map[string]string)
	labels[types.CloudLabel] = types.CloudAzure
	return addLabels(labels, tags)
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

// setResourceName modifies the types.Metadata argument in place, setting the resource name.
// The name is calculated based on nameParts arguments which are joined by hyphens "-".
// If a name override label is present, it will replace the *first* name part.
func setResourceName(overrideLabels []string, meta types.Metadata, firstNamePart string, extraNameParts ...string) types.Metadata {
	nameParts := append([]string{firstNamePart}, extraNameParts...)

	// apply override
	for _, overrideLabel := range overrideLabels {
		if override, found := meta.Labels[overrideLabel]; found && override != "" {
			nameParts[0] = override
			break
		}
	}

	meta.Name = strings.Join(nameParts, "-")

	return meta
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

// setGCPKubeName modifies the types.Metadata in place, overriding the first
// part if the GKE kube cluster override label is present, and setting the kube
// cluster name.
func setGCPKubeName(meta types.Metadata, firstNamePart string, extraNameParts ...string) types.Metadata {
	return setResourceName([]string{types.GCPKubeClusterNameOverrideLabel}, meta, firstNamePart, extraNameParts...)
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
