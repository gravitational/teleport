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
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
)

// RDSMock mocks AWS RDS API.
type RDSMock struct {
	rdsiface.RDSAPI
	DBInstances      []*rds.DBInstance
	DBClusters       []*rds.DBCluster
	DBProxies        []*rds.DBProxy
	DBProxyEndpoints []*rds.DBProxyEndpoint
	DBEngineVersions []*rds.DBEngineVersion
}

func (m *RDSMock) DescribeDBInstancesWithContext(ctx aws.Context, input *rds.DescribeDBInstancesInput, options ...request.Option) (*rds.DescribeDBInstancesOutput, error) {
	if err := checkEngineFilters(input.Filters, m.DBEngineVersions); err != nil {
		return nil, trace.Wrap(err)
	}
	instances, err := applyInstanceFilters(m.DBInstances, input.Filters)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if aws.StringValue(input.DBInstanceIdentifier) == "" {
		return &rds.DescribeDBInstancesOutput{
			DBInstances: instances,
		}, nil
	}
	for _, instance := range instances {
		if aws.StringValue(instance.DBInstanceIdentifier) == aws.StringValue(input.DBInstanceIdentifier) {
			return &rds.DescribeDBInstancesOutput{
				DBInstances: []*rds.DBInstance{instance},
			}, nil
		}
	}
	return nil, trace.NotFound("instance %v not found", aws.StringValue(input.DBInstanceIdentifier))
}

func (m *RDSMock) DescribeDBInstancesPagesWithContext(ctx aws.Context, input *rds.DescribeDBInstancesInput, fn func(*rds.DescribeDBInstancesOutput, bool) bool, options ...request.Option) error {
	if err := checkEngineFilters(input.Filters, m.DBEngineVersions); err != nil {
		return trace.Wrap(err)
	}
	instances, err := applyInstanceFilters(m.DBInstances, input.Filters)
	if err != nil {
		return trace.Wrap(err)
	}
	fn(&rds.DescribeDBInstancesOutput{
		DBInstances: instances,
	}, true)
	return nil
}

func (m *RDSMock) DescribeDBClustersWithContext(ctx aws.Context, input *rds.DescribeDBClustersInput, options ...request.Option) (*rds.DescribeDBClustersOutput, error) {
	if err := checkEngineFilters(input.Filters, m.DBEngineVersions); err != nil {
		return nil, trace.Wrap(err)
	}
	clusters, err := applyClusterFilters(m.DBClusters, input.Filters)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if aws.StringValue(input.DBClusterIdentifier) == "" {
		return &rds.DescribeDBClustersOutput{
			DBClusters: clusters,
		}, nil
	}
	for _, cluster := range clusters {
		if aws.StringValue(cluster.DBClusterIdentifier) == aws.StringValue(input.DBClusterIdentifier) {
			return &rds.DescribeDBClustersOutput{
				DBClusters: []*rds.DBCluster{cluster},
			}, nil
		}
	}
	return nil, trace.NotFound("cluster %v not found", aws.StringValue(input.DBClusterIdentifier))
}

func (m *RDSMock) DescribeDBClustersPagesWithContext(aws aws.Context, input *rds.DescribeDBClustersInput, fn func(*rds.DescribeDBClustersOutput, bool) bool, options ...request.Option) error {
	if err := checkEngineFilters(input.Filters, m.DBEngineVersions); err != nil {
		return trace.Wrap(err)
	}
	clusters, err := applyClusterFilters(m.DBClusters, input.Filters)
	if err != nil {
		return trace.Wrap(err)
	}
	fn(&rds.DescribeDBClustersOutput{
		DBClusters: clusters,
	}, true)
	return nil
}

func (m *RDSMock) ModifyDBInstanceWithContext(ctx aws.Context, input *rds.ModifyDBInstanceInput, options ...request.Option) (*rds.ModifyDBInstanceOutput, error) {
	for i, instance := range m.DBInstances {
		if aws.StringValue(instance.DBInstanceIdentifier) == aws.StringValue(input.DBInstanceIdentifier) {
			if aws.BoolValue(input.EnableIAMDatabaseAuthentication) {
				m.DBInstances[i].IAMDatabaseAuthenticationEnabled = aws.Bool(true)
			}
			return &rds.ModifyDBInstanceOutput{
				DBInstance: m.DBInstances[i],
			}, nil
		}
	}
	return nil, trace.NotFound("instance %v not found", aws.StringValue(input.DBInstanceIdentifier))
}

func (m *RDSMock) ModifyDBClusterWithContext(ctx aws.Context, input *rds.ModifyDBClusterInput, options ...request.Option) (*rds.ModifyDBClusterOutput, error) {
	for i, cluster := range m.DBClusters {
		if aws.StringValue(cluster.DBClusterIdentifier) == aws.StringValue(input.DBClusterIdentifier) {
			if aws.BoolValue(input.EnableIAMDatabaseAuthentication) {
				m.DBClusters[i].IAMDatabaseAuthenticationEnabled = aws.Bool(true)
			}
			return &rds.ModifyDBClusterOutput{
				DBCluster: m.DBClusters[i],
			}, nil
		}
	}
	return nil, trace.NotFound("cluster %v not found", aws.StringValue(input.DBClusterIdentifier))
}

func (m *RDSMock) DescribeDBProxiesWithContext(ctx aws.Context, input *rds.DescribeDBProxiesInput, options ...request.Option) (*rds.DescribeDBProxiesOutput, error) {
	if aws.StringValue(input.DBProxyName) == "" {
		return &rds.DescribeDBProxiesOutput{
			DBProxies: m.DBProxies,
		}, nil
	}
	for _, dbProxy := range m.DBProxies {
		if aws.StringValue(dbProxy.DBProxyName) == aws.StringValue(input.DBProxyName) {
			return &rds.DescribeDBProxiesOutput{
				DBProxies: []*rds.DBProxy{dbProxy},
			}, nil
		}
	}
	return nil, trace.NotFound("proxy %v not found", aws.StringValue(input.DBProxyName))
}

func (m *RDSMock) DescribeDBProxyEndpointsWithContext(ctx aws.Context, input *rds.DescribeDBProxyEndpointsInput, options ...request.Option) (*rds.DescribeDBProxyEndpointsOutput, error) {
	inputProxyName := aws.StringValue(input.DBProxyName)
	inputProxyEndpointName := aws.StringValue(input.DBProxyEndpointName)

	if inputProxyName == "" && inputProxyEndpointName == "" {
		return &rds.DescribeDBProxyEndpointsOutput{
			DBProxyEndpoints: m.DBProxyEndpoints,
		}, nil
	}

	var endpoints []*rds.DBProxyEndpoint
	for _, dbProxyEndpoiont := range m.DBProxyEndpoints {
		if inputProxyEndpointName != "" &&
			inputProxyEndpointName != aws.StringValue(dbProxyEndpoiont.DBProxyEndpointName) {
			continue
		}

		if inputProxyName != "" &&
			inputProxyName != aws.StringValue(dbProxyEndpoiont.DBProxyName) {
			continue
		}

		endpoints = append(endpoints, dbProxyEndpoiont)
	}
	if len(endpoints) == 0 {
		return nil, trace.NotFound("proxy endpoint %v not found", aws.StringValue(input.DBProxyEndpointName))
	}
	return &rds.DescribeDBProxyEndpointsOutput{DBProxyEndpoints: endpoints}, nil
}

func (m *RDSMock) DescribeDBProxiesPagesWithContext(ctx aws.Context, input *rds.DescribeDBProxiesInput, fn func(*rds.DescribeDBProxiesOutput, bool) bool, options ...request.Option) error {
	fn(&rds.DescribeDBProxiesOutput{
		DBProxies: m.DBProxies,
	}, true)
	return nil
}

func (m *RDSMock) DescribeDBProxyEndpointsPagesWithContext(ctx aws.Context, input *rds.DescribeDBProxyEndpointsInput, fn func(*rds.DescribeDBProxyEndpointsOutput, bool) bool, options ...request.Option) error {
	fn(&rds.DescribeDBProxyEndpointsOutput{
		DBProxyEndpoints: m.DBProxyEndpoints,
	}, true)
	return nil
}

func (m *RDSMock) ListTagsForResourceWithContext(ctx aws.Context, input *rds.ListTagsForResourceInput, options ...request.Option) (*rds.ListTagsForResourceOutput, error) {
	return &rds.ListTagsForResourceOutput{}, nil
}

// RDSMockUnauth is a mock RDS client that returns access denied to each call.
type RDSMockUnauth struct {
	rdsiface.RDSAPI
}

func (m *RDSMockUnauth) DescribeDBInstancesWithContext(ctx aws.Context, input *rds.DescribeDBInstancesInput, options ...request.Option) (*rds.DescribeDBInstancesOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

func (m *RDSMockUnauth) DescribeDBClustersWithContext(ctx aws.Context, input *rds.DescribeDBClustersInput, options ...request.Option) (*rds.DescribeDBClustersOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

func (m *RDSMockUnauth) ModifyDBInstanceWithContext(ctx aws.Context, input *rds.ModifyDBInstanceInput, options ...request.Option) (*rds.ModifyDBInstanceOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

func (m *RDSMockUnauth) ModifyDBClusterWithContext(ctx aws.Context, input *rds.ModifyDBClusterInput, options ...request.Option) (*rds.ModifyDBClusterOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

func (m *RDSMockUnauth) DescribeDBInstancesPagesWithContext(ctx aws.Context, input *rds.DescribeDBInstancesInput, fn func(*rds.DescribeDBInstancesOutput, bool) bool, options ...request.Option) error {
	return trace.AccessDenied("unauthorized")
}

func (m *RDSMockUnauth) DescribeDBClustersPagesWithContext(aws aws.Context, input *rds.DescribeDBClustersInput, fn func(*rds.DescribeDBClustersOutput, bool) bool, options ...request.Option) error {
	return trace.AccessDenied("unauthorized")
}

func (m *RDSMockUnauth) DescribeDBProxiesWithContext(ctx aws.Context, input *rds.DescribeDBProxiesInput, options ...request.Option) (*rds.DescribeDBProxiesOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

func (m *RDSMockUnauth) DescribeDBProxyEndpointsWithContext(ctx aws.Context, input *rds.DescribeDBProxyEndpointsInput, options ...request.Option) (*rds.DescribeDBProxyEndpointsOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

func (m *RDSMockUnauth) DescribeDBProxiesPagesWithContext(ctx aws.Context, input *rds.DescribeDBProxiesInput, fn func(*rds.DescribeDBProxiesOutput, bool) bool, options ...request.Option) error {
	return trace.AccessDenied("unauthorized")
}

// RDSMockByDBType is a mock RDS client that mocks API calls by DB type
type RDSMockByDBType struct {
	rdsiface.RDSAPI
	DBInstances rdsiface.RDSAPI
	DBClusters  rdsiface.RDSAPI
	DBProxies   rdsiface.RDSAPI
}

func (m *RDSMockByDBType) DescribeDBInstancesWithContext(ctx aws.Context, input *rds.DescribeDBInstancesInput, options ...request.Option) (*rds.DescribeDBInstancesOutput, error) {
	return m.DBInstances.DescribeDBInstancesWithContext(ctx, input, options...)
}

func (m *RDSMockByDBType) ModifyDBInstanceWithContext(ctx aws.Context, input *rds.ModifyDBInstanceInput, options ...request.Option) (*rds.ModifyDBInstanceOutput, error) {
	return m.DBInstances.ModifyDBInstanceWithContext(ctx, input, options...)
}

func (m *RDSMockByDBType) DescribeDBInstancesPagesWithContext(ctx aws.Context, input *rds.DescribeDBInstancesInput, fn func(*rds.DescribeDBInstancesOutput, bool) bool, options ...request.Option) error {
	return m.DBInstances.DescribeDBInstancesPagesWithContext(ctx, input, fn, options...)
}

func (m *RDSMockByDBType) DescribeDBClustersWithContext(ctx aws.Context, input *rds.DescribeDBClustersInput, options ...request.Option) (*rds.DescribeDBClustersOutput, error) {
	return m.DBClusters.DescribeDBClustersWithContext(ctx, input, options...)
}

func (m *RDSMockByDBType) ModifyDBClusterWithContext(ctx aws.Context, input *rds.ModifyDBClusterInput, options ...request.Option) (*rds.ModifyDBClusterOutput, error) {
	return m.DBClusters.ModifyDBClusterWithContext(ctx, input, options...)
}

func (m *RDSMockByDBType) DescribeDBClustersPagesWithContext(aws aws.Context, input *rds.DescribeDBClustersInput, fn func(*rds.DescribeDBClustersOutput, bool) bool, options ...request.Option) error {
	return m.DBClusters.DescribeDBClustersPagesWithContext(aws, input, fn, options...)
}

func (m *RDSMockByDBType) DescribeDBProxiesWithContext(ctx aws.Context, input *rds.DescribeDBProxiesInput, options ...request.Option) (*rds.DescribeDBProxiesOutput, error) {
	return m.DBProxies.DescribeDBProxiesWithContext(ctx, input, options...)
}

func (m *RDSMockByDBType) DescribeDBProxyEndpointsWithContext(ctx aws.Context, input *rds.DescribeDBProxyEndpointsInput, options ...request.Option) (*rds.DescribeDBProxyEndpointsOutput, error) {
	return m.DBProxies.DescribeDBProxyEndpointsWithContext(ctx, input, options...)
}

func (m *RDSMockByDBType) DescribeDBProxiesPagesWithContext(ctx aws.Context, input *rds.DescribeDBProxiesInput, fn func(*rds.DescribeDBProxiesOutput, bool) bool, options ...request.Option) error {
	return m.DBProxies.DescribeDBProxiesPagesWithContext(ctx, input, fn, options...)
}

// checkEngineFilters checks RDS filters to detect unrecognized engine filters.
func checkEngineFilters(filters []*rds.Filter, engineVersions []*rds.DBEngineVersion) error {
	if len(filters) == 0 {
		return nil
	}
	recognizedEngines := make(map[string]struct{})
	for _, e := range engineVersions {
		recognizedEngines[aws.StringValue(e.Engine)] = struct{}{}
	}
	for _, f := range filters {
		if aws.StringValue(f.Name) != "engine" {
			continue
		}
		for _, v := range f.Values {
			if _, ok := recognizedEngines[aws.StringValue(v)]; !ok {
				return trace.Errorf("unrecognized engine name %q", aws.StringValue(v))
			}
		}
	}
	return nil
}

// applyInstanceFilters filters RDS DBInstances using the provided RDS filters.
func applyInstanceFilters(in []*rds.DBInstance, filters []*rds.Filter) ([]*rds.DBInstance, error) {
	if len(filters) == 0 {
		return in, nil
	}
	var out []*rds.DBInstance
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
func applyClusterFilters(in []*rds.DBCluster, filters []*rds.Filter) ([]*rds.DBCluster, error) {
	if len(filters) == 0 {
		return in, nil
	}
	var out []*rds.DBCluster
	efs := engineFilterSet(filters)
	for _, cluster := range in {
		if clusterEngineMatches(cluster, efs) {
			out = append(out, cluster)
		}
	}
	return out, nil
}

// engineFilterSet builds a string set of engine names from a list of RDS filters.
func engineFilterSet(filters []*rds.Filter) map[string]struct{} {
	return filterValues(filters, "engine")
}

// clusterIdentifierFilterSet builds a string set of ClusterIDs from a list of RDS filters.
func clusterIdentifierFilterSet(filters []*rds.Filter) map[string]struct{} {
	return filterValues(filters, "db-cluster-id")
}

func filterValues(filters []*rds.Filter, filterKey string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, f := range filters {
		if aws.StringValue(f.Name) != filterKey {
			continue
		}
		for _, v := range f.Values {
			out[aws.StringValue(v)] = struct{}{}
		}
	}
	return out
}

// instanceEngineMatches returns whether an RDS DBInstance engine matches any engine name in a filter set.
func instanceEngineMatches(instance *rds.DBInstance, filterSet map[string]struct{}) bool {
	_, ok := filterSet[aws.StringValue(instance.Engine)]
	return ok
}

// instanceClusterIDMatches returns whether an RDS DBInstance ClusterID matches any ClusterID in a filter set.
func instanceClusterIDMatches(instance *rds.DBInstance, filterSet map[string]struct{}) bool {
	_, ok := filterSet[aws.StringValue(instance.DBClusterIdentifier)]
	return ok
}

// clusterEngineMatches returns whether an RDS DBCluster engine matches any engine name in a filter set.
func clusterEngineMatches(cluster *rds.DBCluster, filterSet map[string]struct{}) bool {
	_, ok := filterSet[aws.StringValue(cluster.Engine)]
	return ok
}

// RDSInstance returns a sample rds.DBInstance.
func RDSInstance(name, region string, labels map[string]string, opts ...func(*rds.DBInstance)) *rds.DBInstance {
	instance := &rds.DBInstance{
		DBInstanceArn:        aws.String(fmt.Sprintf("arn:aws:rds:%v:123456789012:db:%v", region, name)),
		DBInstanceIdentifier: aws.String(name),
		DbiResourceId:        aws.String(uuid.New().String()),
		Engine:               aws.String("postgres"),
		DBInstanceStatus:     aws.String("available"),
		Endpoint: &rds.Endpoint{
			Address: aws.String(fmt.Sprintf("%v.aabbccdd.%v.rds.amazonaws.com", name, region)),
			Port:    aws.Int64(5432),
		},
		TagList: libcloudaws.LabelsToTags[rds.Tag](labels),
	}
	for _, opt := range opts {
		opt(instance)
	}
	return instance
}

// RDSCluster returns a sample rds.DBCluster.
func RDSCluster(name, region string, labels map[string]string, opts ...func(*rds.DBCluster)) *rds.DBCluster {
	cluster := &rds.DBCluster{
		DBClusterArn:        aws.String(fmt.Sprintf("arn:aws:rds:%v:123456789012:cluster:%v", region, name)),
		DBClusterIdentifier: aws.String(name),
		DbClusterResourceId: aws.String(uuid.New().String()),
		Engine:              aws.String("aurora-mysql"),
		EngineMode:          aws.String("provisioned"),
		Status:              aws.String("available"),
		Endpoint:            aws.String(fmt.Sprintf("%v.cluster-aabbccdd.%v.rds.amazonaws.com", name, region)),
		ReaderEndpoint:      aws.String(fmt.Sprintf("%v.cluster-ro-aabbccdd.%v.rds.amazonaws.com", name, region)),
		Port:                aws.Int64(3306),
		TagList:             libcloudaws.LabelsToTags[rds.Tag](labels),
		DBClusterMembers: []*rds.DBClusterMember{{
			IsClusterWriter: aws.Bool(true), // One writer by default.
		}},
	}
	for _, opt := range opts {
		opt(cluster)
	}
	return cluster
}

func WithRDSClusterReader(cluster *rds.DBCluster) {
	cluster.DBClusterMembers = append(cluster.DBClusterMembers, &rds.DBClusterMember{
		IsClusterWriter: aws.Bool(false), // Add reader.
	})
}

func WithRDSClusterCustomEndpoint(name string) func(*rds.DBCluster) {
	return func(cluster *rds.DBCluster) {
		parsed, _ := arn.Parse(aws.StringValue(cluster.DBClusterArn))
		cluster.CustomEndpoints = append(cluster.CustomEndpoints, aws.String(
			fmt.Sprintf("%v.cluster-custom-aabbccdd.%v.rds.amazonaws.com", name, parsed.Region),
		))
	}
}

// RDSProxy returns a sample rds.DBProxy.
func RDSProxy(name, region, vpcID string) *rds.DBProxy {
	return &rds.DBProxy{
		DBProxyArn:   aws.String(fmt.Sprintf("arn:aws:rds:%s:123456789012:db-proxy:prx-%s", region, name)),
		DBProxyName:  aws.String(name),
		EngineFamily: aws.String(rds.EngineFamilyMysql),
		Endpoint:     aws.String(fmt.Sprintf("%s.proxy-aabbccdd.%s.rds.amazonaws.com", name, region)),
		VpcId:        aws.String(vpcID),
		RequireTLS:   aws.Bool(true),
		Status:       aws.String("available"),
	}
}

// RDSProxyCustomEndpoint returns a sample rds.DBProxyEndpoint.
func RDSProxyCustomEndpoint(rdsProxy *rds.DBProxy, name, region string) *rds.DBProxyEndpoint {
	return &rds.DBProxyEndpoint{
		Endpoint:            aws.String(fmt.Sprintf("%s.endpoint.proxy-aabbccdd.%s.rds.amazonaws.com", name, region)),
		DBProxyEndpointName: aws.String(name),
		DBProxyName:         rdsProxy.DBProxyName,
		DBProxyEndpointArn:  aws.String(fmt.Sprintf("arn:aws:rds:%v:123456789012:db-proxy-endpoint:prx-endpoint-%v", region, name)),
		TargetRole:          aws.String(rds.DBProxyEndpointTargetRoleReadOnly),
		Status:              aws.String("available"),
	}
}

// DocumentDBCluster returns a sample rds.DBCluster for DocumentDB.
func DocumentDBCluster(name, region string, labels map[string]string, opts ...func(*rds.DBCluster)) *rds.DBCluster {
	cluster := &rds.DBCluster{
		DBClusterArn:        aws.String(fmt.Sprintf("arn:aws:rds:%v:123456789012:cluster:%v", region, name)),
		DBClusterIdentifier: aws.String(name),
		DbClusterResourceId: aws.String(uuid.New().String()),
		Engine:              aws.String("docdb"),
		EngineVersion:       aws.String("5.0.0"),
		Status:              aws.String("available"),
		Endpoint:            aws.String(fmt.Sprintf("%v.cluster-aabbccdd.%v.docdb.amazonaws.com", name, region)),
		ReaderEndpoint:      aws.String(fmt.Sprintf("%v.cluster-ro-aabbccdd.%v.docdb.amazonaws.com", name, region)),
		Port:                aws.Int64(27017),
		TagList:             libcloudaws.LabelsToTags[rds.Tag](labels),
		DBClusterMembers: []*rds.DBClusterMember{{
			IsClusterWriter: aws.Bool(true), // One writer by default.
		}},
	}
	for _, opt := range opts {
		opt(cluster)
	}
	return cluster
}

func WithDocumentDBClusterReader(cluster *rds.DBCluster) {
	WithRDSClusterReader(cluster)
}
