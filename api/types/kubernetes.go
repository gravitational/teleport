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
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/trace"
)

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

// GetNamespace returns the app resource namespace.
func (k *KubernetesClusterV3) GetNamespace() string {
	return k.Metadata.Namespace
}

// SetExpiry sets the app resource expiration time.
func (k *KubernetesClusterV3) SetExpiry(expiry time.Time) {
	k.Metadata.SetExpiry(expiry)
}

// Expiry returns the app resource expiration time.
func (k *KubernetesClusterV3) Expiry() time.Time {
	return k.Metadata.Expiry()
}

// GetName returns the app resource name.
func (k *KubernetesClusterV3) GetName() string {
	return k.Metadata.Name
}

// SetName sets the resource name.
func (k *KubernetesClusterV3) SetName(name string) {
	k.Metadata.Name = name
}

// GetStaticLabels returns the static labels.
func (k *KubernetesClusterV3) GetStaticLabels() map[string]string {
	return k.Metadata.Labels
}

// SetStaticLabels sets the static labels.
func (k *KubernetesClusterV3) SetStaticLabels(sl map[string]string) {
	k.Metadata.Labels = sl
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

	return nil
}
