/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package types

import (
	"time"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/utils"
)

// WatchStatus contains information about a successful Watch request.
type WatchStatus interface {
	Resource
	// GetKinds returns the list of kinds confirmed by the Watch request.
	GetKinds() []WatchKind
	// SetKinds sets the list of kinds confirmed by the Watch request.
	SetKinds([]WatchKind)
	// Clone performs a deep copy of watch status.
	Clone() WatchStatus
}

// GetKind returns the watch status resource kind.
func (w *WatchStatusV1) GetKind() string {
	return w.Kind
}

// GetSubKind returns the watch status resource subkind.
func (w *WatchStatusV1) GetSubKind() string {
	return w.SubKind
}

// SetSubKind sets the watch status resource subkind.
func (w *WatchStatusV1) SetSubKind(k string) {
	w.SubKind = k
}

// GetVersion returns the watch status resource version.
func (w *WatchStatusV1) GetVersion() string {
	return w.Version
}

// GetName returns the watch status resource name.
func (w *WatchStatusV1) GetName() string {
	return w.Metadata.Name
}

// SetName sets the watch status resource name.
func (w *WatchStatusV1) SetName(name string) {
	w.Metadata.Name = name
}

// Expiry returns the watch status resource expiration time.
func (w *WatchStatusV1) Expiry() time.Time {
	return w.Metadata.Expiry()
}

// SetExpiry sets the watch status resource expiration time.
func (w *WatchStatusV1) SetExpiry(time time.Time) {
	w.Metadata.SetExpiry(time)
}

// GetMetadata returns the watch status resource metadata.
func (w *WatchStatusV1) GetMetadata() Metadata {
	return w.Metadata
}

// GetRevision returns the revision
func (w *WatchStatusV1) GetRevision() string {
	return w.Metadata.GetRevision()
}

// SetRevision sets the revision
func (w *WatchStatusV1) SetRevision(rev string) {
	w.Metadata.SetRevision(rev)
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (w *WatchStatusV1) CheckAndSetDefaults() error {
	return nil
}

// GetKinds returns the list of kinds confirmed by the Watch request.
func (w *WatchStatusV1) GetKinds() []WatchKind {
	return w.Spec.Kinds
}

// SetKinds sets the list of kinds confirmed by the Watch request.
func (w *WatchStatusV1) SetKinds(kinds []WatchKind) {
	w.Spec.Kinds = kinds
}

// Clone performs a deep-copy of watch status.
func (w *WatchStatusV1) Clone() WatchStatus {
	return utils.CloneProtoMsg(w)
}

// NewWatchStatus returns a new WatchStatus resource.
func NewWatchStatus(kinds []WatchKind) *WatchStatusV1 {
	return &WatchStatusV1{
		Kind:    KindWatchStatus,
		Version: V1,
		Metadata: Metadata{
			Name:      MetaNameWatchStatus,
			Namespace: defaults.Namespace,
		},
		Spec: WatchStatusSpecV1{
			Kinds: kinds,
		},
	}
}
