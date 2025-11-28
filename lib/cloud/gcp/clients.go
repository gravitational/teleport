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

package gcp

import (
	"context"
	"io"
	"sync"

	gcpcredentials "cloud.google.com/go/iam/credentials/apiv1"
	"github.com/gravitational/trace"
)

// Clients is an interface for providing GCP API clients.
type Clients interface {
	// GetIAMClient returns GCP IAM client.
	GetIAMClient(context.Context) (*gcpcredentials.IamCredentialsClient, error)
	// GetSQLAdminClient returns GCP Cloud SQL Admin client.
	GetSQLAdminClient(context.Context) (SQLAdminClient, error)
	// GetAlloyDBClient returns GCP AlloyDB Admin client.
	GetAlloyDBClient(context.Context) (AlloyDBAdminClient, error)
	// GetGKEClient returns GKE client.
	GetGKEClient(context.Context) (GKEClient, error)
	// GetProjectsClient returns Projects client.
	GetProjectsClient(context.Context) (ProjectsClient, error)
	// GetInstancesClient returns instances client.
	GetInstancesClient(context.Context) (InstancesClient, error)

	io.Closer
}

// NewClients returns a new instance of GCP SDK clients.
func NewClients() Clients {
	return &clients{
		sqlAdmin:     newClientCache(NewSQLAdminClient),
		alloyDBAdmin: newClientCache(NewAlloyDBAdminClient),
		gke:          newClientCache(NewGKEClient),
		projects:     newClientCache(NewProjectsClient),
		instances:    newClientCache(NewInstancesClient),
	}
}

// clients contains GCP-specific clients.
type clients struct {
	// mtx is used for locking.
	mtx sync.RWMutex

	// iam is the cached GCP IAM client.
	iam *gcpcredentials.IamCredentialsClient
	// sqlAdmin is the cached GCP Cloud SQL Admin client.
	sqlAdmin *clientCache[SQLAdminClient]
	// alloyDBAdmin is the cached GCP AlloyDB Admin client.
	alloyDBAdmin *clientCache[AlloyDBAdminClient]
	// gke is the cached GCP Cloud GKE client.
	gke *clientCache[GKEClient]
	// projects is the cached GCP Cloud Projects client.
	projects *clientCache[ProjectsClient]
	// instances is the cached GCP instances client.
	instances *clientCache[InstancesClient]
}

// GetIAMClient returns GCP IAM client.
func (c *clients) GetIAMClient(ctx context.Context) (*gcpcredentials.IamCredentialsClient, error) {
	c.mtx.RLock()
	if c.iam != nil {
		defer c.mtx.RUnlock()
		return c.iam, nil
	}
	c.mtx.RUnlock()
	return c.initIAMClient(ctx)
}

// GetSQLAdminClient returns GCP Cloud SQL Admin client.
func (c *clients) GetSQLAdminClient(ctx context.Context) (SQLAdminClient, error) {
	return c.sqlAdmin.GetClient(ctx)
}

// GetAlloyDBClient returns GCP AlloyDB Admin client.
func (c *clients) GetAlloyDBClient(ctx context.Context) (AlloyDBAdminClient, error) {
	return c.alloyDBAdmin.GetClient(ctx)
}

// GetGKEClient returns GKE client.
func (c *clients) GetGKEClient(ctx context.Context) (GKEClient, error) {
	return c.gke.GetClient(ctx)
}

// GetProjectsClient returns Project client.
func (c *clients) GetProjectsClient(ctx context.Context) (ProjectsClient, error) {
	return c.projects.GetClient(ctx)
}

// GetInstancesClient returns instances client.
func (c *clients) GetInstancesClient(ctx context.Context) (InstancesClient, error) {
	return c.instances.GetClient(ctx)
}

// Close closes all initialized clients.
func (c *clients) Close() (err error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.iam != nil {
		err = c.iam.Close()
		c.iam = nil
	}
	return trace.Wrap(err)
}

func (c *clients) initIAMClient(ctx context.Context) (*gcpcredentials.IamCredentialsClient, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.iam != nil { // If some other thread already got here first.
		return c.iam, nil
	}
	gcpIAM, err := gcpcredentials.NewIamCredentialsClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.iam = gcpIAM
	return gcpIAM, nil
}
