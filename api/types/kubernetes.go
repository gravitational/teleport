/*
Copyright 2021 Gravitational, Inc.

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

package types

import (
	"fmt"
	"sort"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
)

// KubeCluster represents a kubernetes cluster.
type KubeCluster interface {
	// ResourceWithLabels provides common resource methods.
	ResourceWithLabels
	// GetNamespace returns the kube cluster namespace.
	GetNamespace() string
	// GetStaticLabels returns the kube cluster static labels.
	GetStaticLabels() map[string]string
	// SetStaticLabels sets the kube cluster static labels.
	SetStaticLabels(map[string]string)
	// GetDynamicLabels returns the kube cluster dynamic labels.
	GetDynamicLabels() map[string]CommandLabel
	// SetDynamicLabels sets the kube cluster dynamic labels.
	SetDynamicLabels(map[string]CommandLabel)
	// GetKubeconfig returns the kubeconfig payload.
	GetKubeconfig() []byte
	// SetKubeconfig sets the kubeconfig.
	SetKubeconfig([]byte)
	// LabelsString returns all labels as a string.
	LabelsString() string
	// String returns string representation of the kube cluster.
	String() string
	// GetDescription returns the kube cluster description.
	GetDescription() string
	// GetAzureConfig gets the Azure config.
	GetAzureConfig() KubeAzure
	// SetAzureConfig sets the Azure config.
	SetAzureConfig(KubeAzure)
	// GetAWSConfig gets the AWS config.
	GetAWSConfig() KubeAWS
	// SetAWSConfig sets the AWS config.
	SetAWSConfig(KubeAWS)
	// GetGCPConfig gets the GCP config.
	GetGCPConfig() KubeGCP
	// SetGCPConfig sets the GCP config.
	SetGCPConfig(KubeGCP)
	// IsAzure indentifies if the KubeCluster contains Azure details.
	IsAzure() bool
	// IsAWS indentifies if the KubeCluster contains AWS details.
	IsAWS() bool
	// IsGCP indentifies if the KubeCluster contains GCP details.
	IsGCP() bool
	// IsKubeconfig identifies if the KubeCluster contains kubeconfig data.
	IsKubeconfig() bool
	// Copy returns a copy of this kube cluster resource.
	Copy() *KubernetesClusterV3
}

// NewKubernetesClusterV3FromLegacyCluster creates a new Kubernetes cluster resource
// from the legacy type.
func NewKubernetesClusterV3FromLegacyCluster(namespace string, cluster *KubernetesCluster) (*KubernetesClusterV3, error) {
	k := &KubernetesClusterV3{
		Metadata: Metadata{
			Name:      cluster.Name,
			Namespace: namespace,
			Labels:    cluster.StaticLabels,
		},
		Spec: KubernetesClusterSpecV3{
			DynamicLabels: cluster.DynamicLabels,
		},
	}

	if err := k.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return k, nil
}

// NewKubernetesClusterV3WithoutSecrets creates a new copy of the provided cluster
// but without secrets/credentials.
func NewKubernetesClusterV3WithoutSecrets(cluster KubeCluster) (*KubernetesClusterV3, error) {
	// Force a copy of the cluster to deep copy the Metadata fields.
	copiedCluster := cluster.Copy()
	clusterWithoutCreds, err := NewKubernetesClusterV3(
		copiedCluster.Metadata,
		KubernetesClusterSpecV3{
			DynamicLabels: copiedCluster.Spec.DynamicLabels,
		},
	)
	return clusterWithoutCreds, trace.Wrap(err)
}

// NewKubernetesClusterV3 creates a new Kubernetes cluster resource.
func NewKubernetesClusterV3(meta Metadata, spec KubernetesClusterSpecV3) (*KubernetesClusterV3, error) {
	k := &KubernetesClusterV3{
		Metadata: meta,
		Spec:     spec,
	}

	if err := k.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return k, nil
}

// GetVersion returns the resource version.
func (k *KubernetesClusterV3) GetVersion() string {
	return k.Version
}

// GetKind returns the resource kind.
func (k *KubernetesClusterV3) GetKind() string {
	return k.Kind
}

// GetSubKind returns the app resource subkind.
func (k *KubernetesClusterV3) GetSubKind() string {
	return k.SubKind
}

// SetSubKind sets the app resource subkind.
func (k *KubernetesClusterV3) SetSubKind(sk string) {
	k.SubKind = sk
}

// GetResourceID returns the app resource ID.
func (k *KubernetesClusterV3) GetResourceID() int64 {
	return k.Metadata.ID
}

// SetResourceID sets the resource ID.
func (k *KubernetesClusterV3) SetResourceID(id int64) {
	k.Metadata.ID = id
}

// GetMetadata returns the resource metadata.
func (k *KubernetesClusterV3) GetMetadata() Metadata {
	return k.Metadata
}

// Origin returns the origin value of the resource.
func (k *KubernetesClusterV3) Origin() string {
	return k.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (k *KubernetesClusterV3) SetOrigin(origin string) {
	k.Metadata.SetOrigin(origin)
}

// GetNamespace returns the kube resource namespace.
func (k *KubernetesClusterV3) GetNamespace() string {
	return k.Metadata.Namespace
}

// SetExpiry sets the kube resource expiration time.
func (k *KubernetesClusterV3) SetExpiry(expiry time.Time) {
	k.Metadata.SetExpiry(expiry)
}

// Expiry returns the kube resource expiration time.
func (k *KubernetesClusterV3) Expiry() time.Time {
	return k.Metadata.Expiry()
}

// GetName returns the kube resource name.
func (k *KubernetesClusterV3) GetName() string {
	return k.Metadata.Name
}

// SetName sets the resource name.
func (k *KubernetesClusterV3) SetName(name string) {
	k.Metadata.Name = name
}

// GetLabel retrieves the label with the provided key. If not found
// value will be empty and ok will be false.
func (k *KubernetesClusterV3) GetLabel(key string) (value string, ok bool) {
	if cmd, ok := k.Spec.DynamicLabels[key]; ok {
		return cmd.Result, ok
	}

	v, ok := k.Metadata.Labels[key]
	return v, ok
}

// GetStaticLabels returns the static labels.
func (k *KubernetesClusterV3) GetStaticLabels() map[string]string {
	return k.Metadata.Labels
}

// SetStaticLabels sets the static labels.
func (k *KubernetesClusterV3) SetStaticLabels(sl map[string]string) {
	k.Metadata.Labels = sl
}

// GetKubeconfig returns the kubeconfig payload.
func (k *KubernetesClusterV3) GetKubeconfig() []byte {
	return k.Spec.Kubeconfig
}

// SetKubeconfig sets the kubeconfig.
func (k *KubernetesClusterV3) SetKubeconfig(cfg []byte) {
	k.Spec.Kubeconfig = cfg
}

// GetDynamicLabels returns the dynamic labels.
func (k *KubernetesClusterV3) GetDynamicLabels() map[string]CommandLabel {
	if k.Spec.DynamicLabels == nil {
		return nil
	}
	return V2ToLabels(k.Spec.DynamicLabels)
}

// SetDynamicLabels sets the dynamic labels
func (k *KubernetesClusterV3) SetDynamicLabels(dl map[string]CommandLabel) {
	k.Spec.DynamicLabels = LabelsToV2(dl)
}

// GetAllLabels returns the combined static and dynamic labels.
func (k *KubernetesClusterV3) GetAllLabels() map[string]string {
	return CombineLabels(k.Metadata.Labels, k.Spec.DynamicLabels)
}

// LabelsString returns all labels as a string.
func (k *KubernetesClusterV3) LabelsString() string {
	return LabelsAsString(k.Metadata.Labels, k.Spec.DynamicLabels)
}

// GetDescription returns the description.
func (k *KubernetesClusterV3) GetDescription() string {
	return k.Metadata.Description
}

// GetAzureConfig gets the Azure config.
func (k *KubernetesClusterV3) GetAzureConfig() KubeAzure {
	return k.Spec.Azure
}

// SetAzureConfig sets the Azure config.
func (k *KubernetesClusterV3) SetAzureConfig(cfg KubeAzure) {
	k.Spec.Azure = cfg
}

// GetAWSConfig gets the AWS config.
func (k *KubernetesClusterV3) GetAWSConfig() KubeAWS {
	return k.Spec.AWS
}

// SetAWSConfig sets the AWS config.
func (k *KubernetesClusterV3) SetAWSConfig(cfg KubeAWS) {
	k.Spec.AWS = cfg
}

// GetGCPConfig gets the GCP config.
func (k *KubernetesClusterV3) GetGCPConfig() KubeGCP {
	return k.Spec.GCP
}

// SetGCPConfig sets the GCP config.
func (k *KubernetesClusterV3) SetGCPConfig(cfg KubeGCP) {
	k.Spec.GCP = cfg
}

// IsAzure indentifies if the KubeCluster contains Azure details.
func (k *KubernetesClusterV3) IsAzure() bool {
	return !protoKnownFieldsEqual(&k.Spec.Azure, &KubeAzure{})
}

// IsAWS indentifies if the KubeCluster contains AWS details.
func (k *KubernetesClusterV3) IsAWS() bool {
	return !protoKnownFieldsEqual(&k.Spec.AWS, &KubeAWS{})
}

// IsGCP indentifies if the KubeCluster contains GCP details.
func (k *KubernetesClusterV3) IsGCP() bool {
	return !protoKnownFieldsEqual(&k.Spec.GCP, &KubeGCP{})
}

// IsKubeconfig identifies if the KubeCluster contains kubeconfig data.
func (k *KubernetesClusterV3) IsKubeconfig() bool {
	return len(k.Spec.Kubeconfig) > 0
}

// String returns the string representation.
func (k *KubernetesClusterV3) String() string {
	return fmt.Sprintf("KubernetesCluster(Name=%v, Labels=%v)",
		k.GetName(), k.GetAllLabels())
}

// Copy returns a copy of this resource.
func (k *KubernetesClusterV3) Copy() *KubernetesClusterV3 {
	return proto.Clone(k).(*KubernetesClusterV3)
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (k *KubernetesClusterV3) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(k.GetAllLabels()), k.GetName())
	return MatchSearch(fieldVals, values, nil)
}

// setStaticFields sets static resource header and metadata fields.
func (k *KubernetesClusterV3) setStaticFields() {
	k.Kind = KindKubernetesCluster
	k.Version = V3
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (k *KubernetesClusterV3) CheckAndSetDefaults() error {
	k.setStaticFields()
	if err := k.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	for key := range k.Spec.DynamicLabels {
		if !IsValidLabelKey(key) {
			return trace.BadParameter("kubernetes cluster %q invalid label key: %q", k.GetName(), key)
		}
	}

	if err := k.Spec.Azure.CheckAndSetDefaults(); err != nil && k.IsAzure() {
		return trace.Wrap(err)
	}

	if err := k.Spec.AWS.CheckAndSetDefaults(); err != nil && k.IsAWS() {
		return trace.Wrap(err)
	}

	return nil
}

func (k KubeAzure) CheckAndSetDefaults() error {
	if len(k.ResourceGroup) == 0 {
		return trace.BadParameter("invalid Azure ResourceGroup")
	}

	if len(k.ResourceName) == 0 {
		return trace.BadParameter("invalid Azure ResourceName")
	}

	if len(k.SubscriptionID) == 0 {
		return trace.BadParameter("invalid Azure SubscriptionID")
	}

	return nil
}

func (k KubeAWS) CheckAndSetDefaults() error {
	if len(k.Region) == 0 {
		return trace.BadParameter("invalid AWS Region")
	}

	if len(k.Name) == 0 {
		return trace.BadParameter("invalid AWS Name")
	}

	if len(k.AccountID) == 0 {
		return trace.BadParameter("invalid AWS AccountID")
	}

	return nil
}

// KubeClusters represents a list of kube clusters.
type KubeClusters []KubeCluster

// Find returns kube cluster with the specified name or nil.
func (s KubeClusters) Find(name string) KubeCluster {
	for _, cluster := range s {
		if cluster.GetName() == name {
			return cluster
		}
	}
	return nil
}

// ToMap returns these kubernetes clusters as a map keyed by cluster name.
func (s KubeClusters) ToMap() map[string]KubeCluster {
	m := make(map[string]KubeCluster)
	for _, kubeCluster := range s {
		m[kubeCluster.GetName()] = kubeCluster
	}
	return m
}

// Len returns the slice length.
func (s KubeClusters) Len() int { return len(s) }

// Less compares kube clusters by name.
func (s KubeClusters) Less(i, j int) bool {
	return s[i].GetName() < s[j].GetName()
}

// Swap swaps two kube clusters.
func (s KubeClusters) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// SortByCustom custom sorts by given sort criteria.
func (s KubeClusters) SortByCustom(sortBy SortBy) error {
	if sortBy.Field == "" {
		return nil
	}

	isDesc := sortBy.IsDesc
	switch sortBy.Field {
	case ResourceMetadataName:
		sort.SliceStable(s, func(i, j int) bool {
			return stringCompare(s[i].GetName(), s[j].GetName(), isDesc)
		})
	default:
		return trace.NotImplemented("sorting by field %q for resource %q is not supported", sortBy.Field, KindKubernetesCluster)
	}

	return nil
}

// AsResources returns as type resources with labels.
func (s KubeClusters) AsResources() ResourcesWithLabels {
	resources := make(ResourcesWithLabels, 0, len(s))
	for _, cluster := range s {
		resources = append(resources, ResourceWithLabels(cluster))
	}
	return resources
}

// GetFieldVals returns list of select field values.
func (s KubeClusters) GetFieldVals(field string) ([]string, error) {
	vals := make([]string, 0, len(s))
	switch field {
	case ResourceMetadataName:
		for _, server := range s {
			vals = append(vals, server.GetName())
		}
	default:
		return nil, trace.NotImplemented("getting field %q for resource %q is not supported", field, KindKubernetesCluster)
	}

	return vals, nil
}

// DeduplicateKubeClusters deduplicates kube clusters by name.
func DeduplicateKubeClusters(kubeclusters []KubeCluster) []KubeCluster {
	seen := make(map[string]struct{})
	result := make([]KubeCluster, 0, len(kubeclusters))

	for _, cluster := range kubeclusters {
		if _, ok := seen[cluster.GetName()]; ok {
			continue
		}
		seen[cluster.GetName()] = struct{}{}
		result = append(result, cluster)
	}

	return result
}
