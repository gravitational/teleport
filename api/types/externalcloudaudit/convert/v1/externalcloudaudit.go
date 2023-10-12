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

	externalcloudauditv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/externalcloudaudit/v1"
	"github.com/gravitational/teleport/api/types/externalcloudaudit"
	headerv1 "github.com/gravitational/teleport/api/types/header/convert/v1"
)

// FromProtoDraft converts an external representation of a v1 `ExternalCloudAudit`
// into an internal representation of a draft `ExternalCloudAudit` object.
func FromProtoDraft(in *externalcloudauditv1.ExternalCloudAudit) (*externalcloudaudit.ExternalCloudAudit, error) {
	if in == nil {
		return nil, trace.BadParameter("external cloud audit message is nil")
	}

	if in.Spec == nil {
		return nil, trace.BadParameter("spec is missing")
	}

	externalCloudAudit, err := externalcloudaudit.NewDraftExternalCloudAudit(headerv1.FromMetadataProto(in.Header.Metadata), externalcloudaudit.ExternalCloudAuditSpec{
		IntegrationName:        in.Spec.IntegrationName,
		SessionsRecordingsURI:  in.Spec.SessionsRecordingsUri,
		AthenaWorkgroup:        in.Spec.AthenaWorkgroup,
		GlueDatabase:           in.Spec.GlueDatabase,
		GlueTable:              in.Spec.GlueTable,
		AuditEventsLongTermURI: in.Spec.AuditEventsLongTermUri,
		AthenaResultsURI:       in.Spec.AthenaResultsUri,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return externalCloudAudit, nil
}

// FromProtoCluster converts an external representation of a v1 `ExternalCloudAudit`
// into an internal representation of a cluster `ExternalCloudAudit` object.
func FromProtoCluster(in *externalcloudauditv1.ExternalCloudAudit) (*externalcloudaudit.ExternalCloudAudit, error) {
	if in == nil {
		return nil, trace.BadParameter("external cloud audit message is nil")
	}

	if in.Spec == nil {
		return nil, trace.BadParameter("spec is missing")
	}

	externalCloudAudit, err := externalcloudaudit.NewClusterExternalCloudAudit(headerv1.FromMetadataProto(in.Header.Metadata), externalcloudaudit.ExternalCloudAuditSpec{
		IntegrationName:        in.Spec.IntegrationName,
		SessionsRecordingsURI:  in.Spec.SessionsRecordingsUri,
		AthenaWorkgroup:        in.Spec.AthenaWorkgroup,
		GlueDatabase:           in.Spec.GlueDatabase,
		GlueTable:              in.Spec.GlueTable,
		AuditEventsLongTermURI: in.Spec.AuditEventsLongTermUri,
		AthenaResultsURI:       in.Spec.AthenaResultsUri,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return externalCloudAudit, nil
}

// ToProto converts an internal representation of a `ExternalCloudAudit`
// into a v1 external representation of an `ExternalCloudAudit` object.
func ToProto(in *externalcloudaudit.ExternalCloudAudit) *externalcloudauditv1.ExternalCloudAudit {
	return &externalcloudauditv1.ExternalCloudAudit{
		Header: headerv1.ToResourceHeaderProto(in.ResourceHeader),
		Spec: &externalcloudauditv1.ExternalCloudAuditSpec{
			IntegrationName:        in.Spec.IntegrationName,
			SessionsRecordingsUri:  in.Spec.SessionsRecordingsURI,
			AthenaWorkgroup:        in.Spec.AthenaWorkgroup,
			GlueDatabase:           in.Spec.GlueDatabase,
			GlueTable:              in.Spec.GlueTable,
			AuditEventsLongTermUri: in.Spec.AuditEventsLongTermURI,
			AthenaResultsUri:       in.Spec.AthenaResultsURI,
		},
	}
}
