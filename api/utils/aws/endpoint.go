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

// trimAWSParentDomain removes common AWS endpoint suffixes from the endpoint.
func trimAWSParentDomain(endpoint string) (string, error) {
	if strings.HasSuffix(endpoint, AWSEndpointSuffix) {
		return strings.TrimSuffix(endpoint, AWSEndpointSuffix), nil
	}

	if strings.HasSuffix(endpoint, AWSCNEndpointSuffix) {
		return strings.TrimSuffix(endpoint, AWSCNEndpointSuffix), nil
	}

	return "", trace.BadParameter("endpoint %v is not an AWS endpoint", endpoint)
}

// ParseRDSURI extracts the identifier and region from the provided RDS URI.
func ParseRDSURI(uri string) (id, region string, err error) {
	endpoint, _, err := net.SplitHostPort(uri)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	return ParseRDSEndpoint(endpoint)
}

// ParseRDSEndpoint extracts the identifier and region from the provided RDS
// endpoint.
//
// RDS/Aurora endpoints look like this:
// aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com
// aurora-instance-2.abcdefghijklmnop.rds.cn-north-1.amazonaws.com.cn
func ParseRDSEndpoint(endpoint string) (id, region string, err error) {
	trimmedEndpoint, err := trimAWSParentDomain(endpoint)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	parts := strings.Split(trimmedEndpoint, ".")
	if len(parts) != 4 {
		return "", "", trace.BadParameter("failed to parse %v as RDS endpoint", endpoint)
	}

	// Service name/region can be at either position 2 or 3.
	switch {
	case parts[3] == RDSServiceName:
		region = parts[2]

	case parts[2] == RDSServiceName:
		region = parts[3]

	default:
		return "", "", trace.BadParameter("failed to parse %v as RDS endpoint", endpoint)
	}

	return parts[0], region, nil
}

// ParseRedshiftURI extracts cluster ID and region from the provided Redshift
// URI.
//
// Redshift endpoints look like this:
// redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com
// redshift-cluster-2.abcdefghijklmnop.redshift.cn-north-2.amazonaws.com.cn
func ParseRedshiftURI(uri string) (clusterID, region string, err error) {
	endpoint, _, err := net.SplitHostPort(uri)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	trimmedEndpoint, err := trimAWSParentDomain(endpoint)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	parts := strings.Split(trimmedEndpoint, ".")
	if len(parts) != 4 {
		return "", "", trace.BadParameter("failed to parse %v as Redshift endpoint", endpoint)
	}

	// Service name/region can be at either position 2 or 3.
	switch {
	case parts[3] == RedshiftServiceName:
		region = parts[2]

	case parts[2] == RedshiftServiceName:
		region = parts[3]

	default:
		return "", "", trace.BadParameter("failed to parse %v as Redshift endpoint", endpoint)
	}

	return parts[0], region, nil
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
