/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package resources

import (
	"context"
	"fmt"
	"io"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/externalauditstorage"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type externalAuditStorageCollection struct {
	externalAuditStorages []*externalauditstorage.ExternalAuditStorage
}

func externalAuditStorageHandler() Handler {
	return Handler{
		getHandler:    getExternalAuditStorage,
		createHandler: createExternalAuditStorage,
		deleteHandler: deleteExternalAuditStorage,
		singleton:     false,
		mfaRequired:   false,
		description:   "Configures External Audit Storage settings",
	}
}

func getExternalAuditStorage(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	out := []*externalauditstorage.ExternalAuditStorage{}
	name := ref.Name
	switch name {
	case "":
		cluster, err := client.ExternalAuditStorageClient().GetClusterExternalAuditStorage(ctx)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
		} else {
			out = append(out, cluster)
		}
		draft, err := client.ExternalAuditStorageClient().GetDraftExternalAuditStorage(ctx)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
		} else {
			out = append(out, draft)
		}
		return &externalAuditStorageCollection{externalAuditStorages: out}, nil
	case types.MetaNameExternalAuditStorageCluster:
		cluster, err := client.ExternalAuditStorageClient().GetClusterExternalAuditStorage(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &externalAuditStorageCollection{externalAuditStorages: []*externalauditstorage.ExternalAuditStorage{cluster}}, nil
	case types.MetaNameExternalAuditStorageDraft:
		draft, err := client.ExternalAuditStorageClient().GetDraftExternalAuditStorage(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &externalAuditStorageCollection{externalAuditStorages: []*externalauditstorage.ExternalAuditStorage{draft}}, nil
	default:
		return nil, trace.BadParameter("unsupported resource name for external_audit_storage, valid for get are: '', %q, %q", types.MetaNameExternalAuditStorageDraft, types.MetaNameExternalAuditStorageCluster)
	}
}

func createExternalAuditStorage(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	draft, err := services.UnmarshalExternalAuditStorage(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	externalAuditClient := client.ExternalAuditStorageClient()
	if opts.Force {
		if _, err := externalAuditClient.UpsertDraftExternalAuditStorage(ctx, draft); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("External Audit Storage configuration has been updated\n")
	} else {
		if _, err := externalAuditClient.CreateDraftExternalAuditStorage(ctx, draft); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("External Audit Storage configuration has been created\n")
	}
	return nil
}

func deleteExternalAuditStorage(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if ref.Name == types.MetaNameExternalAuditStorageCluster {
		if err := client.ExternalAuditStorageClient().DisableClusterExternalAuditStorage(ctx); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("cluster External Audit Storage configuration has been disabled\n")
	} else { // Note: deletes 'draft' if the user supplies any name other than 'cluster'
		if err := client.ExternalAuditStorageClient().DeleteDraftExternalAuditStorage(ctx); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("draft External Audit Storage configuration has been deleted\n")
	}
	return nil
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
