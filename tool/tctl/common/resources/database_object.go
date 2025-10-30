// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package resources

import (
	"context"
	"fmt"
	"io"

	"github.com/gravitational/trace"

	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/databaseobject"
)

type databaseObjectCollection struct {
	objects []*dbobjectv1.DatabaseObject
}

func (c *databaseObjectCollection) Resources() []types.Resource {
	resources := make([]types.Resource, len(c.objects))
	for i, b := range c.objects {
		resources[i] = databaseobject.ProtoToResource(b)
	}
	return resources
}

func (c *databaseObjectCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Kind", "DB Service", "Protocol"})
	for _, b := range c.objects {
		t.AddRow([]string{
			b.GetMetadata().GetName(),
			fmt.Sprintf("%v", b.GetSpec().GetObjectKind()),
			fmt.Sprintf("%v", b.GetSpec().GetDatabaseServiceName()),
			fmt.Sprintf("%v", b.GetSpec().GetProtocol()),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func databaseObjectHandler() Handler {
	return Handler{
		getHandler:    getDatabaseObject,
		createHandler: createDatabaseObject,
		updateHandler: updateDatabaseObject,
		deleteHandler: deleteDatabaseObject,
		singleton:     false,
		mfaRequired:   false,
		description:   "Representation of a database object that can be imported into Teleport.",
	}
}

func getDatabaseObject(ctx context.Context, client *authclient.Client, ref services.Ref, _ GetOpts) (Collection, error) {
	remote := client.DatabaseObjectClient()
	if ref.Name != "" {
		object, err := remote.GetDatabaseObject(ctx, &dbobjectv1.GetDatabaseObjectRequest{Name: ref.Name})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &databaseObjectCollection{objects: []*dbobjectv1.DatabaseObject{object}}, nil
	}

	objects, err := stream.Collect(clientutils.Resources(ctx,
		func(ctx context.Context, limit int, token string) ([]*dbobjectv1.DatabaseObject, string, error) {
			resp, err := remote.ListDatabaseObjects(ctx, &dbobjectv1.ListDatabaseObjectsRequest{
				PageSize:  int32(limit),
				PageToken: token,
			})

			return resp.GetObjects(), resp.GetNextPageToken(), trace.Wrap(err)
		}))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &databaseObjectCollection{objects: objects}, nil
}

func createDatabaseObject(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	object, err := databaseobject.UnmarshalJSON(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}
	if opts.Force {
		_, err = client.DatabaseObjectClient().UpsertDatabaseObject(ctx, &dbobjectv1.UpsertDatabaseObjectRequest{
			Object: object,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("database object %q has been created\n", object.GetMetadata().GetName())
		return nil
	}
	_, err = client.DatabaseObjectClient().CreateDatabaseObject(ctx, &dbobjectv1.CreateDatabaseObjectRequest{
		Object: object,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("database object %q has been created\n", object.GetMetadata().GetName())
	return nil
}

func updateDatabaseObject(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	object, err := databaseobject.UnmarshalJSON(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = client.DatabaseObjectClient().UpdateDatabaseObject(ctx, &dbobjectv1.UpdateDatabaseObjectRequest{
		Object: object,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("database object %q has been updated\n", object.GetMetadata().GetName())
	return nil
}

func deleteDatabaseObject(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if _, err := client.DatabaseObjectClient().DeleteDatabaseObject(ctx, &dbobjectv1.DeleteDatabaseObjectRequest{Name: ref.Name}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("database object %q has been deleted\n", ref.Name)
	return nil
}
