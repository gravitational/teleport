/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package usertasks

import (
	"fmt"
	"net/url"
	"path"

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	usertasksapi "github.com/gravitational/teleport/api/types/usertasks"
)

// UserTaskDiscoverEKSWithURLs contains the clusters that failed to auto-enroll into the cluster.
type UserTaskDiscoverEKSWithURLs struct {
	*usertasksv1.DiscoverEKS
	// Clusters maps a cluster name to the result of enrolling that cluster into teleport.
	Clusters map[string]*DiscoverEKSClusterWithURLs `json:"clusters,omitempty"`
}

// DiscoverEKSClusterWithURLs contains the result of enrolling an AWS EKS Cluster.
type DiscoverEKSClusterWithURLs struct {
	*usertasksv1.DiscoverEKSCluster

	// ResourceURL is the Amazon Web Console URL to access this EKS Cluster.
	// Always present.
	// Format: https://console.aws.amazon.com/eks/home?region=<region>#/clusters/<cluster-name>
	ResourceURL string `json:"resourceUrl,omitempty"`

	// OpenTeleportAgentURL is the URL to open the Teleport Agent StatefulSet in Amazon EKS Web Console.
	// Present when issue is of type eks-agent-not-connecting.
	// Format: https://console.aws.amazon.com/eks/home?region=<region>#/clusters/<cluster-name>/statefulsets/teleport-kube-agent?namespace=teleport-agent
	OpenTeleportAgentURL string `json:"openTeleportAgentUrl,omitempty"`

	// ManageAccessURL is the URL to open the EKS in Amazon Web Console, in the Manage Access page.
	// Present when issue is of type eks-authentication-mode-unsupported.
	// Format: https://console.aws.amazon.com/eks/home?region=<region>#/clusters/<cluster-name>/manage-access
	ManageAccessURL string `json:"manageAccessUrl,omitempty"`

	// ManageEndpointAccessURL is the URL to open the EKS in Amazon Web Console, in the Manage Endpoint Access page.
	// Present when issue is of type eks-cluster-unreachable and eks-missing-endpoint-public-access.
	// Format: https://console.aws.amazon.com/eks/home?region=<region>#/clusters/<cluster-name>/manage-endpoint-access
	ManageEndpointAccessURL string `json:"manageEndpointAccessUrl,omitempty"`

	// ManageClusterURL is the URL to open the EKS Cluster in Amazon Web Console.
	// Present when issue is of type eks-status-not-active.
	// Format: https://console.aws.amazon.com/eks/home?region=<region>#/clusters/<cluster-name>
	ManageClusterURL string `json:"manageClusterUrl,omitempty"`
}

func withEKSClusterIssueURL(metadata *usertasksv1.UserTask, cluster *usertasksv1.DiscoverEKSCluster) *DiscoverEKSClusterWithURLs {
	ret := &DiscoverEKSClusterWithURLs{
		DiscoverEKSCluster: cluster,
	}
	clusterBaseURL := url.URL{
		Scheme:   "https",
		Host:     "console.aws.amazon.com",
		Path:     path.Join("eks", "home"),
		Fragment: "/clusters/" + cluster.GetName(),
		RawQuery: url.Values{
			"region": []string{metadata.Spec.DiscoverEks.GetRegion()},
		}.Encode(),
	}

	ret.ResourceURL = clusterBaseURL.String()

	switch metadata.Spec.IssueType {
	case usertasksapi.AutoDiscoverEKSIssueAgentNotConnecting:
		clusterBaseURL.Fragment = clusterBaseURL.Fragment + "/statefulsets/teleport-kube-agent?namespace=teleport-agent"
		ret.OpenTeleportAgentURL = clusterBaseURL.String()

	case usertasksapi.AutoDiscoverEKSIssueAuthenticationModeUnsupported:
		clusterBaseURL.Fragment = clusterBaseURL.Fragment + "/manage-access"
		ret.ManageAccessURL = clusterBaseURL.String()

	case usertasksapi.AutoDiscoverEKSIssueClusterUnreachable, usertasksapi.AutoDiscoverEKSIssueMissingEndpoingPublicAccess:
		clusterBaseURL.Fragment = clusterBaseURL.Fragment + "/manage-endpoint-access"
		ret.ManageEndpointAccessURL = clusterBaseURL.String()

	case usertasksapi.AutoDiscoverEKSIssueStatusNotActive:
		ret.ManageClusterURL = clusterBaseURL.String()
	}

	return ret
}

// EKSClustersWithURLs takes a UserTask and enriches the cluster list with URLs.
// Currently, the following URLs will be added:
// - ResourceURL: a link to open the instance in Amazon Web Console.
// The following URLs might be added depending on the issue type:
// - OpenTeleportAgentURL: links directly to the statefulset created during the helm installation
// - ManageAccessURL: links to the Manage Access screen in the Amazon EKS Web Console, for the current EKS Cluster.
// - ManageEndpointAccessURL: links to the Manage Endpoint Access screen in the Amazon EKS Web Console, for the current EKS Cluster.
// - ManageClusterURL: links to the EKS Cluster.
func EKSClustersWithURLs(ut *usertasksv1.UserTask) *UserTaskDiscoverEKSWithURLs {
	clusters := ut.Spec.GetDiscoverEks().GetClusters()
	clustersWithURLs := make(map[string]*DiscoverEKSClusterWithURLs, len(clusters))

	for clusterName, cluster := range clusters {
		clustersWithURLs[clusterName] = withEKSClusterIssueURL(ut, cluster)
	}

	return &UserTaskDiscoverEKSWithURLs{
		DiscoverEKS: ut.Spec.GetDiscoverEks(),
		Clusters:    clustersWithURLs,
	}
}

// UserTaskDiscoverEC2WithURLs contains the instances that failed to auto-enroll into the cluster.
type UserTaskDiscoverEC2WithURLs struct {
	*usertasksv1.DiscoverEC2
	// Instances maps the instance ID name to the result of enrolling that instance into teleport.
	Instances map[string]*DiscoverEC2InstanceWithURLs `json:"instances,omitempty"`
}

// DiscoverEC2InstanceWithURLs contains the result of enrolling an AWS EC2 Instance.
type DiscoverEC2InstanceWithURLs struct {
	*usertasksv1.DiscoverEC2Instance

	// ResourceURL is the Amazon Web Console URL to access this EC2 Instance.
	// Always present.
	// Format: https://console.aws.amazon.com/ec2/home?region=<region>#InstanceDetails:instanceId=<instance-id>
	ResourceURL string `json:"resourceUrl,omitempty"`
}

func withEC2InstanceIssueURL(metadata *usertasksv1.UserTask, instance *usertasksv1.DiscoverEC2Instance) *DiscoverEC2InstanceWithURLs {
	ret := &DiscoverEC2InstanceWithURLs{
		DiscoverEC2Instance: instance,
	}
	instanceBaseURL := url.URL{
		Scheme:   "https",
		Host:     "console.aws.amazon.com",
		Path:     path.Join("ec2", "home"),
		Fragment: "InstanceDetails:instanceId=" + instance.GetInstanceId(),
		RawQuery: url.Values{
			"region": []string{metadata.Spec.DiscoverEc2.GetRegion()},
		}.Encode(),
	}
	ret.ResourceURL = instanceBaseURL.String()

	return ret
}

// EC2InstancesWithURLs takes a UserTask and enriches the instance list with URLs.
// Currently, the following URLs will be added:
// - ResourceURL: a link to open the instance in Amazon Web Console.
func EC2InstancesWithURLs(ut *usertasksv1.UserTask) *UserTaskDiscoverEC2WithURLs {
	instances := ut.Spec.GetDiscoverEc2().GetInstances()
	instancesWithURLs := make(map[string]*DiscoverEC2InstanceWithURLs, len(instances))

	for instanceID, instance := range instances {
		instancesWithURLs[instanceID] = withEC2InstanceIssueURL(ut, instance)
	}

	return &UserTaskDiscoverEC2WithURLs{
		DiscoverEC2: ut.Spec.GetDiscoverEc2(),
		Instances:   instancesWithURLs,
	}
}

// UserTaskDiscoverRDSWithURLs contains the databases that failed to auto-enroll into the cluster.
type UserTaskDiscoverRDSWithURLs struct {
	*usertasksv1.DiscoverRDS
	// Databases maps a database resource id to the result of enrolling that database into teleport.
	// For RDS Aurora Clusters, this is the DBClusterIdentifier.
	// For other RDS databases, this is the DBInstanceIdentifier.
	Databases map[string]*DiscoverRDSDatabaseWithURLs `json:"databases,omitempty"`
}

// DiscoverRDSDatabaseWithURLs contains the result of enrolling an AWS RDS Database.
type DiscoverRDSDatabaseWithURLs struct {
	*usertasksv1.DiscoverRDSDatabase

	// ResourceURL is the Amazon Web Console URL to access this RDS Database.
	// Always present.
	// Format for instances: https://console.aws.amazon.com/rds/home?region=<region>#database:id=<name>;is-cluster=false
	// Format for clusters:  https://console.aws.amazon.com/rds/home?region=<region>#database:id=<name>;is-cluster=true
	ResourceURL string `json:"resourceUrl,omitempty"`

	// ConfigurationURL is the Amazon Web Console URL that shows the configuration of the database.
	// Format https://console.aws.amazon.com/rds/home?region=<region>#database:id=<name>;is-cluster=<is-cluster>;tab=configuration
	ConfigurationURL string `json:"configurationUrl,omitempty"`
}

func withRDSDatabaseIssueURL(metadata *usertasksv1.UserTask, database *usertasksv1.DiscoverRDSDatabase) *DiscoverRDSDatabaseWithURLs {
	ret := &DiscoverRDSDatabaseWithURLs{
		DiscoverRDSDatabase: database,
	}

	fragment := fmt.Sprintf("database:id=%s;is-cluster=%t", database.GetName(), database.GetIsCluster())
	databaseBaseURL := url.URL{
		Scheme:   "https",
		Host:     "console.aws.amazon.com",
		Path:     "/rds/home",
		Fragment: fragment,
		RawQuery: url.Values{
			"region": []string{metadata.GetSpec().GetDiscoverRds().GetRegion()},
		}.Encode(),
	}
	ret.ResourceURL = databaseBaseURL.String()

	switch metadata.GetSpec().GetIssueType() {
	case usertasksapi.AutoDiscoverRDSIssueIAMAuthenticationDisabled:
		databaseBaseURL.Fragment += ";tab=configuration"
		ret.ConfigurationURL = databaseBaseURL.String()
	}

	return ret
}

// RDSDatabasesWithURLs takes a UserTask and enriches the database list with URLs.
// Currently, the following URLs will be added:
// - ResourceURL: a link to open the database in Amazon Web Console.
// - ConfigurationURL: a link to open the database in Amazon Web Console in the configuration tab.
func RDSDatabasesWithURLs(ut *usertasksv1.UserTask) *UserTaskDiscoverRDSWithURLs {
	databases := ut.GetSpec().GetDiscoverRds().GetDatabases()
	databasesWithURLs := make(map[string]*DiscoverRDSDatabaseWithURLs, len(databases))

	for databaseID, database := range databases {
		databasesWithURLs[databaseID] = withRDSDatabaseIssueURL(ut, database)
	}

	return &UserTaskDiscoverRDSWithURLs{
		DiscoverRDS: ut.GetSpec().GetDiscoverRds(),
		Databases:   databasesWithURLs,
	}
}
