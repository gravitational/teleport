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
	"net"
	"strings"

	"github.com/gravitational/trace"
)

// IsAWSEndpoint returns true if the input URI is an AWS endpoint.
func IsAWSEndpoint(uri string) bool {
	// Note that AWSCNEndpointSuffix contains AWSEndpointSuffix so there is no
	// need to search for AWSCNEndpointSuffix explicitly.
	return strings.Contains(uri, AWSEndpointSuffix)
}

// IsRDSEndpoint returns true if the input URI is an RDS endpoint.
//
// https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/Aurora.Overview.Endpoints.html
func IsRDSEndpoint(uri string) bool {
	return strings.Contains(uri, RDSEndpointSubdomain) &&
		IsAWSEndpoint(uri)
}

// IsRedshiftEndpoint returns true if the input URI is an Redshift endpoint.
//
// https://docs.aws.amazon.com/redshift/latest/mgmt/connecting-from-psql.html
func IsRedshiftEndpoint(uri string) bool {
	return strings.Contains(uri, RedshiftEndpointSubdomain) &&
		IsAWSEndpoint(uri)
}

// ParseRDSEndpoint extracts the identifier and region from the provided RDS
// endpoint.
func ParseRDSEndpoint(endpoint string) (id, region string, err error) {
	if strings.ContainsRune(endpoint, ':') {
		endpoint, _, err = net.SplitHostPort(endpoint)
		if err != nil {
			return "", "", trace.Wrap(err)
		}
	}

	if strings.HasSuffix(endpoint, AWSCNEndpointSuffix) {
		return parseRDSCNEndpoint(endpoint)
	}
	return parseRDSEndpoint(endpoint)
}

// parseRDSEndpoint extracts the identifier and region from the provided RDS
// endpoint for standard regions.
//
// RDS/Aurora endpoints look like this:
// aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com
func parseRDSEndpoint(endpoint string) (id, region string, err error) {
	parts := strings.Split(endpoint, ".")
	if !strings.HasSuffix(endpoint, AWSEndpointSuffix) || len(parts) != 6 || parts[3] != RDSServiceName {
		return "", "", trace.BadParameter("failed to parse %v as RDS endpoint", endpoint)
	}
	return parts[0], parts[2], nil
}

// parseRDSEndpoint extracts the identifier and region from the provided RDS
// endpoint for AWS China regions.
//
// RDS/Aurora endpoints look like this for AWS China regions:
// aurora-instance-2.abcdefghijklmnop.rds.cn-north-1.amazonaws.com.cn
func parseRDSCNEndpoint(endpoint string) (id, region string, err error) {
	parts := strings.Split(endpoint, ".")
	if !strings.HasSuffix(endpoint, AWSCNEndpointSuffix) || len(parts) != 7 || parts[2] != RDSServiceName {
		return "", "", trace.BadParameter("failed to parse %v as RDS endpoint", endpoint)
	}
	return parts[0], parts[3], nil
}

// ParseRedshiftEndpoint extracts cluster ID and region from the provided
// Redshift endpoint.
func ParseRedshiftEndpoint(endpoint string) (clusterID, region string, err error) {
	if strings.ContainsRune(endpoint, ':') {
		endpoint, _, err = net.SplitHostPort(endpoint)
		if err != nil {
			return "", "", trace.Wrap(err)
		}
	}

	if strings.HasSuffix(endpoint, AWSCNEndpointSuffix) {
		return parseRedshiftCNEndpoint(endpoint)
	}
	return parseRedshiftEndpoint(endpoint)
}

// ParseRedshiftEndpoint extracts cluster ID and region from the provided
// Redshift endpoint for standard regions.
//
// Redshift endpoints look like this:
// redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com
func parseRedshiftEndpoint(endpoint string) (clusterID, region string, err error) {
	parts := strings.Split(endpoint, ".")
	if !strings.HasSuffix(endpoint, AWSEndpointSuffix) || len(parts) != 6 || parts[3] != RedshiftServiceName {
		return "", "", trace.BadParameter("failed to parse %v as Redshift endpoint", endpoint)
	}
	return parts[0], parts[2], nil
}

// ParseRedshiftEndpoint extracts cluster ID and region from the provided
// Redshift endpoint for AWS China regions.
//
// Redshift endpoints look like this for AWS China regions:
// redshift-cluster-2.abcdefghijklmnop.redshift.cn-north-2.amazonaws.com.cn
func parseRedshiftCNEndpoint(endpoint string) (clusterID, region string, err error) {
	parts := strings.Split(endpoint, ".")
	if !strings.HasSuffix(endpoint, AWSCNEndpointSuffix) || len(parts) != 7 || parts[2] != RedshiftServiceName {
		return "", "", trace.BadParameter("failed to parse %v as Redshift endpoint", endpoint)
	}
	return parts[0], parts[3], nil
}

const (
	// AWSEndpointSuffix is the endpoint suffix for AWS Standard and AWS US
	// GovCloud regions.
	//
	// https://docs.aws.amazon.com/general/latest/gr/rande.html#regional-endpoints
	// https://docs.aws.amazon.com/govcloud-us/latest/UserGuide/using-govcloud-endpoints.html
	AWSEndpointSuffix = ".amazonaws.com"

	// AWSCNEndpointSuffix is the endpoint suffix for AWS China regions.
	//
	// https://docs.amazonaws.cn/en_us/aws/latest/userguide/endpoints-arns.html
	AWSCNEndpointSuffix = ".amazonaws.com.cn"

	// RDSServiceName is the service name for AWS RDS.
	RDSServiceName = "rds"

	// RedshiftServiceName is the service name for AWS Redshift.
	RedshiftServiceName = "redshift"

	// RDSEndpointSubdomain is the RDS/Aurora subdomain.
	RDSEndpointSubdomain = "." + RDSServiceName + "."

	// RedshiftEndpointSubdomain is the Redshift endpoint subdomain.
	RedshiftEndpointSubdomain = "." + RedshiftServiceName + "."
)
