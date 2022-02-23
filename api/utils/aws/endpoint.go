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

// IsRDSEndpoint returns true if input is an RDS endpoint
//
// https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/Aurora.Overview.Endpoints.html
func IsRDSEndpoint(uri string) bool {
	return strings.Contains(uri, AWSEndpointSuffix) && strings.Contains(uri, RDSEndpointSubdomain)
}

// IsRedshiftEndpoint returns true if input is an RDS endpoint
//
// https://docs.aws.amazon.com/redshift/latest/mgmt/connecting-from-psql.html
func IsRedshiftEndpoint(uri string) bool {
	return strings.Contains(uri, AWSEndpointSuffix) && strings.Contains(uri, RedshiftEndpointSubdomain)
}

// TrimAWSEndpointSuffixes removes AWS endpoint suffixes from the endpoint.
func TrimAWSEndpointSuffixes(endpoint string) (string, bool) {
	switch {
	case strings.HasSuffix(endpoint, AWSEndpointSuffix):
		return endpoint[:len(endpoint)-len(AWSEndpointSuffix)], true

	case strings.HasSuffix(endpoint, AWSCNEndpointSuffix):
		return endpoint[:len(endpoint)-len(AWSCNEndpointSuffix)], true
	}
	return endpoint, false
}

// ParseRDSEndpoint extracts IDs and region from the provided RDS endpoint.
func ParseRDSEndpoint(endpoint string) (instanceID, region string, err error) {
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	// RDS/Aurora endpoints look like this:
	// aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com
	// aurora-instance-2.abcdefghijklmnop.rds.cn-north-1.amazonaws.com.cn
	trimmedHost, hasSuffix := TrimAWSEndpointSuffixes(host)
	if !hasSuffix {
		return "", "", trace.BadParameter("endpoint %v is not an AWS endpoint", endpoint)
	}

	parts := strings.Split(trimmedHost, ".")
	if len(parts) != 4 {
		return "", "", trace.BadParameter("failed to parse %v as RDS endpoint", endpoint)
	}

	// Region and service name at either position 2 or 3.
	if parts[3] == RDSServiceName {
		return parts[0], parts[2], nil
	} else if parts[2] == RDSServiceName {
		return parts[0], parts[3], nil
	} else {
		return "", "", trace.BadParameter("endpoint %v is not an RDS endpoint", endpoint)
	}
}

// ParseRedshiftEndpoint extracts cluster ID and region from the provided Redshift endpoint.
func ParseRedshiftEndpoint(endpoint string) (clusterID, region string, err error) {
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	// Redshift endpoints look like this:
	// redshift-cluster-1.abcdefghijklmnop.us-east-1.rds.amazonaws.com
	// redshift-cluster-2.abcdefghijklmnop.rds.cn-north-1.amazonaws.com.cn
	trimmedHost, hasSuffix := TrimAWSEndpointSuffixes(host)
	if !hasSuffix {
		return "", "", trace.BadParameter("endpoint %v is not an AWS endpoint", endpoint)
	}

	parts := strings.Split(trimmedHost, ".")
	if len(parts) != 4 {
		return "", "", trace.BadParameter("failed to parse %v as Redshift endpoint", endpoint)
	}

	// Region and service name at either position 2 or 3.
	if parts[3] == RedshiftServiceName {
		return parts[0], parts[2], nil
	} else if parts[2] == RedshiftServiceName {
		return parts[0], parts[3], nil
	} else {
		return "", "", trace.BadParameter("endpoint %v is not an Redshift endpoint", endpoint)
	}
}

const (
	// AWSEndpointSuffix is the endpoint suffix for the AWS Standard regions.
	AWSEndpointSuffix = ".amazonaws.com"

	// AWSCNEndpointSuffix is the endpoint suffix for the AWS China regions.
	AWSCNEndpointSuffix = ".amazonaws.com.cn"

	// RDSServiceName is the service name for AWS RDS.
	RDSServiceName = "rds"

	// RedshiftServiceName is the service name for AWS Redshift.
	RedshiftServiceName = "redshift"

	// RDSEndpointSubdomain is the RDS/Aurora subdomain.
	RDSEndpointSubdomain = "." + RDSServiceName + "."

	// RedshiftEndpointSubdomain is the Redshift endpoint suffix.
	RedshiftEndpointSubdomain = "." + RedshiftServiceName + "."
)
