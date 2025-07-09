// Copyright 2024 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package usertask

import (
	"context"

	"github.com/gravitational/trace"

	usertaskv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
)

// Client is a client for the User Task API.
type Client struct {
	grpcClient usertaskv1.UserTaskServiceClient
}

// NewClient creates a new User Task client.
func NewClient(grpcClient usertaskv1.UserTaskServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// ListUserTasks returns a list of User Tasks.
func (c *Client) ListUserTasks(ctx context.Context, pageSize int64, nextToken string, filters *usertaskv1.ListUserTasksFilters) ([]*usertaskv1.UserTask, string, error) {
	resp, err := c.grpcClient.ListUserTasks(ctx, &usertaskv1.ListUserTasksRequest{
		PageSize:  pageSize,
		PageToken: nextToken,
		Filters:   filters,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return resp.UserTasks, resp.NextPageToken, nil
}

// CreateUserTask creates a new User Task.
func (c *Client) CreateUserTask(ctx context.Context, req *usertaskv1.UserTask) (*usertaskv1.UserTask, error) {
	rsp, err := c.grpcClient.CreateUserTask(ctx, &usertaskv1.CreateUserTaskRequest{
		UserTask: req,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsp, nil
}

// GetUserTask returns a User Task by name.
func (c *Client) GetUserTask(ctx context.Context, name string) (*usertaskv1.UserTask, error) {
	rsp, err := c.grpcClient.GetUserTask(ctx, &usertaskv1.GetUserTaskRequest{
		Name: name,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsp, nil
}

// UpdateUserTask updates an existing User Task.
func (c *Client) UpdateUserTask(ctx context.Context, req *usertaskv1.UserTask) (*usertaskv1.UserTask, error) {
	rsp, err := c.grpcClient.UpdateUserTask(ctx, &usertaskv1.UpdateUserTaskRequest{
		UserTask: req,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsp, nil
}

// UpsertUserTask upserts a User Task.
func (c *Client) UpsertUserTask(ctx context.Context, req *usertaskv1.UserTask) (*usertaskv1.UserTask, error) {
	rsp, err := c.grpcClient.UpsertUserTask(ctx, &usertaskv1.UpsertUserTaskRequest{
		UserTask: req,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsp, nil
}

// DeleteUserTask deletes a User Task.
func (c *Client) DeleteUserTask(ctx context.Context, name string) error {
	_, err := c.grpcClient.DeleteUserTask(ctx, &usertaskv1.DeleteUserTaskRequest{
		Name: name,
	})
	return trace.Wrap(err)
}

// DeleteAllUserTasks deletes all User Tasks.
// Not implemented. Added to satisfy the interface.
func (c *Client) DeleteAllUserTasks(_ context.Context) error {
	return trace.NotImplemented("DeleteAllUserTasks is not implemented")
}
