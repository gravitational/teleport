/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package collections

import (
	"io"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/externalauditstorage"
	"github.com/gravitational/teleport/lib/asciitable"
)

type externalAuditStorageCollection struct {
	externalAuditStorages []*externalauditstorage.ExternalAuditStorage
}

func NewExternalAuditStorageCollection(externalAuditStorages []*externalauditstorage.ExternalAuditStorage) ResourceCollection {
	return &externalAuditStorageCollection{externalAuditStorages: externalAuditStorages}
}

func (c *externalAuditStorageCollection) Resources() (r []types.Resource) {
	for _, a := range c.externalAuditStorages {
		r = append(r, a)
	}
	return r
}

func (c *externalAuditStorageCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, a := range c.externalAuditStorages {
		rows = append(rows, []string{
			a.GetName(),
			a.Spec.IntegrationName,
			a.Spec.PolicyName,
			a.Spec.Region,
			a.Spec.SessionRecordingsURI,
			a.Spec.AuditEventsLongTermURI,
			a.Spec.AthenaResultsURI,
			a.Spec.AthenaWorkgroup,
			a.Spec.GlueDatabase,
			a.Spec.GlueTable,
		})
	}
	headers := []string{"Name", "IntegrationName", "PolicyName", "Region", "SessionRecordingsURI", "AuditEventsLongTermURI", "AthenaResultsURI", "AthenaWorkgroup", "GlueDatabase", "GlueTable"}
	t := asciitable.MakeTable(headers, rows...)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
