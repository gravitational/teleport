/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package aws

import (
	"fmt"
	"path"
	"reflect"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/aws/aws-sdk-go/service/redshiftserverless"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ReadableResourceName returns a human readable string of an AWS resource type.
func ReadableResourceName(r interface{}) string {
	switch v := r.(type) {
	case *rds.DBInstance:
		return fmt.Sprintf("RDS instance %q", aws.StringValue(v.DBInstanceIdentifier))
	case *rds.DBCluster:
		return fmt.Sprintf("Aurora cluster %q", aws.StringValue(v.DBClusterIdentifier))
	case *rds.DBProxy:
		return fmt.Sprintf("RDS Proxy %q", aws.StringValue(v.DBProxyName))
	case *rds.DBProxyEndpoint:
		return fmt.Sprintf("RDS Proxy endpoint %q (proxy %q)", aws.StringValue(v.DBProxyEndpointName), aws.StringValue(v.DBProxyName))
	case *memorydb.Cluster:
		return fmt.Sprintf("MemoryDB %q", aws.StringValue(v.Name))
	case *elasticache.ReplicationGroup:
		return fmt.Sprintf("ElastiCache %q", aws.StringValue(v.ReplicationGroupId))
	case *redshift.Cluster:
		return fmt.Sprintf("Redshift cluster %q", aws.StringValue(v.ClusterIdentifier))
	case *redshiftserverless.Workgroup:
		return fmt.Sprintf("Redshift Serverless workgroup %q (namespace %q)", aws.StringValue(v.WorkgroupName), aws.StringValue(v.NamespaceName))
	case *redshiftserverless.EndpointAccess:
		return fmt.Sprintf("Redshift Serverless endpoint %q (workgroup %q)", aws.StringValue(v.EndpointName), aws.StringValue(v.WorkgroupName))

	default:
		value := reflect.Indirect(reflect.ValueOf(r))
		return fmt.Sprintf("%s %q", guessResourceType(value), guessResourceName(value))
	}
}

func guessResourceType(v reflect.Value) string {
	resourceType := v.Type().Name()
	if pkgPath := v.Type().PkgPath(); pkgPath != "" {
		pkgName := cases.Title(language.Und).String(path.Base(pkgPath))
		return fmt.Sprintf("%s %s", pkgName, resourceType)
	}
	return resourceType
}

func guessResourceName(v reflect.Value) string {
	if v.Kind() != reflect.Struct {
		return "<unknown>"
	}

	// Try a few attributes to find the name. For example, if resource type is
	// service.DBCluster, try:
	// - service.DBCluster.Name
	// - service.DBCluster.DBClusterName
	// - service.DBCluster.DBClusterIdentifier
	resourceType := v.Type().Name()
	tryFieldNames := []string{
		"Name",
		resourceType + "Name",
		resourceType + "Identifier",
	}

	for _, tryFieldName := range tryFieldNames {
		field := reflect.Indirect(v.FieldByName(tryFieldName))
		if field.IsValid() && field.Kind() == reflect.String {
			return field.String()
		}
	}
	return "<unknown>"
}
