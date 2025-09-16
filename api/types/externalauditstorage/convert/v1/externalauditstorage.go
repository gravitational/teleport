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

package v1

import (
	"github.com/gravitational/trace"

	externalauditstoragev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/externalauditstorage/v1"
	"github.com/gravitational/teleport/api/types/externalauditstorage"
	headerv1 "github.com/gravitational/teleport/api/types/header/convert/v1"
)

// FromProtoDraft converts an external representation of a v1 `ExternalAuditStorage`
// into an internal representation of a draft `ExternalAuditStorage` object.
func FromProtoDraft(in *externalauditstoragev1.ExternalAuditStorage) (*externalauditstorage.ExternalAuditStorage, error) {
	if in == nil {
		return nil, trace.BadParameter("External Audit Storage message is nil")
	}

	if in.Spec == nil {
		return nil, trace.BadParameter("spec is missing")
	}

	externalAuditStorage, err := externalauditstorage.NewDraftExternalAuditStorage(headerv1.FromMetadataProto(in.Header.Metadata), externalauditstorage.ExternalAuditStorageSpec{
		IntegrationName:        in.Spec.IntegrationName,
		PolicyName:             in.Spec.PolicyName,
		Region:                 in.Spec.Region,
		SessionRecordingsURI:   in.Spec.SessionRecordingsUri,
		AthenaWorkgroup:        in.Spec.AthenaWorkgroup,
		GlueDatabase:           in.Spec.GlueDatabase,
		GlueTable:              in.Spec.GlueTable,
		AuditEventsLongTermURI: in.Spec.AuditEventsLongTermUri,
		AthenaResultsURI:       in.Spec.AthenaResultsUri,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return externalAuditStorage, nil
}

// FromProtoCluster converts an external representation of a v1 `ExternalAuditStorage`
// into an internal representation of a cluster `ExternalAuditStorage` object.
func FromProtoCluster(in *externalauditstoragev1.ExternalAuditStorage) (*externalauditstorage.ExternalAuditStorage, error) {
	if in == nil {
		return nil, trace.BadParameter("External Audit Storage message is nil")
	}

	if in.Spec == nil {
		return nil, trace.BadParameter("spec is missing")
	}

	externalAuditStorage, err := externalauditstorage.NewClusterExternalAuditStorage(headerv1.FromMetadataProto(in.Header.Metadata), externalauditstorage.ExternalAuditStorageSpec{
		IntegrationName:        in.Spec.IntegrationName,
		PolicyName:             in.Spec.PolicyName,
		Region:                 in.Spec.Region,
		SessionRecordingsURI:   in.Spec.SessionRecordingsUri,
		AthenaWorkgroup:        in.Spec.AthenaWorkgroup,
		GlueDatabase:           in.Spec.GlueDatabase,
		GlueTable:              in.Spec.GlueTable,
		AuditEventsLongTermURI: in.Spec.AuditEventsLongTermUri,
		AthenaResultsURI:       in.Spec.AthenaResultsUri,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return externalAuditStorage, nil
}

// ToProto converts an internal representation of a `ExternalAuditStorage`
// into a v1 external representation of an `ExternalAuditStorage` object.
func ToProto(in *externalauditstorage.ExternalAuditStorage) *externalauditstoragev1.ExternalAuditStorage {
	return &externalauditstoragev1.ExternalAuditStorage{
		Header: headerv1.ToResourceHeaderProto(in.ResourceHeader),
		Spec: &externalauditstoragev1.ExternalAuditStorageSpec{
			IntegrationName:        in.Spec.IntegrationName,
			PolicyName:             in.Spec.PolicyName,
			Region:                 in.Spec.Region,
			SessionRecordingsUri:   in.Spec.SessionRecordingsURI,
			AthenaWorkgroup:        in.Spec.AthenaWorkgroup,
			GlueDatabase:           in.Spec.GlueDatabase,
			GlueTable:              in.Spec.GlueTable,
			AuditEventsLongTermUri: in.Spec.AuditEventsLongTermURI,
			AthenaResultsUri:       in.Spec.AthenaResultsURI,
		},
	}
}
