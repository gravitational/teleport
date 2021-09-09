/*
Copyright 2020-2021 Gravitational, Inc.

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

import "time"

var ClusterEncryptionConfigName = "cluster/encryption_config"

type ClusterEncryptionConfig interface {
	// ResourceWithSecrets provides common methods for objects
	ResourceWithSecrets
	// GetSessionEncryptionKeys fetches all session encryption keys from the resource.
	GetSessionEncryptionKeys() []*SessionEncryptionKey
	// SetSessionEncryptionKeys sets the session encryption keys of the resource.
	SetSessionEncryptionKeys([]*SessionEncryptionKey)
}

// NewClusterEncryptionConfig creates a `ClusterEncryptionConfig` resource from a specification.
func NewClusterEncryptionConfig(spec ClusterEncryptionConfigSpecV3) ClusterEncryptionConfig {
	return &ClusterEncryptionConfigV3{
		Metadata: Metadata{
			Name: ClusterEncryptionConfigName,
		},
		Version: V3,
		Spec: spec,
	}
}

func (m *ClusterEncryptionConfigV3) GetKind() string {
	return m.Kind
}

func (m *ClusterEncryptionConfigV3) GetSubKind() string {
	return m.SubKind
}

func (m *ClusterEncryptionConfigV3) SetSubKind(s string) {
	m.SubKind = s
}

func (m *ClusterEncryptionConfigV3) GetVersion() string {
	return m.Version
}

func (m *ClusterEncryptionConfigV3) GetName() string {
	return m.Metadata.Name
}

func (m *ClusterEncryptionConfigV3) SetName(s string) {
	m.Metadata.Name = s
}

func (m *ClusterEncryptionConfigV3) Expiry() time.Time {
	return *m.Metadata.Expires
}

func (m *ClusterEncryptionConfigV3) SetExpiry(t time.Time) {
	m.Metadata.Expires = &t
}

func (m *ClusterEncryptionConfigV3) GetMetadata() Metadata {
	return m.Metadata
}

func (m *ClusterEncryptionConfigV3) GetResourceID() int64 {
	return m.Metadata.ID
}

func (m *ClusterEncryptionConfigV3) SetResourceID(i int64) {
	m.Metadata.ID = i
}

func (m *ClusterEncryptionConfigV3) CheckAndSetDefaults() error {
	return nil
}

func (m *ClusterEncryptionConfigV3) WithoutSecrets() Resource {
	m2 := *m
	m2.Spec.MasterKeys = nil
	return &m2
}

func (m *ClusterEncryptionConfigV3) GetSessionEncryptionKeys() []*SessionEncryptionKey {
	return m.Spec.MasterKeys
}

func (m *ClusterEncryptionConfigV3) SetSessionEncryptionKeys(keys []*SessionEncryptionKey) {
	m.Spec.MasterKeys = keys
}