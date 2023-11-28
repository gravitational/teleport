// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package services

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/externalauditstorage"
	"github.com/gravitational/teleport/lib/utils"
)

// UnmarshalExternalAuditStorage unmarshals the External Audit Storage resource from JSON.
func UnmarshalExternalAuditStorage(data []byte, opts ...MarshalOption) (*externalauditstorage.ExternalAuditStorage, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing External Audit Storage data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out *externalauditstorage.ExternalAuditStorage
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

// MarshalExternalAuditStorage marshals the External Audit Storage resource to JSON.
func MarshalExternalAuditStorage(externalAuditStorage *externalauditstorage.ExternalAuditStorage, opts ...MarshalOption) ([]byte, error) {
	if err := externalAuditStorage.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.PreserveResourceID {
		copy := *externalAuditStorage
		copy.SetResourceID(0)
		copy.SetRevision("")
		externalAuditStorage = &copy
	}
	return utils.FastMarshal(externalAuditStorage)
}
