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

package testutil

import (
	"context"

	gcpcredentials "cloud.google.com/go/iam/credentials/apiv1"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/lib/cloud/gcp"
)

type TestGCPClients struct {
	GCPSQL       gcp.SQLAdminClient
	GCPAlloyDB   gcp.AlloyDBAdminClient
	GCPGKE       gcp.GKEClient
	GCPProjects  gcp.ProjectsClient
	GCPInstances gcp.InstancesClient
}

func (c *TestGCPClients) Close() error {
	return nil
}

// GetGCPIAMClient returns GCP IAM client.
func (c *TestGCPClients) GetGCPIAMClient(ctx context.Context) (*gcpcredentials.IamCredentialsClient, error) {
	return gcpcredentials.NewIamCredentialsClient(ctx,
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())), // Insecure must be set for unauth client.
		option.WithoutAuthentication())
}

// GetGCPSQLAdminClient returns GCP Cloud SQL Admin client.
func (c *TestGCPClients) GetGCPSQLAdminClient(ctx context.Context) (gcp.SQLAdminClient, error) {
	return c.GCPSQL, nil
}

func (c *TestGCPClients) GetGCPAlloyDBClient(ctx context.Context) (gcp.AlloyDBAdminClient, error) {
	return c.GCPAlloyDB, nil
}

// GetGCPGKEClient returns GKE client.
func (c *TestGCPClients) GetGCPGKEClient(ctx context.Context) (gcp.GKEClient, error) {
	return c.GCPGKE, nil
}

// GetGCPProjectsClient returns GCP projects client.
func (c *TestGCPClients) GetGCPProjectsClient(ctx context.Context) (gcp.ProjectsClient, error) {
	return c.GCPProjects, nil
}

// GetGCPInstancesClient returns instances client.
func (c *TestGCPClients) GetGCPInstancesClient(ctx context.Context) (gcp.InstancesClient, error) {
	return c.GCPInstances, nil
}
