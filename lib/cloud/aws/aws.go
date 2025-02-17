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
	"context"
	"log/slog"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	ectypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	memorydbtypes "github.com/aws/aws-sdk-go-v2/service/memorydb/types"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	"github.com/coreos/go-semver/semver"

	"github.com/gravitational/teleport/lib/services"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// IsResourceAvailable checks if the input status indicates the resource is
// available for use.
//
// Note that this function checks some common values but not necessarily covers
// everything. For types that have other known status values, separate
// functions (e.g. IsDBClusterAvailable) can be implemented.
func IsResourceAvailable(r any, status *string) bool {
	switch strings.ToLower(aws.ToString(status)) {
	case "available", "modifying", "snapshotting", "active":
		return true

	case "creating", "deleting", "create-failed":
		return false

	default:
		slog.WarnContext(context.Background(), "Assuming that AWS resource with an unknown status is available",
			"status", aws.ToString(status),
			"resource", logutils.TypeAttr(r),
		)
		return true
	}
}

// IsElastiCacheClusterAvailable checks if the ElastiCache cluster is
// available.
func IsElastiCacheClusterAvailable(cluster *ectypes.ReplicationGroup) bool {
	return IsResourceAvailable(cluster, cluster.Status)
}

// IsMemoryDBClusterAvailable checks if the MemoryDB cluster is available.
func IsMemoryDBClusterAvailable(cluster *memorydbtypes.Cluster) bool {
	return IsResourceAvailable(cluster, cluster.Status)
}

// IsOpenSearchDomainAvailable checks if the OpenSearch domain is available.
func IsOpenSearchDomainAvailable(domain *opensearchtypes.DomainStatus) bool {
	return aws.ToBool(domain.Created) && !aws.ToBool(domain.Deleted)
}

// IsRDSProxyAvailable checks if the RDS Proxy is available.
func IsRDSProxyAvailable(dbProxy *rdstypes.DBProxy) bool {
	switch dbProxy.Status {
	case
		rdstypes.DBProxyStatusAvailable,
		rdstypes.DBProxyStatusModifying,
		rdstypes.DBProxyStatusReactivating:
		return true
	case
		rdstypes.DBProxyStatusCreating,
		rdstypes.DBProxyStatusDeleting,
		rdstypes.DBProxyStatusIncompatibleNetwork,
		rdstypes.DBProxyStatusInsufficientResourceLimits,
		rdstypes.DBProxyStatusSuspended,
		rdstypes.DBProxyStatusSuspending:
		return false
	}
	slog.WarnContext(context.Background(), "Assuming RDS Proxy with unknown status is available",
		"status", dbProxy.Status,
	)
	return true
}

// IsRDSProxyCustomEndpointAvailable checks if the RDS Proxy custom endpoint is available.
func IsRDSProxyCustomEndpointAvailable(customEndpoint *rdstypes.DBProxyEndpoint) bool {
	switch customEndpoint.Status {
	case
		rdstypes.DBProxyEndpointStatusAvailable,
		rdstypes.DBProxyEndpointStatusModifying:
		return true
	case
		rdstypes.DBProxyEndpointStatusCreating,
		rdstypes.DBProxyEndpointStatusDeleting,
		rdstypes.DBProxyEndpointStatusIncompatibleNetwork,
		rdstypes.DBProxyEndpointStatusInsufficientResourceLimits:
		return false
	}
	slog.WarnContext(context.Background(), "Assuming RDS Proxy custom endpoint with unknown status is available",
		"status", customEndpoint.Status,
	)
	return true
}

// IsRDSInstanceSupported returns true if database supports IAM authentication.
// Currently, only MariaDB is being checked.
func IsRDSInstanceSupported(instance *rdstypes.DBInstance) bool {
	// TODO(jakule): Check other engines.
	if aws.ToString(instance.Engine) != services.RDSEngineMariaDB {
		return true
	}

	// MariaDB follows semver schema: https://mariadb.org/about/
	ver, err := semver.NewVersion(aws.ToString(instance.EngineVersion))
	if err != nil {
		slog.ErrorContext(context.Background(), "Failed to parse RDS MariaDB version", "version", aws.ToString(instance.EngineVersion))
		return false
	}

	// Min supported MariaDB version that supports IAM is 10.6
	// https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.html
	minIAMSupportedVer := semver.New("10.6.0")
	return !ver.LessThan(*minIAMSupportedVer)
}

// IsRDSClusterSupported checks whether the Aurora cluster is supported.
func IsRDSClusterSupported(cluster *rdstypes.DBCluster) bool {
	switch aws.ToString(cluster.EngineMode) {
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
func AuroraMySQLVersion(cluster *rdstypes.DBCluster) string {
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
	version := aws.ToString(cluster.EngineVersion)
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
func IsDocumentDBClusterSupported(cluster *rdstypes.DBCluster) bool {
	ver, err := semver.NewVersion(aws.ToString(cluster.EngineVersion))
	if err != nil {
		slog.ErrorContext(context.Background(), "Failed to parse DocumentDB engine version", "version", aws.ToString(cluster.EngineVersion))
		return false
	}

	return !ver.LessThan(semver.Version{Major: 5})
}

// IsElastiCacheClusterSupported checks whether the ElastiCache cluster is
// supported.
func IsElastiCacheClusterSupported(cluster *ectypes.ReplicationGroup) bool {
	return aws.ToBool(cluster.TransitEncryptionEnabled)
}

// IsMemoryDBClusterSupported checks whether the MemoryDB cluster is supported.
func IsMemoryDBClusterSupported(cluster *memorydbtypes.Cluster) bool {
	return aws.ToBool(cluster.TLSEnabled)
}

// IsRDSInstanceAvailable checks if the RDS instance is available.
func IsRDSInstanceAvailable(instanceStatus, instanceIdentifier *string) bool {
	// For a full list of status values, see:
	// https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/accessing-monitoring.html
	switch aws.ToString(instanceStatus) {
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
		slog.WarnContext(context.Background(), "Assuming RDS instance with unknown status is available",
			"status", aws.ToString(instanceStatus),
			"instance", aws.ToString(instanceIdentifier),
		)
		return true
	}
}

// IsDBClusterAvailable checks if the RDS or DocumentDB cluster is available.
func IsDBClusterAvailable(clusterStatus, clusterIndetifier *string) bool {
	// For a full list of status values, see:
	// https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/accessing-monitoring.html
	switch aws.ToString(clusterStatus) {
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
		slog.WarnContext(context.Background(), "Assuming Aurora cluster with unknown status is available",
			"status", aws.ToString(clusterStatus),
			"cluster", aws.ToString(clusterIndetifier),
		)
		return true
	}
}

// IsRedshiftClusterAvailable checks if the Redshift cluster is available.
func IsRedshiftClusterAvailable(cluster *redshifttypes.Cluster) bool {
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
	switch aws.ToString(cluster.ClusterStatus) {
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
		slog.WarnContext(context.Background(), "Assuming Redshift cluster with unknown status is available",
			"status", aws.ToString(cluster.ClusterStatus),
			"cluster", aws.ToString(cluster.ClusterIdentifier),
		)
		return true
	}
}
