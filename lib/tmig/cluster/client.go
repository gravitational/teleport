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

package cluster

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// IdentityFormat describes how the identity is stored.
type IdentityFormat int

const (
	FormatProfile      IdentityFormat = iota // directory-based tsh profile
	FormatIdentityFile                       // exported identity file (tctl auth sign)
)

// Client wraps a connection to a Teleport cluster.
type Client struct {
	proxy        string
	identityPath string
	format       IdentityFormat
}

// Connect creates a cluster client. The actual Teleport client connection
// is established lazily on first API call.
func Connect(ctx context.Context, proxy string, identityPath string) (*Client, error) {
	format, err := DetectIdentityFormat(identityPath)
	if err != nil {
		return nil, fmt.Errorf("detecting identity format for %s: %w", identityPath, err)
	}
	return &Client{
		proxy:        proxy,
		identityPath: identityPath,
		format:       format,
	}, nil
}

// Close releases resources held by the client.
func (c *Client) Close() error {
	return nil // real implementation closes the gRPC connection
}

// DetectIdentityFormat checks whether the path is a directory (profile) or file.
func DetectIdentityFormat(path string) (IdentityFormat, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	if info.IsDir() {
		return FormatProfile, nil
	}
	// Check if it looks like a PEM identity file
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if strings.HasPrefix(string(data), "-----BEGIN") {
		return FormatIdentityFile, nil
	}
	return FormatIdentityFile, nil
}
