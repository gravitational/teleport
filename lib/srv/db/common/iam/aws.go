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

package iam

import (
	"fmt"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	awsutils "github.com/gravitational/teleport/api/utils/aws"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
)

// GetAWSPolicyDocument returns the AWS IAM policy document for provided
// database for setting up the default IAM identity of the database agent.
func GetAWSPolicyDocument(db types.Database) (*awslib.PolicyDocument, Placeholders, error) {
	switch db.GetType() {
	case types.DatabaseTypeRDS, types.DatabaseTypeRDSProxy:
		return getRDSPolicyDocument(db)
	case types.DatabaseTypeRedshift:
		return getRedshiftPolicyDocument(db)
	case types.DatabaseTypeElastiCache:
		return getElastiCachePolicyDocument(db)
	case types.DatabaseTypeElastiCacheServerless:
		return getElastiCacheServerlessPolicyDocument(db)
	case types.DatabaseTypeMemoryDB:
		return getMemoryDBPolicyDocument(db)
	default:
		return nil, nil, trace.BadParameter("GetAWSPolicyDocument is not supported for database type %s", db.GetType())
	}

}

// GetAWSPolicyDocumentForAssumedRole returns the AWS IAM policy document for
// provided database for setting up the IAM role assumed by the database agent.
func GetAWSPolicyDocumentForAssumedRole(db types.Database) (*awslib.PolicyDocument, Placeholders, error) {
	switch db.GetType() {
	case types.DatabaseTypeRedshift:
		return getRedshiftAssumedRolePolicyDocument(db)
	case types.DatabaseTypeRedshiftServerless:
		return getRedshiftServerlessPolicyDocument(db)
	}
	return nil, nil, trace.BadParameter("GetAWSPolicyDocumentForAssumedRole is not supported for database type %s", db.GetType())
}

// GetReadableAWSPolicyDocument returns the indented JSON string of the AWS IAM
// policy document for provided database.
func GetReadableAWSPolicyDocument(db types.Database) (string, error) {
	policyDoc, _, err := GetAWSPolicyDocument(db)
	if err != nil {
		return "", trace.Wrap(err)
	}
	marshaled, err := policyDoc.Marshal()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return marshaled, nil
}

// GetReadableAWSPolicyDocumentForAssumedRole returns the indented JSON string
// of the AWS IAM policy document for provided database.
func GetReadableAWSPolicyDocumentForAssumedRole(db types.Database) (string, error) {
	policyDoc, _, err := GetAWSPolicyDocumentForAssumedRole(db)
	if err != nil {
		return "", trace.Wrap(err)
	}
	marshaled, err := policyDoc.Marshal()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return marshaled, nil
}

// CheckElastiCacheSupportsIAMAuth returns whether the given ElastiCache
// database supports IAM auth.
// AWS ElastiCache Redis supports IAM auth for redis version 7+.
func CheckElastiCacheSupportsIAMAuth(database types.Database) (bool, error) {
	version, ok := database.GetLabel("engine-version")
	if !ok {
		return false, trace.NotFound("database missing engine-version label")
	}
	return checkRedisEngineVersionSupportsIAMAuth(version)
}

func checkRedisEngineVersionSupportsIAMAuth(version string) (bool, error) {
	v, err := semver.NewVersion(strings.TrimPrefix(version, "v"))
	if err != nil {
		return false, trace.Wrap(err, "failed to parse engine-version")
	}
	return v.Major >= 7, nil
}

// CheckMemoryDBSupportsIAMAuth returns whether the given MemoryDB database
// supports IAM auth.
// AWS MemoryDB supports IAM auth for redis version 7+.
func CheckMemoryDBSupportsIAMAuth(database types.Database) (bool, error) {
	version, ok := database.GetLabel("engine-version")
	if !ok {
		return false, trace.NotFound("database missing engine-version label")
	}

	// MemoryDB version may not have patch version (e.g. "7.0")
	if strings.Count(version, ".") == 1 {
		version = version + ".0"
	}
	return checkRedisEngineVersionSupportsIAMAuth(version)
}

func getRDSPolicyDocument(db types.Database) (*awslib.PolicyDocument, Placeholders, error) {
	aws := db.GetAWS()
	partition := awsutils.GetPartitionFromRegion(aws.Region)
	region := aws.Region
	accountID := aws.AccountID
	resourceID := getRDSResourceID(db)

	placeholders := Placeholders(nil).
		setPlaceholderIfEmpty(&region, "{region}").
		setPlaceholderIfEmpty(&partition, "{partition}").
		setPlaceholderIfEmpty(&accountID, "{account_id}").
		setPlaceholderIfEmpty(&resourceID, "{resource_id}")

	policyDoc := awslib.NewPolicyDocument(&awslib.Statement{
		Effect:  awslib.EffectAllow,
		Actions: awslib.SliceOrString{"rds-db:connect"},
		Resources: awslib.SliceOrString{
			fmt.Sprintf("arn:%v:rds-db:%v:%v:dbuser:%v/*", partition, region, accountID, resourceID),
		},
	})
	return policyDoc, placeholders, nil
}

// getRDSResourceID returns the resource ID for RDS or RDS Proxy database.
func getRDSResourceID(db types.Database) string {
	switch db.GetType() {
	case types.DatabaseTypeRDS:
		return db.GetAWS().RDS.ResourceID
	case types.DatabaseTypeRDSProxy:
		return db.GetAWS().RDSProxy.ResourceID
	default:
		return ""
	}
}

func getRedshiftPolicyDocument(db types.Database) (*awslib.PolicyDocument, Placeholders, error) {
	aws := db.GetAWS()
	partition := awsutils.GetPartitionFromRegion(aws.Region)
	region := aws.Region
	accountID := aws.AccountID
	clusterID := aws.Redshift.ClusterID

	placeholders := Placeholders(nil).
		setPlaceholderIfEmpty(&region, "{region}").
		setPlaceholderIfEmpty(&partition, "{partition}").
		setPlaceholderIfEmpty(&accountID, "{account_id}").
		setPlaceholderIfEmpty(&clusterID, "{cluster_id}")

	policyDoc := awslib.NewPolicyDocument(&awslib.Statement{
		Effect:  awslib.EffectAllow,
		Actions: awslib.SliceOrString{"redshift:GetClusterCredentials"},
		Resources: awslib.SliceOrString{
			fmt.Sprintf("arn:%v:redshift:%v:%v:dbuser:%v/*", partition, region, accountID, clusterID),
			fmt.Sprintf("arn:%v:redshift:%v:%v:dbname:%v/*", partition, region, accountID, clusterID),
			fmt.Sprintf("arn:%v:redshift:%v:%v:dbgroup:%v/*", partition, region, accountID, clusterID),
		},
	})
	return policyDoc, placeholders, nil
}

// getRedshiftAssumedRolePolicyDocument returns the policy document used for
// Redshift databases when the database user is an IAM role.
//
// https://docs.aws.amazon.com/service-authorization/latest/reference/list_amazonredshift.html
func getRedshiftAssumedRolePolicyDocument(db types.Database) (*awslib.PolicyDocument, Placeholders, error) {
	aws := db.GetAWS()
	partition := awsutils.GetPartitionFromRegion(aws.Region)
	region := aws.Region
	accountID := aws.AccountID
	clusterID := aws.Redshift.ClusterID

	placeholders := Placeholders(nil).
		setPlaceholderIfEmpty(&region, "{region}").
		setPlaceholderIfEmpty(&partition, "{partition}").
		setPlaceholderIfEmpty(&accountID, "{account_id}").
		setPlaceholderIfEmpty(&clusterID, "{cluster_id}")
	policyDoc := awslib.NewPolicyDocument(&awslib.Statement{
		Effect:  awslib.EffectAllow,
		Actions: awslib.SliceOrString{"redshift:GetClusterCredentialsWithIAM"},
		Resources: awslib.SliceOrString{
			fmt.Sprintf("arn:%v:redshift:%v:%v:dbname:%v/*", partition, region, accountID, clusterID),
		},
	})
	return policyDoc, placeholders, nil
}

// getRedshiftServerlessPolicyDocument returns the policy document used for
// Redshift Serverless databases.
//
// https://docs.aws.amazon.com/service-authorization/latest/reference/list_amazonredshiftserverless.html
func getRedshiftServerlessPolicyDocument(db types.Database) (*awslib.PolicyDocument, Placeholders, error) {
	aws := db.GetAWS()
	partition := awsutils.GetPartitionFromRegion(aws.Region)
	region := aws.Region
	accountID := aws.AccountID
	workgroupID := aws.RedshiftServerless.WorkgroupID

	placeholders := Placeholders(nil).
		setPlaceholderIfEmpty(&region, "{region}").
		setPlaceholderIfEmpty(&partition, "{partition}").
		setPlaceholderIfEmpty(&accountID, "{account_id}").
		setPlaceholderIfEmpty(&workgroupID, "{workgroup_id}")

	policyDoc := awslib.NewPolicyDocument(&awslib.Statement{
		Effect:  awslib.EffectAllow,
		Actions: awslib.SliceOrString{"redshift-serverless:GetCredentials"},
		Resources: awslib.SliceOrString{
			fmt.Sprintf("arn:%v:redshift-serverless:%v:%v:workgroup/%v", partition, region, accountID, workgroupID),
		},
	})
	return policyDoc, placeholders, nil
}

// getElastiCachePolicyDocument returns the policy document used for
// ElastiCache databases.
//
// https://docs.aws.amazon.com/service-authorization/latest/reference/list_amazonelasticache.html
func getElastiCachePolicyDocument(db types.Database) (*awslib.PolicyDocument, Placeholders, error) {
	meta := db.GetAWS()
	partition := awsutils.GetPartitionFromRegion(meta.Region)
	region := meta.Region
	accountID := meta.AccountID
	replicationGroupID := meta.ElastiCache.ReplicationGroupID

	placeholders := Placeholders(nil).
		setPlaceholderIfEmpty(&region, "{region}").
		setPlaceholderIfEmpty(&partition, "{partition}").
		setPlaceholderIfEmpty(&accountID, "{account_id}").
		setPlaceholderIfEmpty(&replicationGroupID, "{replication_group_id}")

	policyDoc := awslib.NewPolicyDocument(&awslib.Statement{
		Effect:  awslib.EffectAllow,
		Actions: awslib.SliceOrString{"elasticache:Connect"},
		Resources: awslib.SliceOrString{
			fmt.Sprintf("arn:%v:elasticache:%v:%v:replicationgroup:%v", partition, region, accountID, replicationGroupID),
			fmt.Sprintf("arn:%v:elasticache:%v:%v:user:*", partition, region, accountID),
		},
	})
	return policyDoc, placeholders, nil
}

// getElastiCacheServerlessPolicyDocument returns the policy document used for
// ElastiCache databases.
//
// https://docs.aws.amazon.com/AmazonElastiCache/latest/dg/auth-iam.html
func getElastiCacheServerlessPolicyDocument(db types.Database) (*awslib.PolicyDocument, Placeholders, error) {
	meta := db.GetAWS()
	partition := awsutils.GetPartitionFromRegion(meta.Region)
	region := meta.Region
	accountID := meta.AccountID
	cacheName := meta.ElastiCacheServerless.CacheName

	placeholders := Placeholders(nil).
		setPlaceholderIfEmpty(&region, "{region}").
		setPlaceholderIfEmpty(&partition, "{partition}").
		setPlaceholderIfEmpty(&accountID, "{account_id}").
		setPlaceholderIfEmpty(&cacheName, "{cache_name}")

	policyDoc := awslib.NewPolicyDocument(&awslib.Statement{
		Effect:  awslib.EffectAllow,
		Actions: awslib.SliceOrString{"elasticache:Connect"},
		Resources: awslib.SliceOrString{
			fmt.Sprintf("arn:%v:elasticache:%v:%v:serverlesscache:%v", partition, region, accountID, cacheName),
			fmt.Sprintf("arn:%v:elasticache:%v:%v:user:*", partition, region, accountID),
		},
	})
	return policyDoc, placeholders, nil
}

// getMemoryDBPolicyDocument returns the policy document used for MemoryDB
// databases.
//
// https://docs.aws.amazon.com/memorydb/latest/devguide/auth-iam.html
func getMemoryDBPolicyDocument(db types.Database) (*awslib.PolicyDocument, Placeholders, error) {
	meta := db.GetAWS()
	partition := awsutils.GetPartitionFromRegion(meta.Region)
	region := meta.Region
	accountID := meta.AccountID
	clusterName := meta.MemoryDB.ClusterName

	placeholders := Placeholders(nil).
		setPlaceholderIfEmpty(&region, "{region}").
		setPlaceholderIfEmpty(&partition, "{partition}").
		setPlaceholderIfEmpty(&accountID, "{account_id}").
		setPlaceholderIfEmpty(&clusterName, "{cluster_name}")

	policyDoc := awslib.NewPolicyDocument(&awslib.Statement{
		Effect:  awslib.EffectAllow,
		Actions: awslib.SliceOrString{"memorydb:Connect"},
		Resources: awslib.SliceOrString{
			// Note that MemoryDB requires `/` to divide resources like
			// `cluster/<cluster_name>`, whereas ElastiCache requires `:` like
			// `replicationgroup:<id>`.
			fmt.Sprintf("arn:%v:memorydb:%v:%v:cluster/%v", partition, region, accountID, clusterName),
			fmt.Sprintf("arn:%v:memorydb:%v:%v:user/*", partition, region, accountID),
		},
	})
	return policyDoc, placeholders, nil
}

// Placeholders defines a slice of strings used as placeholders.
type Placeholders []string

func (s Placeholders) setPlaceholderIfEmpty(attr *string, placeholder string) Placeholders {
	if attr == nil || *attr != "" {
		return s
	}
	*attr = placeholder
	return append(s, placeholder)
}
