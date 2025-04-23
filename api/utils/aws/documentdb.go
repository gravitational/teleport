/*
Copyright 2024 Gravitational, Inc.

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
	"net/url"
	"strings"

	"github.com/gravitational/trace"
)

// IsDocumentDBEndpoint returns true if the input URI is a DocumentDB endpoint.
//
// https://docs.aws.amazon.com/documentdb/latest/developerguide/endpoints.html
func IsDocumentDBEndpoint(uri string) bool {
	return isAWSServiceEndpoint(uri, DocumentDBServiceName)
}

// DocumentDBEndpointDetails contains information about a DocumentDB endpoint.
type DocumentDBEndpointDetails struct {
	// ClusterID is the identifier of a DocumentDB cluster.
	ClusterID string
	// InstanceID is the identifier of a DocumentDB instance.
	InstanceID string
	// Region is the AWS region for the endpoint.
	Region string
	// EndpointType specifies the type of the endpoint.
	EndpointType string
}

// ParseDocumentDBEndpoint parses and extracts info from the provided
// DocumentDB endpoint.
func ParseDocumentDBEndpoint(endpoint string) (*DocumentDBEndpointDetails, error) {
	if !strings.HasPrefix(endpoint, "mongodb+srv://") &&
		!strings.HasPrefix(endpoint, "mongodb://") {
		endpoint = "mongodb://" + endpoint
	}

	docdbURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	endpoint = docdbURL.Hostname()

	if strings.HasSuffix(endpoint, AWSCNEndpointSuffix) {
		return parseDocumentDBCNEndpoint(endpoint)
	}
	if strings.HasSuffix(endpoint, AWSEndpointSuffix) {
		return parseDocumentDBEndpoint(endpoint)
	}
	return nil, trace.BadParameter("failed to parse %v as DocumentDB endpoint", endpoint)
}

func parseDocumentDBCNEndpoint(endpoint string) (*DocumentDBEndpointDetails, error) {
	// Example:
	// my-documentdb-cluster-id.cluster-abcdefghijklmnop.docdb.cn-north-1.amazonaws.com.cn
	parts := strings.Split(strings.TrimSuffix(endpoint, AWSCNEndpointSuffix), ".")
	if len(parts) != 4 {
		return nil, trace.BadParameter("failed to parse %v as DocumentDB CN endpoint", endpoint)
	}
	if parts[2] != DocumentDBServiceName {
		return nil, trace.BadParameter("failed to parse %v as DocumentDB CN endpoint", endpoint)
	}
	return makeDocumentDBDetails(parts[0], parts[1], parts[3]), nil
}

func parseDocumentDBEndpoint(endpoint string) (*DocumentDBEndpointDetails, error) {
	// Examples:
	// my-documentdb-cluster-id.cluster-abcdefghijklmnop.us-east-1.docdb.amazonaws.com
	// my-documentdb-cluster-id.cluster-ro-abcdefghijklmnop.us-east-1.docdb.amazonaws.com
	// my-instance-id.abcdefghijklmnop.us-east-1.docdb.amazonaws.com
	parts := strings.Split(strings.TrimSuffix(endpoint, AWSEndpointSuffix), ".")
	if len(parts) != 4 {
		return nil, trace.BadParameter("failed to parse %v as DocumentDB endpoint", endpoint)
	}
	if parts[3] != DocumentDBServiceName {
		return nil, trace.BadParameter("failed to parse %v as DocumentDB endpoint", endpoint)
	}
	return makeDocumentDBDetails(parts[0], parts[1], parts[2]), nil
}

func makeDocumentDBDetails(id, endpointTypePart, region string) *DocumentDBEndpointDetails {
	endpointType := guessDocumentDBEndpointType(endpointTypePart)
	if endpointType == DocumentDBInstanceEndpoint {
		return &DocumentDBEndpointDetails{
			InstanceID:   id,
			Region:       region,
			EndpointType: endpointType,
		}
	}

	return &DocumentDBEndpointDetails{
		ClusterID:    id,
		Region:       region,
		EndpointType: endpointType,
	}
}

func guessDocumentDBEndpointType(endpointTypePart string) string {
	switch {
	case strings.HasPrefix(endpointTypePart, "cluster-ro-"):
		return DocumentDBClusterReaderEndpoint
	case strings.HasPrefix(endpointTypePart, "cluster-"):
		return DocumentDBClusterEndpoint
	default:
		return DocumentDBInstanceEndpoint
	}
}

const (
	// DocumentDBServiceName is the service name for AWS DocumentDB.
	//
	// TODO(greedy52) support DocumentDB Elastic clusters when IAM Auth support
	// is added. Note that Elastic clusters use "docdb-elastic" as the service
	// name in the endpoint.
	DocumentDBServiceName = "docdb"

	// DocumentDBClusterEndpoint specifies a DocumentDB primary/cluster
	// endpoint.
	DocumentDBClusterEndpoint = "cluster"
	// DocumentDBReaderEndpoint specifies a DocumentDB reader endpoint.
	DocumentDBClusterReaderEndpoint = "reader"
	// DocumentDBInstanceEndpoint specifies a DocumentDB instance endpoint.
	DocumentDBInstanceEndpoint = "instance"
)
