/*
 * *
 *  * Teleport
 *  * Copyright (C) 2024 Gravitational, Inc.
 *  *
 *  * This program is free software: you can redistribute it and/or modify
 *  * it under the terms of the GNU Affero General Public License as published by
 *  * the Free Software Foundation, either version 3 of the License, or
 *  * (at your option) any later version.
 *  *
 *  * This program is distributed in the hope that it will be useful,
 *  * but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  * GNU Affero General Public License for more details.
 *  *
 *  * You should have received a copy of the GNU Affero General Public License
 *  * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package sessionrecordingmetadata

import (
	"context"
	sessionrecordingmetatadav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/sessionrecordingmetatada/v1"
	"github.com/gravitational/trace"
)

// Client is an access list client that conforms to the following lib/services interfaces:
// * services.SessionRecordingMetadatas
type Client struct {
	grpcClient sessionrecordingmetatadav1.SessionRecordingMetadataServiceClient
}

// NewClient creates a new Access List client.
func NewClient(grpcClient sessionrecordingmetatadav1.SessionRecordingMetadataServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

func (c *Client) GetSessionRecordingMetadata(ctx context.Context, sessionID string) (*sessionrecordingmetatadav1.SessionRecordingMetadata, error) {
	resp, err := c.grpcClient.GetSessionRecordingMetadata(ctx, &sessionrecordingmetatadav1.GetSessionRecordingMetadataRequest{
		SessionId: sessionID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

func (c *Client) CreateSessionRecordingMetadata(ctx context.Context, metadata *sessionrecordingmetatadav1.SessionRecordingMetadata) (*sessionrecordingmetatadav1.SessionRecordingMetadata, error) {
	resp, err := c.grpcClient.CreateSessionRecordingMetadata(ctx, &sessionrecordingmetatadav1.CreateSessionRecordingMetadataRequest{
		SessionRecordingMetadata: metadata,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

func (c *Client) UpdateSessionRecordingMetadata(ctx context.Context, metadata *sessionrecordingmetatadav1.SessionRecordingMetadata) (*sessionrecordingmetatadav1.SessionRecordingMetadata, error) {
	resp, err := c.grpcClient.UpdateSessionRecordingMetadata(ctx, &sessionrecordingmetatadav1.UpdateSessionRecordingMetadataRequest{
		SessionRecordingMetadata: metadata,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

func (c *Client) DeleteSessionRecordingMetadata(ctx context.Context, sessionID string) error {
	_, err := c.grpcClient.DeleteSessionRecordingMetadata(ctx, &sessionrecordingmetatadav1.DeleteSessionRecordingMetadataRequest{
		SessionId: sessionID,
	})
	return trace.Wrap(err)
}

// ListSessionRecordingMetadata returns a paginated list of session recording metadata
func (c *Client) ListSessionRecordingMetadata(ctx context.Context, pageSize int, nextToken string, sessionIDs []string, withSummary bool) ([]*sessionrecordingmetatadav1.SessionRecordingMetadata, string, error) {
	resp, err := c.grpcClient.ListSessionRecordingMetadata(ctx, &sessionrecordingmetatadav1.ListSessionRecordingMetadataRequest{
		PageSize:    int32(pageSize),
		PageToken:   nextToken,
		SessionIds:  sessionIDs,
		WithSummary: withSummary,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return resp.GetSessionRecordingMetadata(), resp.GetNextPageToken(), nil
}
