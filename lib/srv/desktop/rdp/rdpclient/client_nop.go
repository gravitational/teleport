//go:build !desktop_access_rdp
// +build !desktop_access_rdp

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

// This file lets us compile /lib/srv/desktop without including the real RDP
// implementation. Use the desktop_access_rdp build tag to include the
// real implementation.

package rdpclient

import (
	"context"
	"errors"
	"time"
)

// Client is the dummy RDP client.
type Client struct {
}

// New creates and connects a new Client based on opts.
//
//nolint:staticcheck // SA4023. False positive, depends on build tags.
func New(cfg Config) (*Client, error) {
	return nil, errors.New("the real rdpclient.Client implementation was not included in this build")
}

// Run starts the rdp client and blocks until the client disconnects,
// then runs the cleanup.
func (c *Client) Run(ctx context.Context) error {
	return errors.New("the real rdpclient.Client implementation was not included in this build")
}

func (c *Client) GetClientUsername() string {
	return ""
}

// GetClientLastActive returns the time of the last recorded activity.
func (c *Client) GetClientLastActive() time.Time {
	return time.Now().UTC()
}

// UpdateClientActivity updates the client activity timestamp.
func (c *Client) UpdateClientActivity() {}
