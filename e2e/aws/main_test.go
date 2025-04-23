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

package e2e

import (
	"testing"

	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// awsRegionEnv is the environment variable that specifies the AWS region
	// where the EKS cluster is running.
	awsRegionEnv = "AWS_REGION"
	// discoveryMatcherLabelsEnv is the env variable that specifies the matcher
	// labels to use in test discovery services.
	discoveryMatcherLabelsEnv = "DISCOVERY_MATCHER_LABELS"
	// rdsAccessRoleARNEnv is the environment variable that specifies the IAM
	// role ARN that Teleport Database Service will assume to access RDS
	// databases.
	// See modules/databases-ci/ from cloud-terraform repo for more details.
	rdsAccessRoleARNEnv = "RDS_ACCESS_ROLE"
	// rdsDiscoveryRoleARNEnv is the environment variable that specifies the
	// IAM role ARN that Teleport Discovery Service will assume to discover
	// RDS databases.
	// See modules/databases-ci/ from cloud-terraform repo for more details.
	rdsDiscoveryRoleARNEnv = "RDS_DISCOVERY_ROLE"
	// rdsPostgresInstanceNameEnv is the environment variable that specifies the
	// name of the RDS Postgres DB instance that will be created by the Teleport
	// Discovery Service.
	rdsPostgresInstanceNameEnv = "RDS_POSTGRES_INSTANCE_NAME"
	// rdsMySQLInstanceNameEnv is the environment variable that specifies the
	// name of the RDS MySQL DB instance that will be created by the Teleport
	// Discovery Service.
	rdsMySQLInstanceNameEnv = "RDS_MYSQL_INSTANCE_NAME"
	// rdsMariaDBInstanceNameEnv is the environment variable that specifies the
	// name of the RDS MariaDB instance that will be created by the Teleport
	// Discovery Service.
	rdsMariaDBInstanceNameEnv = "RDS_MARIADB_INSTANCE_NAME"
	// rssAccessRoleARNEnv is the environment variable that specifies the IAM
	// role ARN that Teleport Database Service will assume to access Redshift
	// Serverless databases.
	// See modules/databases-ci/ from cloud-terraform repo for more details.
	rssAccessRoleARNEnv = "REDSHIFT_SERVERLESS_ACCESS_ROLE"
	// rssDiscoveryRoleARNEnv is the environment variable that specifies the
	// IAM role ARN that Teleport Discovery Service will assume to discover
	// Redshift Serverless databases.
	// See modules/databases-ci/ from cloud-terraform repo for more details.
	rssDiscoveryRoleARNEnv = "REDSHIFT_SERVERLESS_DISCOVERY_ROLE"
	// rssNameEnv is the environment variable that specifies the
	// name of the Redshift Serverless workgroup that will be created by the
	// Teleport Discovery Service.
	rssNameEnv = "REDSHIFT_SERVERLESS_WORKGROUP_NAME"
	// rssEndpointNameEnv is the environment variable that specifies the
	// name of the Redshift Serverless workgroup's access endpoint that
	// will be created by the Teleport Discovery Service.
	rssEndpointNameEnv = "REDSHIFT_SERVERLESS_ENDPOINT_NAME"
	// rssDBUserEnv is the name of the IAM role that tests will use as a
	// database user to connect to Redshift Serverless.
	rssDBUserEnv = "REDSHIFT_SERVERLESS_IAM_DB_USER"
	// redshiftAccessRoleARNEnv is the environment variable that specifies the
	// IAM role ARN that Teleport Database Service will assume to access Redshift
	// cluster databases.
	// See modules/databases-ci/ from cloud-terraform repo for more details.
	redshiftAccessRoleARNEnv = "REDSHIFT_ACCESS_ROLE"
	// redshiftDiscoveryRoleARNEnv is the environment variable that specifies the
	// IAM role ARN that Teleport Discovery Service will assume to discover
	// Redshift cluster databases.
	// See modules/databases-ci/ from cloud-terraform repo for more details.
	redshiftDiscoveryRoleARNEnv = "REDSHIFT_DISCOVERY_ROLE"
	// redshiftNameEnv is the environment variable that specifies the
	// name of the Redshift cluster db that will be created by the
	// Teleport Discovery Service.
	redshiftNameEnv = "REDSHIFT_CLUSTER_NAME"
	// redshiftIAMDBUserEnv is the name of the IAM role that tests will use as a
	// database user to connect to Redshift Serverless.
	redshiftIAMDBUserEnv = "REDSHIFT_IAM_DB_USER"
	// kubeSvcRoleARNEnv is the environment variable that specifies
	// the IAM role that Teleport Kubernetes Service will assume to access the EKS cluster.
	// This role needs to have the following permissions:
	// - eks:DescribeCluster
	// But it also requires the role to be mapped to a Kubernetes group with the following RBAC permissions:
	//	apiVersion: rbac.authorization.k8s.io/v1
	//	kind: ClusterRole
	//	metadata:
	//		name: teleport-role
	//	rules:
	//	- apiGroups:
	//		- ""
	//		resources:
	//		- users
	//		- groups
	//		- serviceaccounts
	//		verbs:
	//		- impersonate
	//	- apiGroups:
	//		- ""
	//		resources:
	//		- pods
	//		verbs:
	//		- get
	//	- apiGroups:
	//		- "authorization.k8s.io"
	//		resources:
	//		- selfsubjectaccessreviews
	//		- selfsubjectrulesreviews
	//		verbs:
	//		- create
	// check modules/eks-discovery-ci/ from cloud-terraform repo for more details.
	kubeSvcRoleARNEnv = "KUBERNETES_SERVICE_ASSUME_ROLE"
	// kubeDiscoverySvcRoleARNEnv is the environment variable that specifies
	// the IAM role that Teleport Discovery Service will assume to list the EKS clusters.
	// This role needs to have the following permissions:
	// - eks:DescribeCluster
	// - eks:ListClusters
	// check modules/eks-discovery-ci/ from cloud-terraform repo for more details.
	kubeDiscoverySvcRoleARNEnv = "KUBE_DISCOVERY_SERVICE_ASSUME_ROLE"
	// eksClusterNameEnv is the environment variable that specifies the name of
	// the EKS cluster that will be created by Teleport Discovery Service.
	eksClusterNameEnv = "EKS_CLUSTER_NAME"
)

// TestMain will re-execute Teleport to run a command if "exec" is passed to
// it as an argument. Otherwise, it will run tests as normal.
func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	// agents connect over a reverse tunnel to proxy, so we use insecure mode.
	lib.SetInsecureDevMode(true)
	helpers.TestMainImplementation(m)
}
