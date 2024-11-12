// Copyright 2024 Gravitational, Inc.
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

package databaseobject

import (
	"context"

	"github.com/gravitational/trace"

	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
)

// Client is an DatabaseObject client that conforms to the following lib/services interfaces:
//   - services.DatabaseObjects
type Client struct {
	grpcClient dbobjectv1.DatabaseObjectServiceClient
}

// NewClient creates a new Database Object client.
func NewClient(grpcClient dbobjectv1.DatabaseObjectServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// ListDatabaseObjects returns a paginated list of DatabaseObjects.
func (c *Client) ListDatabaseObjects(ctx context.Context, pageSize int, nextToken string) ([]*dbobjectv1.DatabaseObject, string, error) {
	resp, err := c.grpcClient.ListDatabaseObjects(ctx, &dbobjectv1.ListDatabaseObjectsRequest{
		PageSize:  int32(pageSize),
		PageToken: nextToken,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return resp.Objects, resp.GetNextPageToken(), nil
}

// GetDatabaseObject returns the specified DatabaseObject resource.
func (c *Client) GetDatabaseObject(ctx context.Context, name string) (*dbobjectv1.DatabaseObject, error) {
	resp, err := c.grpcClient.GetDatabaseObject(ctx, &dbobjectv1.GetDatabaseObjectRequest{Name: name})
	return resp, trace.Wrap(err)
}

// CreateDatabaseObject creates the DatabaseObject.
func (c *Client) CreateDatabaseObject(ctx context.Context, obj *dbobjectv1.DatabaseObject) (*dbobjectv1.DatabaseObject, error) {
	resp, err := c.grpcClient.CreateDatabaseObject(ctx, &dbobjectv1.CreateDatabaseObjectRequest{Object: obj})
	return resp, trace.Wrap(err)
}

// UpdateDatabaseObject updates the DatabaseObject.
func (c *Client) UpdateDatabaseObject(ctx context.Context, obj *dbobjectv1.DatabaseObject) (*dbobjectv1.DatabaseObject, error) {
	resp, err := c.grpcClient.UpdateDatabaseObject(ctx, &dbobjectv1.UpdateDatabaseObjectRequest{Object: obj})
	return resp, trace.Wrap(err)
}

// UpsertDatabaseObject creates or updates a DatabaseObject.
func (c *Client) UpsertDatabaseObject(ctx context.Context, obj *dbobjectv1.DatabaseObject) (*dbobjectv1.DatabaseObject, error) {
	resp, err := c.grpcClient.UpsertDatabaseObject(ctx, &dbobjectv1.UpsertDatabaseObjectRequest{Object: obj})
	return resp, trace.Wrap(err)
}

// DeleteDatabaseObject removes the specified DatabaseObject resource.
func (c *Client) DeleteDatabaseObject(ctx context.Context, name string) error {
	_, err := c.grpcClient.DeleteDatabaseObject(ctx, &dbobjectv1.DeleteDatabaseObjectRequest{Name: name})
	return trace.Wrap(err)
}
