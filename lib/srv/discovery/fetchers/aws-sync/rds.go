/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package aws_sync

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

// rdsClient defines a subset of the AWS RDS client API.
type rdsClient interface {
	rds.DescribeDBClustersAPIClient
	rds.DescribeDBInstancesAPIClient
}

// pollAWSRDSDatabases is a function that returns a function that fetches
// RDS instances and clusters.
func (a *Fetcher) pollAWSRDSDatabases(ctx context.Context, result *Resources, collectErr func(error)) func() error {
	return func() error {
		var err error
		result.RDSDatabases, err = a.fetchAWSRDSDatabases(ctx)
		if err != nil {
			collectErr(trace.Wrap(err, "failed to fetch databases"))
		}
		return nil
	}
}

// fetchAWSRDSDatabases fetches RDS databases from all regions.
func (a *Fetcher) fetchAWSRDSDatabases(ctx context.Context) (
	[]*accessgraphv1alpha.AWSRDSDatabaseV1,
	error,
) {
	var (
		dbs     []*accessgraphv1alpha.AWSRDSDatabaseV1
		hostsMu sync.Mutex
		errs    []error
	)
	eG, ctx := errgroup.WithContext(ctx)
	// Set the limit to 5 to avoid too many concurrent requests.
	// This is a temporary solution until we have a better way to limit the
	// number of concurrent requests.
	eG.SetLimit(5)
	collectDBs := func(db []*accessgraphv1alpha.AWSRDSDatabaseV1, err error) {
		hostsMu.Lock()
		defer hostsMu.Unlock()
		if err != nil {
			errs = append(errs, err)
		}
		if db != nil {
			dbs = append(dbs, db...)
		}

	}

	for _, region := range a.Regions {
		eG.Go(func() error {
			awsCfg, err := a.AWSConfigProvider.GetConfig(ctx, region, a.getAWSOptions()...)
			if err != nil {
				collectDBs(nil, trace.Wrap(err))
				return nil
			}
			clt := a.awsClients.getRDSClient(awsCfg)
			a.collectDBInstances(ctx, clt, region, collectDBs)
			a.collectDBClusters(ctx, clt, region, collectDBs)
			return nil
		})
	}

	err := eG.Wait()
	return dbs, trace.NewAggregate(append(errs, err)...)
}

// awsRDSInstanceToRDS converts an rdstypes.DBInstance to accessgraphv1alpha.AWSRDSDatabaseV1
// representation.
func awsRDSInstanceToRDS(instance *rdstypes.DBInstance, region, accountID string) *accessgraphv1alpha.AWSRDSDatabaseV1 {
	var tags []*accessgraphv1alpha.AWSTag
	for _, v := range instance.TagList {
		tags = append(tags, &accessgraphv1alpha.AWSTag{
			Key:   aws.ToString(v.Key),
			Value: strPtrToWrapper(v.Value),
		})
	}

	return &accessgraphv1alpha.AWSRDSDatabaseV1{
		Name:      aws.ToString(instance.DBInstanceIdentifier),
		Arn:       aws.ToString(instance.DBInstanceArn),
		CreatedAt: awsTimeToProtoTime(instance.InstanceCreateTime),
		Status:    aws.ToString(instance.DBInstanceStatus),
		Region:    region,
		AccountId: accountID,
		Tags:      tags,
		EngineDetails: &accessgraphv1alpha.AWSRDSEngineV1{
			Engine:  aws.ToString(instance.Engine),
			Version: aws.ToString(instance.EngineVersion),
		},
		IsCluster:    false,
		ResourceId:   aws.ToString(instance.DbiResourceId),
		LastSyncTime: timestamppb.Now(),
	}
}

// awsRDSInstanceToRDS converts an rdstypes.DBCluster to accessgraphv1alpha.AWSRDSDatabaseV1
// representation.
func awsRDSClusterToRDS(instance *rdstypes.DBCluster, region, accountID string) *accessgraphv1alpha.AWSRDSDatabaseV1 {
	var tags []*accessgraphv1alpha.AWSTag
	for _, v := range instance.TagList {
		tags = append(tags, &accessgraphv1alpha.AWSTag{
			Key:   aws.ToString(v.Key),
			Value: strPtrToWrapper(v.Value),
		})
	}

	return &accessgraphv1alpha.AWSRDSDatabaseV1{
		Name:      aws.ToString(instance.DBClusterIdentifier),
		Arn:       aws.ToString(instance.DBClusterArn),
		CreatedAt: awsTimeToProtoTime(instance.ClusterCreateTime),
		Status:    aws.ToString(instance.Status),
		Region:    region,
		AccountId: accountID,
		Tags:      tags,
		EngineDetails: &accessgraphv1alpha.AWSRDSEngineV1{
			Engine:  aws.ToString(instance.Engine),
			Version: aws.ToString(instance.EngineVersion),
		},
		IsCluster:    true,
		ResourceId:   aws.ToString(instance.DbClusterResourceId),
		LastSyncTime: timestamppb.Now(),
	}
}

func (a *Fetcher) collectDBInstances(ctx context.Context,
	clt rdsClient,
	region string,
	collectDBs func([]*accessgraphv1alpha.AWSRDSDatabaseV1, error),
) {
	pager := rds.NewDescribeDBInstancesPaginator(clt,
		&rds.DescribeDBInstancesInput{},
		func(ddpo *rds.DescribeDBInstancesPaginatorOptions) {
			ddpo.StopOnDuplicateToken = true
		},
	)
	var instances []*accessgraphv1alpha.AWSRDSDatabaseV1
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			old := sliceFilter(a.lastResult.RDSDatabases, func(db *accessgraphv1alpha.AWSRDSDatabaseV1) bool {
				return !db.IsCluster && db.Region == region && db.AccountId == a.AccountID
			})
			collectDBs(old, trace.Wrap(err))
			return
		}
		for _, db := range page.DBInstances {
			// if instance belongs to a cluster, skip it as we want to represent the cluster itself
			// and we pull it using DescribeDBClustersPaginator instead.
			if aws.ToString(db.DBClusterIdentifier) != "" {
				continue
			}
			protoRDS := awsRDSInstanceToRDS(&db, region, a.AccountID)
			instances = append(instances, protoRDS)
		}
	}
	collectDBs(instances, nil)
}

func (a *Fetcher) collectDBClusters(
	ctx context.Context,
	clt rdsClient,
	region string,
	collectDBs func([]*accessgraphv1alpha.AWSRDSDatabaseV1, error),
) {
	pager := rds.NewDescribeDBClustersPaginator(clt, &rds.DescribeDBClustersInput{},
		func(ddpo *rds.DescribeDBClustersPaginatorOptions) {
			ddpo.StopOnDuplicateToken = true
		},
	)
	var clusters []*accessgraphv1alpha.AWSRDSDatabaseV1
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			old := sliceFilter(a.lastResult.RDSDatabases, func(db *accessgraphv1alpha.AWSRDSDatabaseV1) bool {
				return db.IsCluster && db.Region == region && db.AccountId == a.AccountID
			})
			collectDBs(old, trace.Wrap(err))
			return
		}
		for _, db := range page.DBClusters {
			protoRDS := awsRDSClusterToRDS(&db, region, a.AccountID)
			clusters = append(clusters, protoRDS)
		}
	}
	collectDBs(clusters, nil)
}
