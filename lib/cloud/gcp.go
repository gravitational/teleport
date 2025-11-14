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
	"context"
	"io"
	"sync"

	gcpcredentials "cloud.google.com/go/iam/credentials/apiv1"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/cloud/gcp"
)

// GCPClients is an interface for providing GCP API clients.
type GCPClients interface {
	// GetGCPIAMClient returns GCP IAM client.
	GetGCPIAMClient(context.Context) (*gcpcredentials.IamCredentialsClient, error)
	// GetGCPSQLAdminClient returns GCP Cloud SQL Admin client.
	GetGCPSQLAdminClient(context.Context) (gcp.SQLAdminClient, error)
	// GetGCPAlloyDBClient returns GCP AlloyDB Admin client.
	GetGCPAlloyDBClient(context.Context) (gcp.AlloyDBAdminClient, error)
	// GetGCPGKEClient returns GKE client.
	GetGCPGKEClient(context.Context) (gcp.GKEClient, error)
	// GetGCPProjectsClient returns Projects client.
	GetGCPProjectsClient(context.Context) (gcp.ProjectsClient, error)
	// GetGCPInstancesClient returns instances client.
	GetGCPInstancesClient(context.Context) (gcp.InstancesClient, error)

	io.Closer
}

var _ GCPClients = (*gcpClients)(nil)

func newGCPClients() *gcpClients {
	return &gcpClients{
		gcpSQLAdmin:     newClientCache(gcp.NewSQLAdminClient),
		gcpAlloyDBAdmin: newClientCache(gcp.NewAlloyDBAdminClient),
		gcpGKE:          newClientCache(gcp.NewGKEClient),
		gcpProjects:     newClientCache(gcp.NewProjectsClient),
		gcpInstances:    newClientCache(gcp.NewInstancesClient),
	}
}

func NewGCPClients() GCPClients {
	return newGCPClients()
}

// gcpClients contains GCP-specific clients.
type gcpClients struct {
	// mtx is used for locking.
	mtx sync.RWMutex

	// gcpIAM is the cached GCP IAM client.
	gcpIAM *gcpcredentials.IamCredentialsClient
	// gcpSQLAdmin is the cached GCP Cloud SQL Admin client.
	gcpSQLAdmin *clientCache[gcp.SQLAdminClient]
	// gcpAlloyDBAdmin is the cached GCP AlloyDB Admin client.
	gcpAlloyDBAdmin *clientCache[gcp.AlloyDBAdminClient]
	// gcpGKE is the cached GCP Cloud GKE client.
	gcpGKE *clientCache[gcp.GKEClient]
	// gcpProjects is the cached GCP Cloud Projects client.
	gcpProjects *clientCache[gcp.ProjectsClient]
	// gcpInstances is the cached GCP instances client.
	gcpInstances *clientCache[gcp.InstancesClient]
}

// GetGCPIAMClient returns GCP IAM client.
func (c *gcpClients) GetGCPIAMClient(ctx context.Context) (*gcpcredentials.IamCredentialsClient, error) {
	c.mtx.RLock()
	if c.gcpIAM != nil {
		defer c.mtx.RUnlock()
		return c.gcpIAM, nil
	}
	c.mtx.RUnlock()
	return c.initGCPIAMClient(ctx)
}

// GetGCPSQLAdminClient returns GCP Cloud SQL Admin client.
func (c *gcpClients) GetGCPSQLAdminClient(ctx context.Context) (gcp.SQLAdminClient, error) {
	return c.gcpSQLAdmin.GetClient(ctx)
}

// GetGCPAlloyDBClient returns GCP AlloyDB Admin client.
func (c *gcpClients) GetGCPAlloyDBClient(ctx context.Context) (gcp.AlloyDBAdminClient, error) {
	return c.gcpAlloyDBAdmin.GetClient(ctx)
}

// GetGCPGKEClient returns GKE client.
func (c *gcpClients) GetGCPGKEClient(ctx context.Context) (gcp.GKEClient, error) {
	return c.gcpGKE.GetClient(ctx)
}

// GetGCPProjectsClient returns Project client.
func (c *gcpClients) GetGCPProjectsClient(ctx context.Context) (gcp.ProjectsClient, error) {
	return c.gcpProjects.GetClient(ctx)
}

// GetGCPInstancesClient returns instances client.
func (c *gcpClients) GetGCPInstancesClient(ctx context.Context) (gcp.InstancesClient, error) {
	return c.gcpInstances.GetClient(ctx)
}

func (c *gcpClients) initGCPIAMClient(ctx context.Context) (*gcpcredentials.IamCredentialsClient, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.gcpIAM != nil { // If some other thread already got here first.
		return c.gcpIAM, nil
	}
	gcpIAM, err := gcpcredentials.NewIamCredentialsClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.gcpIAM = gcpIAM
	return gcpIAM, nil
}

func (c *gcpClients) Close() error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if c.gcpIAM != nil {
		gcpIAM := c.gcpIAM
		c.gcpIAM = nil
		return gcpIAM.Close()
	}

	return nil
}
