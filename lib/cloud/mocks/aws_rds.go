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

package mocks

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdsv2 "github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/cloud/awstesthelpers"
)

type RDSClient struct {
	Unauth bool

	DBInstances      []rdstypes.DBInstance
	DBClusters       []rdstypes.DBCluster
	DBProxies        []rdstypes.DBProxy
	DBProxyEndpoints []rdstypes.DBProxyEndpoint
	DBEngineVersions []rdstypes.DBEngineVersion
}

func (c *RDSClient) DescribeDBInstances(_ context.Context, input *rdsv2.DescribeDBInstancesInput, _ ...func(*rdsv2.Options)) (*rdsv2.DescribeDBInstancesOutput, error) {
	if c.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

	if err := checkEngineFilters(input.Filters, c.DBEngineVersions); err != nil {
		return nil, trace.Wrap(err)
	}
	instances, err := applyInstanceFilters(c.DBInstances, input.Filters)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if aws.ToString(input.DBInstanceIdentifier) == "" {
		return &rdsv2.DescribeDBInstancesOutput{
			DBInstances: instances,
		}, nil
	}
	for _, instance := range instances {
		if aws.ToString(instance.DBInstanceIdentifier) == aws.ToString(input.DBInstanceIdentifier) {
			return &rdsv2.DescribeDBInstancesOutput{
				DBInstances: []rdstypes.DBInstance{instance},
			}, nil
		}
	}
	return nil, trace.NotFound("instance %v not found", aws.ToString(input.DBInstanceIdentifier))
}

func (c *RDSClient) DescribeDBClusters(_ context.Context, input *rdsv2.DescribeDBClustersInput, _ ...func(*rdsv2.Options)) (*rdsv2.DescribeDBClustersOutput, error) {
	if c.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

	if err := checkEngineFilters(input.Filters, c.DBEngineVersions); err != nil {
		return nil, trace.Wrap(err)
	}
	clusters, err := applyClusterFilters(c.DBClusters, input.Filters)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if aws.ToString(input.DBClusterIdentifier) == "" {
		return &rdsv2.DescribeDBClustersOutput{
			DBClusters: clusters,
		}, nil
	}
	for _, cluster := range clusters {
		if aws.ToString(cluster.DBClusterIdentifier) == aws.ToString(input.DBClusterIdentifier) {
			return &rdsv2.DescribeDBClustersOutput{
				DBClusters: []rdstypes.DBCluster{cluster},
			}, nil
		}
	}
	return nil, trace.NotFound("cluster %v not found", aws.ToString(input.DBClusterIdentifier))
}

func (c *RDSClient) ModifyDBInstance(ctx context.Context, input *rdsv2.ModifyDBInstanceInput, optFns ...func(*rdsv2.Options)) (*rdsv2.ModifyDBInstanceOutput, error) {
	if c.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

	for i, instance := range c.DBInstances {
		if aws.ToString(instance.DBInstanceIdentifier) == aws.ToString(input.DBInstanceIdentifier) {
			if aws.ToBool(input.EnableIAMDatabaseAuthentication) {
				c.DBInstances[i].IAMDatabaseAuthenticationEnabled = aws.Bool(true)
			}
			return &rdsv2.ModifyDBInstanceOutput{
				DBInstance: &c.DBInstances[i],
			}, nil
		}
	}
	return nil, trace.NotFound("instance %v not found", aws.ToString(input.DBInstanceIdentifier))
}

func (c *RDSClient) ModifyDBCluster(ctx context.Context, input *rdsv2.ModifyDBClusterInput, optFns ...func(*rdsv2.Options)) (*rdsv2.ModifyDBClusterOutput, error) {
	if c.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

	for i, cluster := range c.DBClusters {
		if aws.ToString(cluster.DBClusterIdentifier) == aws.ToString(input.DBClusterIdentifier) {
			if aws.ToBool(input.EnableIAMDatabaseAuthentication) {
				c.DBClusters[i].IAMDatabaseAuthenticationEnabled = aws.Bool(true)
			}
			return &rdsv2.ModifyDBClusterOutput{
				DBCluster: &c.DBClusters[i],
			}, nil
		}
	}
	return nil, trace.NotFound("cluster %v not found", aws.ToString(input.DBClusterIdentifier))
}

func (c *RDSClient) DescribeDBProxies(_ context.Context, input *rdsv2.DescribeDBProxiesInput, _ ...func(*rdsv2.Options)) (*rdsv2.DescribeDBProxiesOutput, error) {
	if c.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

	if aws.ToString(input.DBProxyName) == "" {
		return &rdsv2.DescribeDBProxiesOutput{
			DBProxies: c.DBProxies,
		}, nil
	}
	for _, dbProxy := range c.DBProxies {
		if aws.ToString(dbProxy.DBProxyName) == aws.ToString(input.DBProxyName) {
			return &rdsv2.DescribeDBProxiesOutput{
				DBProxies: []rdstypes.DBProxy{dbProxy},
			}, nil
		}
	}
	return nil, trace.NotFound("proxy %v not found", aws.ToString(input.DBProxyName))
}

func (c *RDSClient) DescribeDBProxyEndpoints(_ context.Context, input *rdsv2.DescribeDBProxyEndpointsInput, _ ...func(*rdsv2.Options)) (*rdsv2.DescribeDBProxyEndpointsOutput, error) {
	if c.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

	inputProxyName := aws.ToString(input.DBProxyName)
	inputProxyEndpointName := aws.ToString(input.DBProxyEndpointName)

	if inputProxyName == "" && inputProxyEndpointName == "" {
		return &rdsv2.DescribeDBProxyEndpointsOutput{
			DBProxyEndpoints: c.DBProxyEndpoints,
		}, nil
	}

	var endpoints []rdstypes.DBProxyEndpoint
	for _, dbProxyEndpoiont := range c.DBProxyEndpoints {
		if inputProxyEndpointName != "" &&
			inputProxyEndpointName != aws.ToString(dbProxyEndpoiont.DBProxyEndpointName) {
			continue
		}

		if inputProxyName != "" &&
			inputProxyName != aws.ToString(dbProxyEndpoiont.DBProxyName) {
			continue
		}

		endpoints = append(endpoints, dbProxyEndpoiont)
	}
	if len(endpoints) == 0 {
		return nil, trace.NotFound("proxy endpoint %v not found", aws.ToString(input.DBProxyEndpointName))
	}
	return &rdsv2.DescribeDBProxyEndpointsOutput{DBProxyEndpoints: endpoints}, nil
}

func (c *RDSClient) ListTagsForResource(context.Context, *rds.ListTagsForResourceInput, ...func(*rds.Options)) (*rds.ListTagsForResourceOutput, error) {
	return &rds.ListTagsForResourceOutput{}, nil
}

// checkEngineFilters checks RDS filters to detect unrecognized engine filters.
func checkEngineFilters(filters []rdstypes.Filter, engineVersions []rdstypes.DBEngineVersion) error {
	if len(filters) == 0 {
		return nil
	}
	recognizedEngines := make(map[string]struct{})
	for _, e := range engineVersions {
		recognizedEngines[aws.ToString(e.Engine)] = struct{}{}
	}
	for _, f := range filters {
		if aws.ToString(f.Name) != "engine" {
			continue
		}
		for _, v := range f.Values {
			if _, ok := recognizedEngines[v]; !ok {
				return trace.Errorf("unrecognized engine name %q", v)
			}
		}
	}
	return nil
}

// applyInstanceFilters filters RDS DBInstances using the provided RDS filters.
func applyInstanceFilters(in []rdstypes.DBInstance, filters []rdstypes.Filter) ([]rdstypes.DBInstance, error) {
	if len(filters) == 0 {
		return in, nil
	}
	var out []rdstypes.DBInstance
	efs := engineFilterSet(filters)
	clusterIDs := clusterIdentifierFilterSet(filters)
	for _, instance := range in {
		if len(efs) > 0 && !instanceEngineMatches(instance, efs) {
			continue
		}
		if len(clusterIDs) > 0 && !instanceClusterIDMatches(instance, clusterIDs) {
			continue
		}
		out = append(out, instance)
	}
	return out, nil
}

// applyClusterFilters filters RDS DBClusters using the provided RDS filters.
func applyClusterFilters(in []rdstypes.DBCluster, filters []rdstypes.Filter) ([]rdstypes.DBCluster, error) {
	if len(filters) == 0 {
		return in, nil
	}
	var out []rdstypes.DBCluster
	efs := engineFilterSet(filters)
	for _, cluster := range in {
		if clusterEngineMatches(cluster, efs) {
			out = append(out, cluster)
		}
	}
	return out, nil
}

// engineFilterSet builds a string set of engine names from a list of RDS filters.
func engineFilterSet(filters []rdstypes.Filter) map[string]struct{} {
	return filterValues(filters, "engine")
}

// clusterIdentifierFilterSet builds a string set of ClusterIDs from a list of RDS filters.
func clusterIdentifierFilterSet(filters []rdstypes.Filter) map[string]struct{} {
	return filterValues(filters, "db-cluster-id")
}

func filterValues(filters []rdstypes.Filter, filterKey string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, f := range filters {
		if aws.ToString(f.Name) != filterKey {
			continue
		}
		for _, v := range f.Values {
			out[v] = struct{}{}
		}
	}
	return out
}

// instanceEngineMatches returns whether an RDS DBInstance engine matches any engine name in a filter set.
func instanceEngineMatches(instance rdstypes.DBInstance, filterSet map[string]struct{}) bool {
	_, ok := filterSet[aws.ToString(instance.Engine)]
	return ok
}

// instanceClusterIDMatches returns whether an RDS DBInstance ClusterID matches any ClusterID in a filter set.
func instanceClusterIDMatches(instance rdstypes.DBInstance, filterSet map[string]struct{}) bool {
	_, ok := filterSet[aws.ToString(instance.DBClusterIdentifier)]
	return ok
}

// clusterEngineMatches returns whether an RDS DBCluster engine matches any engine name in a filter set.
func clusterEngineMatches(cluster rdstypes.DBCluster, filterSet map[string]struct{}) bool {
	_, ok := filterSet[aws.ToString(cluster.Engine)]
	return ok
}

// RDSInstance returns a sample rdstypes.DBInstance.
func RDSInstance(name, region string, labels map[string]string, opts ...func(*rdstypes.DBInstance)) *rdstypes.DBInstance {
	instance := &rdstypes.DBInstance{
		DBInstanceArn:        aws.String(fmt.Sprintf("arn:aws:rds:%v:123456789012:db:%v", region, name)),
		DBInstanceIdentifier: aws.String(name),
		DbiResourceId:        aws.String(uuid.New().String()),
		Engine:               aws.String("postgres"),
		DBInstanceStatus:     aws.String("available"),
		Endpoint: &rdstypes.Endpoint{
			Address: aws.String(fmt.Sprintf("%v.aabbccdd.%v.rds.amazonaws.com", name, region)),
			Port:    aws.Int32(5432),
		},
		TagList: awstesthelpers.LabelsToRDSTags(labels),
	}
	for _, opt := range opts {
		opt(instance)
	}
	return instance
}

// RDSCluster returns a sample rdstypes.DBCluster.
func RDSCluster(name, region string, labels map[string]string, opts ...func(*rdstypes.DBCluster)) *rdstypes.DBCluster {
	cluster := &rdstypes.DBCluster{
		DBClusterArn:        aws.String(fmt.Sprintf("arn:aws:rds:%v:123456789012:cluster:%v", region, name)),
		DBClusterIdentifier: aws.String(name),
		DbClusterResourceId: aws.String(uuid.New().String()),
		Engine:              aws.String("aurora-mysql"),
		EngineMode:          aws.String("provisioned"),
		Status:              aws.String("available"),
		Endpoint:            aws.String(fmt.Sprintf("%v.cluster-aabbccdd.%v.rds.amazonaws.com", name, region)),
		ReaderEndpoint:      aws.String(fmt.Sprintf("%v.cluster-ro-aabbccdd.%v.rds.amazonaws.com", name, region)),
		Port:                aws.Int32(3306),
		TagList:             awstesthelpers.LabelsToRDSTags(labels),
		DBClusterMembers: []rdstypes.DBClusterMember{{
			IsClusterWriter: aws.Bool(true), // One writer by default.
		}},
	}
	for _, opt := range opts {
		opt(cluster)
	}
	return cluster
}

func WithRDSClusterReader(cluster *rdstypes.DBCluster) {
	cluster.DBClusterMembers = append(cluster.DBClusterMembers, rdstypes.DBClusterMember{
		IsClusterWriter: aws.Bool(false), // Add reader.
	})
}

func WithRDSClusterCustomEndpoint(name string) func(*rdstypes.DBCluster) {
	return func(cluster *rdstypes.DBCluster) {
		parsed, _ := arn.Parse(aws.ToString(cluster.DBClusterArn))
		cluster.CustomEndpoints = append(cluster.CustomEndpoints,
			fmt.Sprintf("%v.cluster-custom-aabbccdd.%v.rds.amazonaws.com", name, parsed.Region),
		)
	}
}

// RDSProxy returns a sample rdstypes.DBProxy.
func RDSProxy(name, region, vpcID string) *rdstypes.DBProxy {
	return &rdstypes.DBProxy{
		DBProxyArn:   aws.String(fmt.Sprintf("arn:aws:rds:%s:123456789012:db-proxy:prx-%s", region, name)),
		DBProxyName:  aws.String(name),
		EngineFamily: aws.String(string(rdstypes.EngineFamilyMysql)),
		Endpoint:     aws.String(fmt.Sprintf("%s.proxy-aabbccdd.%s.rds.amazonaws.com", name, region)),
		VpcId:        aws.String(vpcID),
		RequireTLS:   aws.Bool(true),
		Status:       "available",
	}
}

// RDSProxyCustomEndpoint returns a sample rdstypes.DBProxyEndpoint.
func RDSProxyCustomEndpoint(rdsProxy *rdstypes.DBProxy, name, region string) *rdstypes.DBProxyEndpoint {
	return &rdstypes.DBProxyEndpoint{
		Endpoint:            aws.String(fmt.Sprintf("%s.endpoint.proxy-aabbccdd.%s.rds.amazonaws.com", name, region)),
		DBProxyEndpointName: aws.String(name),
		DBProxyName:         rdsProxy.DBProxyName,
		DBProxyEndpointArn:  aws.String(fmt.Sprintf("arn:aws:rds:%v:123456789012:db-proxy-endpoint:prx-endpoint-%v", region, name)),
		TargetRole:          rdstypes.DBProxyEndpointTargetRoleReadOnly,
		Status:              "available",
	}
}

// DocumentDBCluster returns a sample rdstypes.DBCluster for DocumentDB.
func DocumentDBCluster(name, region string, labels map[string]string, opts ...func(*rdstypes.DBCluster)) *rdstypes.DBCluster {
	cluster := &rdstypes.DBCluster{
		DBClusterArn:        aws.String(fmt.Sprintf("arn:aws:rds:%v:123456789012:cluster:%v", region, name)),
		DBClusterIdentifier: aws.String(name),
		DbClusterResourceId: aws.String(uuid.New().String()),
		Engine:              aws.String("docdb"),
		EngineVersion:       aws.String("5.0.0"),
		Status:              aws.String("available"),
		Endpoint:            aws.String(fmt.Sprintf("%v.cluster-aabbccdd.%v.docdb.amazonaws.com", name, region)),
		ReaderEndpoint:      aws.String(fmt.Sprintf("%v.cluster-ro-aabbccdd.%v.docdb.amazonaws.com", name, region)),
		Port:                aws.Int32(27017),
		TagList:             awstesthelpers.LabelsToRDSTags(labels),
		DBClusterMembers: []rdstypes.DBClusterMember{{
			IsClusterWriter: aws.Bool(true), // One writer by default.
		}},
	}
	for _, opt := range opts {
		opt(cluster)
	}
	return cluster
}

func WithDocumentDBClusterReader(cluster *rdstypes.DBCluster) {
	WithRDSClusterReader(cluster)
}
