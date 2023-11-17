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

package services

import (
	"context"

	"github.com/gravitational/trace"

	externalcloudauditclient "github.com/gravitational/teleport/api/client/externalcloudaudit"
	"github.com/gravitational/teleport/api/types/externalcloudaudit"
	"github.com/gravitational/teleport/lib/utils"
)

var _ ExternalCloudAudits = (*externalcloudauditclient.Client)(nil)

// ExternalCloudAuditGetter defines an interface for reading external cloud audits.
type ExternalCloudAuditGetter interface {
	// GetDraftExternalCloudAudit returns the draft external cloud audit resource.
	GetDraftExternalCloudAudit(context.Context) (*externalcloudaudit.ExternalCloudAudit, error)
	// GetClusterExternalCloudAudit returns the cluster external cloud audit resource.
	GetClusterExternalCloudAudit(context.Context) (*externalcloudaudit.ExternalCloudAudit, error)
}

// ExternalCloudAudits defines an interface for managing ExternalCloudAudits.
type ExternalCloudAudits interface {
	ExternalCloudAuditGetter

	// UpsertDraftExternalCloudAudit upserts the draft external cloud audit resource.
	UpsertDraftExternalCloudAudit(context.Context, *externalcloudaudit.ExternalCloudAudit) (*externalcloudaudit.ExternalCloudAudit, error)
	// GenerateDraftExternalCloudAudit create a new draft external cloud audit
	// resource with randomized resource names and upserts it as the current
	// draft.
	GenerateDraftExternalCloudAudit(ctx context.Context, integrationName, region string) (*externalcloudaudit.ExternalCloudAudit, error)
	// DeleteDraftExternalCloudAudit deletes the draft external cloud audit resource.
	DeleteDraftExternalCloudAudit(context.Context) error
	// PromoteToClusterExternalCloudAudit promotes draft to cluster external
	// cloud audit resource.
	PromoteToClusterExternalCloudAudit(context.Context) error
	// DisableClusterExternalCloudAudit disables an cluster external cloud audit
	// resource.
	DisableClusterExternalCloudAudit(context.Context) error
}

// UnmarshalExternalCloudAudit unmarshals the external cloud audit resource from JSON.
func UnmarshalExternalCloudAudit(data []byte, opts ...MarshalOption) (*externalcloudaudit.ExternalCloudAudit, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing external cloud audit data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out *externalcloudaudit.ExternalCloudAudit
	if err := utils.FastUnmarshal(data, &out); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := out.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		out.SetResourceID(cfg.ID)
	}
	if cfg.Revision != "" {
		out.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		out.SetExpiry(cfg.Expires)
	}
	return out, nil
}

// MarshalExternalCloudAudit marshals the external cloud audit resource to JSON.
func MarshalExternalCloudAudit(externalCloudaudit *externalcloudaudit.ExternalCloudAudit, opts ...MarshalOption) ([]byte, error) {
	if err := externalCloudaudit.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.PreserveResourceID {
		copy := *externalCloudaudit
		copy.SetResourceID(0)
		copy.SetRevision("")
		externalCloudaudit = &copy
	}
	return utils.FastMarshal(externalCloudaudit)
}
