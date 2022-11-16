/*
copyright 2022 gravitational, inc.

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

package services

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/redshiftserverless"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
)

type hasStatus interface {
	GetStatus() *string
}

func isCommonAWSDatabaseAvailable(r hasStatus) bool {
	switch strings.ToLower(aws.StringValue(r.GetStatus())) {
	case "available", "modifying", "snapshotting":
		return true

	case "creating", "deleting", "create-failed":
		return false

	default:
		log.Warnf("Unknown status type: %q. Assuming %q is available.", aws.StringValue(r.GetStatus()), r)
		return true
	}
}

// RedshiftServerlessWorkgroup is a type alias of redshiftserverless.Workgroup.
type RedshiftServerlessWorkgroup redshiftserverless.Workgroup

// NewRedshiftServerlessWorkgroup create a new RedshiftServerlessWorkgroup.
func NewRedshiftServerlessWorkgroup(workgroup *redshiftserverless.Workgroup) *RedshiftServerlessWorkgroup {
	return (*RedshiftServerlessWorkgroup)(workgroup)
}

func (r RedshiftServerlessWorkgroup) IsSupported() bool {
	return true // always supported
}
func (r RedshiftServerlessWorkgroup) IsAvailable() bool {
	return isCommonAWSDatabaseAvailable(r)
}
func (r RedshiftServerlessWorkgroup) GetStatus() *string {
	return r.Status
}
func (r RedshiftServerlessWorkgroup) GetARN() *string {
	return r.WorkgroupArn
}
func (r RedshiftServerlessWorkgroup) String() string {
	return fmt.Sprintf("Redshift Serverless Workgroup %v (Namespace %v)", aws.StringValue(r.WorkgroupName), aws.StringValue(r.NamespaceName))
}
func (r RedshiftServerlessWorkgroup) Labels(metadata *types.AWS, tags map[string]string) map[string]string {
	labels := labelsFromAWSMetadata(metadata)
	labels[labelEndpointType] = "workgroup"
	labels[labelNamespace] = aws.StringValue(r.NamespaceName)
	if r.Endpoint != nil && len(r.Endpoint.VpcEndpoints) > 0 {
		labels[labelVPCID] = aws.StringValue(r.Endpoint.VpcEndpoints[0].VpcId)
	}
	return addLabels(labels, tags)
}
func (r RedshiftServerlessWorkgroup) AWSMetadata() (*types.AWS, error) {
	parsedARN, err := arn.Parse(aws.StringValue(r.GetARN()))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &types.AWS{
		Region:    parsedARN.Region,
		AccountID: parsedARN.AccountID,
		RedshiftServerless: types.RedshiftServerless{
			WorkgroupName: aws.StringValue(r.WorkgroupName),
		},
	}, nil
}
func (r RedshiftServerlessWorkgroup) ToDatabase(tags map[string]string) (types.Database, error) {
	if r.Endpoint == nil {
		return nil, trace.BadParameter("missing endpoint")
	}

	metadata, err := r.AWSMetadata()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return types.NewDatabaseV3(
		setDBName(types.Metadata{
			Description: fmt.Sprintf("Redshift Serverless workgroup in %v", metadata.Region),
			Labels:      r.Labels(metadata, tags),
		}, metadata.RedshiftServerless.WorkgroupName),
		types.DatabaseSpecV3{
			Protocol: defaults.ProtocolPostgres,
			URI:      fmt.Sprintf("%v:%v", aws.StringValue(r.Endpoint.Address), aws.Int64Value(r.Endpoint.Port)),
			AWS:      *metadata,
		})
}

// RedshiftServerlessEndpointAccess is a type alias of redshiftserverless.EndpointAccess
type RedshiftServerlessEndpointAccess struct {
	*redshiftserverless.EndpointAccess

	// Workgroup the workgroup that owns the endpoint.
	Workgroup *redshiftserverless.Workgroup
}

// NewRedshiftServerlessEndpointAccess create a new RedshiftServerlessEndpointAccess.
func NewRedshiftServerlessEndpointAccess(endpoint *redshiftserverless.EndpointAccess, workgroup *redshiftserverless.Workgroup) *RedshiftServerlessEndpointAccess {
	return &RedshiftServerlessEndpointAccess{
		EndpointAccess: endpoint,
		Workgroup:      workgroup,
	}
}

func (r RedshiftServerlessEndpointAccess) IsSupported() bool {
	return true // always supported
}
func (r RedshiftServerlessEndpointAccess) IsAvailable() bool {
	return isCommonAWSDatabaseAvailable(r)
}
func (r RedshiftServerlessEndpointAccess) GetStatus() *string {
	return r.EndpointStatus
}
func (r RedshiftServerlessEndpointAccess) GetARN() *string {
	return r.EndpointArn
}
func (r RedshiftServerlessEndpointAccess) String() string {
	return fmt.Sprintf("Redshift Serverless Endpoint %v (Workgroup %v, Namespace %v)",
		aws.StringValue(r.EndpointName),
		aws.StringValue(r.WorkgroupName),
		aws.StringValue(r.Workgroup.NamespaceName),
	)
}
func (r RedshiftServerlessEndpointAccess) Labels(metadata *types.AWS, tags map[string]string) map[string]string {
	labels := labelsFromAWSMetadata(metadata)
	labels[labelEndpointType] = "vpc-endpoint"
	labels[labelWorkgroup] = aws.StringValue(r.WorkgroupName)
	labels[labelNamespace] = aws.StringValue(r.Workgroup.NamespaceName)
	if r.VpcEndpoint != nil {
		labels[labelVPCID] = aws.StringValue(r.VpcEndpoint.VpcId)
	}
	return addLabels(labels, tags)
}
func (r RedshiftServerlessEndpointAccess) AWSMetadata() (*types.AWS, error) {
	parsedARN, err := arn.Parse(aws.StringValue(r.GetARN()))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &types.AWS{
		Region:    parsedARN.Region,
		AccountID: parsedARN.AccountID,
		RedshiftServerless: types.RedshiftServerless{
			WorkgroupName: aws.StringValue(r.WorkgroupName),
			EndpointName:  aws.StringValue(r.EndpointName),
		},
	}, nil
}
func (r RedshiftServerlessEndpointAccess) ToDatabase(tags map[string]string) (types.Database, error) {
	if r.Workgroup.Endpoint == nil {
		return nil, trace.BadParameter("missing endpoint")
	}

	metadata, err := r.AWSMetadata()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return types.NewDatabaseV3(
		setDBName(types.Metadata{
			Description: fmt.Sprintf("Redshift Serverless Endpoint in %v", metadata.Region),
			Labels:      r.Labels(metadata, tags),
		}, metadata.RedshiftServerless.WorkgroupName, metadata.RedshiftServerless.EndpointName),
		types.DatabaseSpecV3{
			Protocol: defaults.ProtocolPostgres,
			URI:      fmt.Sprintf("%v:%v", aws.StringValue(r.Address), aws.Int64Value(r.Port)),
			AWS:      *metadata,

			// Use workgroup's default address as the server name.
			TLS: types.DatabaseTLS{
				ServerName: aws.StringValue(r.Workgroup.Endpoint.Address),
			},
		})
}
