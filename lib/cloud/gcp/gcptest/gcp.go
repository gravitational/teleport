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

package gcptest

import (
	"context"

	gcpcredentials "cloud.google.com/go/iam/credentials/apiv1"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/lib/cloud/gcp"
)

type Clients struct {
	GCPSQL       gcp.SQLAdminClient
	GCPAlloyDB   gcp.AlloyDBAdminClient
	GCPGKE       gcp.GKEClient
	GCPProjects  gcp.ProjectsClient
	GCPInstances gcp.InstancesClient
}

var _ gcp.Clients = (*Clients)(nil)

func (c *Clients) Close() error {
	return nil
}

// GetIAMClient returns GCP IAM client.
func (c *Clients) GetIAMClient(ctx context.Context) (*gcpcredentials.IamCredentialsClient, error) {
	return gcpcredentials.NewIamCredentialsClient(ctx,
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())), // Insecure must be set for unauth client.
		option.WithoutAuthentication())
}

// GetSQLAdminClient returns GCP Cloud SQL Admin client.
func (c *Clients) GetSQLAdminClient(ctx context.Context) (gcp.SQLAdminClient, error) {
	return c.GCPSQL, nil
}

func (c *Clients) GetAlloyDBClient(ctx context.Context) (gcp.AlloyDBAdminClient, error) {
	return c.GCPAlloyDB, nil
}

// GetGKEClient returns GKE client.
func (c *Clients) GetGKEClient(ctx context.Context) (gcp.GKEClient, error) {
	return c.GCPGKE, nil
}

// GetProjectsClient returns GCP projects client.
func (c *Clients) GetProjectsClient(ctx context.Context) (gcp.ProjectsClient, error) {
	return c.GCPProjects, nil
}

// GetInstancesClient returns instances client.
func (c *Clients) GetInstancesClient(ctx context.Context) (gcp.InstancesClient, error) {
	return c.GCPInstances, nil
}
