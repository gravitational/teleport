/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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
