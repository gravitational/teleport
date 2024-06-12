/*
Copyright 2023 Gravitational, Inc.

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

package legacy

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
)

// FromHeaderMetadata will convert a *header.Metadata object to this metadata object.
// TODO: Remove this once we get rid of the old Metadata object.
func FromHeaderMetadata(metadata header.Metadata) types.Metadata {
	return types.Metadata{
		ID:          metadata.ID,
		Name:        metadata.Name,
		Expires:     &metadata.Expires,
		Description: metadata.Description,
		Labels:      metadata.Labels,
		Revision:    metadata.Revision,
	}
}
