// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package aws

import (
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/aws/aws-sdk-go/service/opensearchservice"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/coreos/go-semver/semver"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/services"
)

// IsResourceAvailable checks if the input status indicates the resource is
// available for use.
//
// Note that this function checks some common values but not necessarily covers
// everything. For types that have other known status values, separate
// functions (e.g. IsRDSClusterAvailable) can be implemented.
func IsResourceAvailable(r interface{}, status *string) bool {
	switch strings.ToLower(aws.StringValue(status)) {
	case "available", "modifying", "snapshotting", "active":
		return true

	case "creating", "deleting", "create-failed":
		return false

	default:
		log.WithField("aws_resource", r).Warnf("Unknown status type: %q. Assuming the AWS resource %T is available.", aws.StringValue(status), r)
		return true
	}
}

// IsElastiCacheClusterAvailable checks if the ElastiCache cluster is
// available.
func IsElastiCacheClusterAvailable(cluster *elasticache.ReplicationGroup) bool {
	return IsResourceAvailable(cluster, cluster.Status)
}

// IsMemoryDBClusterAvailable checks if the MemoryDB cluster is available.
func IsMemoryDBClusterAvailable(cluster *memorydb.Cluster) bool {
	return IsResourceAvailable(cluster, cluster.Status)
}

// IsOpenSearchDomainAvailable checks if the OpenSearch domain is available.
func IsOpenSearchDomainAvailable(domain *opensearchservice.DomainStatus) bool {
	return aws.BoolValue(domain.Created) && !aws.BoolValue(domain.Deleted)
}

// IsRDSProxyAvailable checks if the RDS Proxy is available.
func IsRDSProxyAvailable(dbProxy *rds.DBProxy) bool {
	return IsResourceAvailable(dbProxy, dbProxy.Status)
}

// IsRDSProxyCustomEndpointAvailable checks if the RDS Proxy custom endpoint is available.
func IsRDSProxyCustomEndpointAvailable(customEndpoint *rds.DBProxyEndpoint) bool {
	return IsResourceAvailable(customEndpoint, customEndpoint.Status)
}

// IsRDSInstanceSupported returns true if database supports IAM authentication.
// Currently, only MariaDB is being checked.
func IsRDSInstanceSupported(instance *rds.DBInstance) bool {
	// TODO(jakule): Check other engines.
	if aws.StringValue(instance.Engine) != services.RDSEngineMariaDB {
		return true
	}

	// MariaDB follows semver schema: https://mariadb.org/about/
	ver, err := semver.NewVersion(aws.StringValue(instance.EngineVersion))
	if err != nil {
		log.Errorf("Failed to parse RDS MariaDB version: %s", aws.StringValue(instance.EngineVersion))
		return false
	}

	// Min supported MariaDB version that supports IAM is 10.6
	// https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.html
	minIAMSupportedVer := semver.New("10.6.0")
	return !ver.LessThan(*minIAMSupportedVer)
}

// IsRDSClusterSupported checks whether the Aurora cluster is supported.
func IsRDSClusterSupported(cluster *rds.DBCluster) bool {
	switch aws.StringValue(cluster.EngineMode) {
	// Aurora Serverless v1 does NOT support IAM authentication.
	// https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/aurora-serverless.html#aurora-serverless.limitations
	//
	// Note that Aurora Serverless v2 does support IAM authentication.
	// https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/aurora-serverless-v2.html
	// However, v2's engine mode is "provisioned" instead of "serverless" so it
	// goes to the default case (true).
	case services.RDSEngineModeServerless:
		return false

	// Aurora MySQL 1.22.2, 1.20.1, 1.19.6, and 5.6.10a only: Parallel query doesn't support AWS Identity and Access Management (IAM) database authentication.
	// https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/aurora-mysql-parallel-query.html#aurora-mysql-parallel-query-limitations
	case services.RDSEngineModeParallelQuery:
		if slices.Contains([]string{"1.22.2", "1.20.1", "1.19.6", "5.6.10a"}, AuroraMySQLVersion(cluster)) {
			return false
		}
	}

	return true
}

// AuroraMySQLVersion extracts aurora mysql version from engine version
func AuroraMySQLVersion(cluster *rds.DBCluster) string {
	// version guide: https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/AuroraMySQL.Updates.Versions.html
	// a list of all the available versions: https://docs.aws.amazon.com/cli/latest/reference/rds/describe-db-engine-versions.html
	//
	// some examples of possible inputs:
	// 5.6.10a
	// 5.7.12
	// 5.6.mysql_aurora.1.22.0
	// 5.6.mysql_aurora.1.22.1
	// 5.6.mysql_aurora.1.22.1.3
	//
	// general format is: <mysql-major-version>.mysql_aurora.<aurora-mysql-version>
	// 5.6.10a and 5.7.12 are "legacy" versions and they are returned as it is
	version := aws.StringValue(cluster.EngineVersion)
	parts := strings.Split(version, ".mysql_aurora.")
	if len(parts) == 2 {
		return parts[1]
	}
	return version
}

// IsDocumentDBClusterSupported checks whether IAM authentication is supported
// for this DocumentDB cluster.
//
// https://docs.aws.amazon.com/documentdb/latest/developerguide/iam-identity-auth.html
func IsDocumentDBClusterSupported(cluster *rds.DBCluster) bool {
	ver, err := semver.NewVersion(aws.StringValue(cluster.EngineVersion))
	if err != nil {
		log.Errorf("Failed to parse DocumentDB engine version: %s", aws.StringValue(cluster.EngineVersion))
		return false
	}

	minIAMSupportedVer := semver.New("5.0.0")
	return !ver.LessThan(*minIAMSupportedVer)
}

// IsElastiCacheClusterSupported checks whether the ElastiCache cluster is
// supported.
func IsElastiCacheClusterSupported(cluster *elasticache.ReplicationGroup) bool {
	return aws.BoolValue(cluster.TransitEncryptionEnabled)
}

// IsMemoryDBClusterSupported checks whether the MemoryDB cluster is supported.
func IsMemoryDBClusterSupported(cluster *memorydb.Cluster) bool {
	return aws.BoolValue(cluster.TLSEnabled)
}

// IsRDSInstanceAvailable checks if the RDS instance is available.
func IsRDSInstanceAvailable(instanceStatus, instanceIdentifier *string) bool {
	// For a full list of status values, see:
	// https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/accessing-monitoring.html
	switch aws.StringValue(instanceStatus) {
	// Statuses marked as "Billed" in the above guide.
	case "available", "backing-up", "configuring-enhanced-monitoring",
		"configuring-iam-database-auth", "configuring-log-exports",
		"converting-to-vpc", "incompatible-option-group",
		"incompatible-parameters", "maintenance", "modifying", "moving-to-vpc",
		"rebooting", "resetting-master-credentials", "renaming", "restore-error",
		"storage-full", "storage-optimization", "upgrading":
		return true

	// Statuses marked as "Not billed" in the above guide.
	case "creating", "deleting", "failed",
		"inaccessible-encryption-credentials", "incompatible-network",
		"incompatible-restore":
		return false

	// Statuses marked as "Billed for storage" in the above guide.
	case "inaccessible-encryption-credentials-recoverable", "starting",
		"stopped", "stopping":
		return false

	// Statuses that have no billing information in the above guide, but
	// believed to be unavailable.
	case "insufficient-capacity":
		return false

	default:
		log.Warnf("Unknown status type: %q. Assuming RDS instance %q is available.",
			aws.StringValue(instanceStatus),
			aws.StringValue(instanceIdentifier),
		)
		return true
	}
}

// IsRDSClusterAvailable checks if the RDS cluster is available.
func IsRDSClusterAvailable(clusterStatus, clusterIndetifier *string) bool {
	// For a full list of status values, see:
	// https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/accessing-monitoring.html
	switch aws.StringValue(clusterStatus) {
	// Statuses marked as "Billed" in the above guide.
	case "active", "available", "backing-up", "backtracking", "failing-over",
		"maintenance", "migrating", "modifying", "promoting", "renaming",
		"resetting-master-credentials", "update-iam-db-auth", "upgrading":
		return true

	// Statuses marked as "Not billed" in the above guide.
	case "cloning-failed", "creating", "deleting",
		"inaccessible-encryption-credentials", "migration-failed":
		return false

	// Statuses marked as "Billed for storage" in the above guide.
	case "starting", "stopped", "stopping":
		return false

	default:
		log.Warnf("Unknown status type: %q. Assuming Aurora cluster %q is available.",
			aws.StringValue(clusterStatus),
			aws.StringValue(clusterIndetifier),
		)
		return true
	}
}

// IsDocumentDBClusterAvailable checks if the DocumentDB cluster is available.
func IsDocumentDBClusterAvailable(clusterStatus, clusterIndetifier *string) bool {
	// List of status values for DocumentDB is a subset of RDS's list:
	// https://docs.aws.amazon.com/documentdb/latest/developerguide/monitoring_docdb-cluster_status.html
	return IsRDSClusterAvailable(clusterStatus, clusterIndetifier)
}

// IsRedshiftClusterAvailable checks if the Redshift cluster is available.
func IsRedshiftClusterAvailable(cluster *redshift.Cluster) bool {
	// For a full list of status values, see:
	// https://docs.aws.amazon.com/redshift/latest/mgmt/working-with-clusters.html#rs-mgmt-cluster-status
	//
	// Note that the Redshift guide does not specify billing information like
	// the RDS and Aurora guides do. Most Redshift statuses are
	// cross-referenced with similar statuses from RDS and Aurora guides to
	// determine the availability.
	//
	// For "incompatible-xxx" statuses, the cluster is assumed to be available
	// if the status is resulted by modifying the cluster, and the cluster is
	// assumed to be unavailable if the cluster cannot be created or restored.
	switch aws.StringValue(cluster.ClusterStatus) {
	//nolint:misspell // cancelling is marked as non-existing word
	case "available", "available, prep-for-resize", "available, resize-cleanup",
		"cancelling-resize", "final-snapshot", "modifying", "rebooting",
		"renaming", "resizing", "rotating-keys", "storage-full", "updating-hsm",
		"incompatible-parameters", "incompatible-hsm":
		return true

	case "creating", "deleting", "hardware-failure", "paused",
		"incompatible-network":
		return false

	default:
		log.Warnf("Unknown status type: %q. Assuming Redshift cluster %q is available.",
			aws.StringValue(cluster.ClusterStatus),
			aws.StringValue(cluster.ClusterIdentifier),
		)
		return true
	}
}
