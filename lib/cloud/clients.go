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

package cloud

import (
	"io"

	"github.com/gravitational/trace"
)

type Clients interface {
	GCPClients() GCPClients
	AzureClients() AzureClients
	InstanceMetadataClient() InstanceMetadataClient

	io.Closer
}

// NewClients returns default cloud clients using default options, which implies ambient credentials.
func NewClients() (Clients, error) {
	azure, err := newAzureClients()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &clients{
		gcpClients:             newGCPClients(),
		azureClients:           azure,
		instanceMetadataClient: newInstanceMetadataClient(),
	}, nil
}

type clients struct {
	gcpClients             GCPClients
	azureClients           AzureClients
	instanceMetadataClient InstanceMetadataClient
}

func (c *clients) GCPClients() GCPClients {
	return c.gcpClients
}

func (c *clients) AzureClients() AzureClients {
	return c.azureClients
}

func (c *clients) InstanceMetadataClient() InstanceMetadataClient {
	return c.instanceMetadataClient
}

func (c *clients) Close() error {
	return c.gcpClients.Close()
}
