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
	"regexp"
	"slices"
	"sort"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/compare"
	"github.com/gravitational/teleport/api/utils"
)

var _ compare.IsEqual[KubeCluster] = (*KubernetesClusterV3)(nil)

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
	// GetCloud gets the cloud this kube cluster is running on, or an empty string if it
	// isn't running on a cloud provider.
	GetCloud() string
}

// DiscoveredEKSCluster represents a server discovered by EKS discovery fetchers.
type DiscoveredEKSCluster interface {
	// KubeCluster is base discovered cluster.
	KubeCluster
	// GetKubeCluster returns base cluster.
	GetKubeCluster() KubeCluster
	// GetIntegration returns integration name used when discovering this cluster.
	GetIntegration() string
	// GetKubeAppDiscovery returns setting showing if Kubernetes App Discovery show be enabled for the discovered cluster.
	GetKubeAppDiscovery() bool
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

// GetRevision returns the revision
func (k *KubernetesClusterV3) GetRevision() string {
	return k.Metadata.GetRevision()
}

// SetRevision sets the revision
func (k *KubernetesClusterV3) SetRevision(rev string) {
	k.Metadata.SetRevision(rev)
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

// GetCloud gets the cloud this kube cluster is running on, or an empty string if it
// isn't running on a cloud provider.
func (k *KubernetesClusterV3) GetCloud() string {
	switch {
	case k.IsAzure():
		return CloudAzure
	case k.IsAWS():
		return CloudAWS
	case k.IsGCP():
		return CloudGCP
	default:
		return ""
	}
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
	return utils.CloneProtoMsg(k)
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

// validKubeClusterName filters the allowed characters in kubernetes cluster
// names. We need this because cluster names are used for cert filenames on the
// client side, in the ~/.tsh directory. Restricting characters helps with
// sneaky cluster names being used for client directory traversal and exploits.
var validKubeClusterName = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// ValidateKubeClusterName returns an error if a given string is not a valid
// KubeCluster name.
func ValidateKubeClusterName(name string) error {
	return ValidateResourceName(validKubeClusterName, name)
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

	if err := ValidateKubeClusterName(k.Metadata.Name); err != nil {
		return trace.Wrap(err, "invalid kubernetes cluster name")
	}

	if err := k.Spec.Azure.CheckAndSetDefaults(); err != nil && k.IsAzure() {
		return trace.Wrap(err)
	}

	if err := k.Spec.AWS.CheckAndSetDefaults(); err != nil && k.IsAWS() {
		return trace.Wrap(err)
	}

	if err := k.Spec.GCP.CheckAndSetDefaults(); err != nil && k.IsGCP() {
		return trace.Wrap(err)
	}

	return nil
}

// IsEqual determines if two user resources are equivalent to one another.
func (k *KubernetesClusterV3) IsEqual(i KubeCluster) bool {
	if other, ok := i.(*KubernetesClusterV3); ok {
		return deriveTeleportEqualKubernetesClusterV3(k, other)
	}
	return false
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

func (k KubeGCP) CheckAndSetDefaults() error {
	if len(k.Location) == 0 {
		return trace.BadParameter("invalid GCP Location")
	}

	if len(k.ProjectID) == 0 {
		return trace.BadParameter("invalid GCP ProjectID")
	}

	if len(k.Name) == 0 {
		return trace.BadParameter("invalid GCP Name")
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

var _ ResourceWithLabels = (*KubernetesResourceV1)(nil)

// NewKubernetesPodV1 creates a new kubernetes resource with kind "pod".
func NewKubernetesPodV1(meta Metadata, spec KubernetesResourceSpecV1) (*KubernetesResourceV1, error) {
	pod := &KubernetesResourceV1{
		Kind:     KindKubePod,
		Metadata: meta,
		Spec:     spec,
	}

	if err := pod.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return pod, nil
}

// NewKubernetesResourceV1 creates a new kubernetes resource .
func NewKubernetesResourceV1(kind string, meta Metadata, spec KubernetesResourceSpecV1) (*KubernetesResourceV1, error) {
	resource := &KubernetesResourceV1{
		Kind:     kind,
		Metadata: meta,
		Spec:     spec,
	}
	if err := resource.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return resource, nil
}

// GetKind returns resource kind.
func (k *KubernetesResourceV1) GetKind() string {
	return k.Kind
}

// GetSubKind returns resource subkind.
func (k *KubernetesResourceV1) GetSubKind() string {
	return k.SubKind
}

// GetVersion returns resource version.
func (k *KubernetesResourceV1) GetVersion() string {
	return k.Version
}

// GetMetadata returns object metadata.
func (k *KubernetesResourceV1) GetMetadata() Metadata {
	return k.Metadata
}

// SetSubKind sets resource subkind.
func (k *KubernetesResourceV1) SetSubKind(subKind string) {
	k.SubKind = subKind
}

// GetName returns the name of the resource.
func (k *KubernetesResourceV1) GetName() string {
	return k.Metadata.GetName()
}

// SetName sets the name of the resource.
func (k *KubernetesResourceV1) SetName(name string) {
	k.Metadata.SetName(name)
}

// Expiry returns object expiry setting.
func (k *KubernetesResourceV1) Expiry() time.Time {
	return k.Metadata.Expiry()
}

// SetExpiry sets object expiry.
func (k *KubernetesResourceV1) SetExpiry(expire time.Time) {
	k.Metadata.SetExpiry(expire)
}

// GetResourceID returns resource ID.
func (k *KubernetesResourceV1) GetResourceID() int64 {
	return k.Metadata.ID
}

// SetResourceID sets resource ID.
func (k *KubernetesResourceV1) SetResourceID(id int64) {
	k.Metadata.ID = id
}

// GetRevision returns the revision
func (k *KubernetesResourceV1) GetRevision() string {
	return k.Metadata.GetRevision()
}

// SetRevision sets the revision
func (k *KubernetesResourceV1) SetRevision(rev string) {
	k.Metadata.SetRevision(rev)
}

// CheckAndSetDefaults validates the Resource and sets any empty fields to
// default values.
func (k *KubernetesResourceV1) CheckAndSetDefaults() error {
	k.setStaticFields()
	if !slices.Contains(KubernetesResourcesKinds, k.Kind) {
		return trace.BadParameter("invalid kind %q defined; allowed values: %v", k.Kind, KubernetesResourcesKinds)
	}
	if err := k.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// Unless the resource is cluster-wide, it must have a namespace.
	if len(k.Spec.Namespace) == 0 && !slices.Contains(KubernetesClusterWideResourceKinds, k.Kind) {
		return trace.BadParameter("missing kubernetes namespace")
	}

	return nil
}

// setStaticFields sets static resource header and metadata fields.
func (k *KubernetesResourceV1) setStaticFields() {
	k.Version = V1
}

// Origin returns the origin value of the resource.
func (k *KubernetesResourceV1) Origin() string {
	return k.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (k *KubernetesResourceV1) SetOrigin(origin string) {
	k.Metadata.SetOrigin(origin)
}

// GetLabel retrieves the label with the provided key. If not found
// value will be empty and ok will be false.
func (k *KubernetesResourceV1) GetLabel(key string) (value string, ok bool) {
	v, ok := k.Metadata.Labels[key]
	return v, ok
}

// GetAllLabels returns all resource's labels.
func (k *KubernetesResourceV1) GetAllLabels() map[string]string {
	return k.Metadata.Labels
}

// GetStaticLabels returns the resource's static labels.
func (k *KubernetesResourceV1) GetStaticLabels() map[string]string {
	return k.Metadata.Labels
}

// SetStaticLabels sets the resource's static labels.
func (k *KubernetesResourceV1) SetStaticLabels(sl map[string]string) {
	k.Metadata.Labels = sl
}

// MatchSearch goes through select field values of a resource
// and tries to match against the list of search values.
func (k *KubernetesResourceV1) MatchSearch(searchValues []string) bool {
	fieldVals := append(utils.MapToStrings(k.GetAllLabels()), k.GetName(), k.Spec.Namespace)
	return MatchSearch(fieldVals, searchValues, nil)
}

// KubeResources represents a list of Kubernetes resources.
type KubeResources []*KubernetesResourceV1

// Find returns Kubernetes resource with the specified name or nil if the resource
// was not found.
func (k KubeResources) Find(name string) *KubernetesResourceV1 {
	for _, cluster := range k {
		if cluster.GetName() == name {
			return cluster
		}
	}
	return nil
}

// ToMap returns these kubernetes resources as a map keyed by resource name.
func (k KubeResources) ToMap() map[string]*KubernetesResourceV1 {
	m := make(map[string]*KubernetesResourceV1)
	for _, kubeCluster := range k {
		m[kubeCluster.GetName()] = kubeCluster
	}
	return m
}

// Len returns the slice length.
func (k KubeResources) Len() int { return len(k) }

// Less compares Kubernetes resources by name.
func (k KubeResources) Less(i, j int) bool {
	return k[i].GetName() < k[j].GetName()
}

// Swap swaps two Kubernetes resources.
func (k KubeResources) Swap(i, j int) { k[i], k[j] = k[j], k[i] }

// SortByCustom custom sorts by given sort criteria.
func (k KubeResources) SortByCustom(sortBy SortBy) error {
	if sortBy.Field == "" {
		return nil
	}

	isDesc := sortBy.IsDesc
	switch sortBy.Field {
	case ResourceMetadataName:
		sort.SliceStable(k, func(i, j int) bool {
			return stringCompare(k[i].GetName(), k[j].GetName(), isDesc)
		})
	default:
		return trace.NotImplemented("sorting by field %q for kubernetes resources is not supported", sortBy.Field)
	}

	return nil
}

// AsResources returns as type resources with labels.
func (k KubeResources) AsResources() ResourcesWithLabels {
	resources := make(ResourcesWithLabels, 0, len(k))
	for _, resource := range k {
		resources = append(resources, ResourceWithLabels(resource))
	}
	return resources
}
