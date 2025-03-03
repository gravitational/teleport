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

package ui

import (
	"strconv"
	"strings"

	"github.com/gravitational/teleport/api/constants"
	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/ui"
)

// Server describes a server for webapp
type Server struct {
	// Kind is the kind of resource. Used to parse which kind in a list of unified resources in the UI
	Kind string `json:"kind"`
	// Tunnel indicates of this server is connected over a reverse tunnel.
	Tunnel bool `json:"tunnel"`
	// SubKind is a node subkind such as OpenSSH
	SubKind string `json:"subKind"`
	// Name is this server name
	Name string `json:"id"`
	// ClusterName is this server cluster name
	ClusterName string `json:"siteId"`
	// Hostname is this server hostname
	Hostname string `json:"hostname"`
	// Addrr is this server ip address
	Addr string `json:"addr"`
	// Labels is this server list of labels
	Labels []ui.Label `json:"tags"`
	// SSHLogins is the list of logins this user can use on this server
	SSHLogins []string `json:"sshLogins"`
	// AWS contains metadata for instances hosted in AWS.
	AWS *AWSMetadata `json:"aws,omitempty"`
	// RequireRequest indicates if a returned resource is only accessible after an access request
	RequiresRequest bool `json:"requiresRequest,omitempty"`
}

// AWSMetadata describes the AWS metadata for instances hosted in AWS.
// This type is the same as types.AWSInfo but has json fields in camelCase form for the WebUI.
type AWSMetadata struct {
	AccountID   string `json:"accountId"`
	InstanceID  string `json:"instanceId"`
	Region      string `json:"region"`
	VPCID       string `json:"vpcId"`
	Integration string `json:"integration"`
	SubnetID    string `json:"subnetId"`
}

// MakeServer creates a server object for the web ui
func MakeServer(clusterName string, server types.Server, logins []string, requiresRequest bool) Server {
	serverLabels := server.GetStaticLabels()
	serverCmdLabels := server.GetCmdLabels()
	uiLabels := ui.MakeLabelsWithoutInternalPrefixes(serverLabels, ui.TransformCommandLabels(serverCmdLabels))

	uiServer := Server{
		Kind:            server.GetKind(),
		ClusterName:     clusterName,
		Labels:          uiLabels,
		Name:            server.GetName(),
		Hostname:        server.GetHostname(),
		Addr:            server.GetAddr(),
		Tunnel:          server.GetUseTunnel(),
		SubKind:         server.GetSubKind(),
		RequiresRequest: requiresRequest,
		SSHLogins:       logins,
	}

	if server.GetSubKind() == types.SubKindOpenSSHEICENode {
		awsMetadata := server.GetAWSInfo()
		uiServer.AWS = &AWSMetadata{
			AccountID:   awsMetadata.AccountID,
			InstanceID:  awsMetadata.InstanceID,
			Region:      awsMetadata.Region,
			Integration: awsMetadata.Integration,
			SubnetID:    awsMetadata.SubnetID,
			VPCID:       awsMetadata.VPCID,
		}
	}

	return uiServer
}

// EKSCluster represents and EKS cluster, analog of awsoidc.EKSCluster, but used by web ui.
type EKSCluster struct {
	Name                 string     `json:"name"`
	Region               string     `json:"region"`
	Arn                  string     `json:"arn"`
	Labels               []ui.Label `json:"labels"`
	JoinLabels           []ui.Label `json:"joinLabels"`
	Status               string     `json:"status"`
	EndpointPublicAccess bool       `json:"endpointPublicAccess"`
	AuthenticationMode   string     `json:"authenticationMode"`
}

// KubeCluster describes a kube cluster.
type KubeCluster struct {
	// Kind is the kind of resource. Used to parse which kind in a list of unified resources in the UI
	Kind string `json:"kind"`
	// Name is the name of the kube cluster.
	Name string `json:"name"`
	// Labels is a map of static and dynamic labels associated with an kube cluster.
	Labels []ui.Label `json:"labels"`
	// KubeUsers is the list of allowed Kubernetes RBAC users that the user can impersonate.
	KubeUsers []string `json:"kubernetes_users"`
	// KubeGroups is the list of allowed Kubernetes RBAC groups that the user can impersonate.
	KubeGroups []string `json:"kubernetes_groups"`
	// RequireRequest indicates if a returned resource is only accessible after an access request
	RequiresRequest bool `json:"requiresRequest,omitempty"`
}

// MakeKubeCluster creates a kube cluster object for the web ui
func MakeKubeCluster(cluster types.KubeCluster, accessChecker services.AccessChecker, requiresRequest bool) KubeCluster {
	staticLabels := cluster.GetStaticLabels()
	dynamicLabels := cluster.GetDynamicLabels()
	uiLabels := ui.MakeLabelsWithoutInternalPrefixes(staticLabels, ui.TransformCommandLabels(dynamicLabels))
	kubeUsers, kubeGroups := getAllowedKubeUsersAndGroupsForCluster(accessChecker, cluster)
	return KubeCluster{
		Kind:            cluster.GetKind(),
		Name:            cluster.GetName(),
		Labels:          uiLabels,
		KubeUsers:       kubeUsers,
		RequiresRequest: requiresRequest,
		KubeGroups:      kubeGroups,
	}
}

// MakeEKSClusters creates EKS objects for the web UI.
func MakeEKSClusters(clusters []*integrationv1.EKSCluster) []EKSCluster {
	uiEKSClusters := make([]EKSCluster, 0, len(clusters))

	for _, cluster := range clusters {
		uiEKSClusters = append(uiEKSClusters, EKSCluster{
			Name:                 cluster.Name,
			Region:               cluster.Region,
			Arn:                  cluster.Arn,
			Labels:               ui.MakeLabelsWithoutInternalPrefixes(cluster.Labels),
			JoinLabels:           ui.MakeLabelsWithoutInternalPrefixes(cluster.JoinLabels),
			Status:               cluster.Status,
			EndpointPublicAccess: cluster.EndpointPublicAccess,
			AuthenticationMode:   cluster.AuthenticationMode,
		})
	}
	return uiEKSClusters
}

// MakeKubeClusters creates ui kube objects and returns a list.
func MakeKubeClusters(clusters []types.KubeCluster, accessChecker services.AccessChecker) []KubeCluster {
	uiKubeClusters := make([]KubeCluster, 0, len(clusters))
	for _, cluster := range clusters {
		staticLabels := cluster.GetStaticLabels()
		dynamicLabels := cluster.GetDynamicLabels()
		uiLabels := ui.MakeLabelsWithoutInternalPrefixes(staticLabels, ui.TransformCommandLabels(dynamicLabels))

		kubeUsers, kubeGroups := getAllowedKubeUsersAndGroupsForCluster(accessChecker, cluster)

		uiKubeClusters = append(uiKubeClusters, KubeCluster{
			Name:       cluster.GetName(),
			Labels:     uiLabels,
			KubeUsers:  kubeUsers,
			KubeGroups: kubeGroups,
		})
	}

	return uiKubeClusters
}

// KubeResource describes a Kubernetes resource.
type KubeResource struct {
	// Kind is the kind of the Kubernetes resource.
	// Curently supported kinds are: pod.
	Kind string `json:"kind"`
	// Name is the name of the Kubernetes resource.
	Name string `json:"name"`
	// Labels is a map of static associated with a Kubernetes resource.
	Labels []ui.Label `json:"labels"`
	// Namespace is the Kubernetes namespace where the resource is located.
	Namespace string `json:"namespace"`
	// KubeCluster is the Kubernetes cluster the resource blongs to.
	KubeCluster string `json:"cluster"`
}

// MakeKubeResources creates ui kube resource objects and returns a list.
func MakeKubeResources(resources []*types.KubernetesResourceV1, cluster string) []KubeResource {
	uiKubeResources := make([]KubeResource, 0, len(resources))
	for _, resource := range resources {
		staticLabels := resource.GetStaticLabels()
		uiLabels := ui.MakeLabelsWithoutInternalPrefixes(staticLabels)

		uiKubeResources = append(uiKubeResources,
			KubeResource{
				Kind:        resource.Kind,
				Name:        resource.GetName(),
				Labels:      uiLabels,
				Namespace:   resource.Spec.Namespace,
				KubeCluster: cluster,
			},
		)
	}
	return uiKubeResources
}

// getAllowedKubeUsersAndGroupsForCluster works on a given set of roles to return
// a list of allowed `kubernetes_users` and `kubernetes_groups` that can be used
// to access a given kubernetes cluster.
// This function ignores any verification of the TTL associated with
// each Role, and focuses only on listing all users and groups that the user may
// have access to.
func getAllowedKubeUsersAndGroupsForCluster(accessChecker services.AccessChecker, kube types.KubeCluster) (kubeUsers []string, kubeGroups []string) {
	matcher := services.NewKubernetesClusterLabelMatcher(kube.GetAllLabels(), accessChecker.Traits())
	// We ignore the TTL verification because we want to include every possibility.
	// Later, if the user certificate expiration is longer than the maximum allowed TTL
	// for the role that defines the `kubernetes_*` principals the request will be
	// denied by Kubernetes Service.
	// We ignore the returning error since we are only interested in allowed users and groups.
	kubeGroups, kubeUsers, _ = accessChecker.CheckKubeGroupsAndUsers(0, true /* force ttl override*/, matcher)
	return
}

// ConnectionDiagnostic describes a connection diagnostic.
type ConnectionDiagnostic struct {
	// ID is the identifier of the connection diagnostic.
	ID string `json:"id"`
	// Success is whether the connection was successful
	Success bool `json:"success"`
	// Message is the diagnostic summary
	Message string `json:"message"`
	// Traces contains multiple checkpoints results
	Traces []ConnectionDiagnosticTraceUI `json:"traces,omitempty"`
}

// ConnectionDiagnosticTraceUI describes a connection diagnostic trace using a UI representation.
// This is required in order to have a more friendly representation of the enum fields - TraceType and Status.
// They are converted into string instead of using the numbers (as they are represented in gRPC).
type ConnectionDiagnosticTraceUI struct {
	// TraceType as string
	TraceType string `json:"traceType,omitempty"`
	// Status as string
	Status string `json:"status,omitempty"`
	// Details of the trace
	Details string `json:"details,omitempty"`
	// Error in case of failure
	Error string `json:"error,omitempty"`
}

// ConnectionDiagnosticTraceUIFromTypes converts a list of ConnectionDiagnosticTrace into its format for HTTP API.
// This is mostly copying things around and converting the enum into a string value.
func ConnectionDiagnosticTraceUIFromTypes(traces []*types.ConnectionDiagnosticTrace) []ConnectionDiagnosticTraceUI {
	ret := make([]ConnectionDiagnosticTraceUI, 0)

	for _, t := range traces {
		ret = append(ret, ConnectionDiagnosticTraceUI{
			TraceType: t.Type.String(),
			Status:    t.Status.String(),
			Details:   t.Details,
			Error:     t.Error,
		})
	}

	return ret
}

// Database describes a database server.
type Database struct {
	// Kind is the kind of resource. Used to parse which kind in a list of unified resources in the UI
	Kind string `json:"kind"`
	// Name is the name of the database.
	Name string `json:"name"`
	// Desc is the database description.
	Desc string `json:"desc"`
	// Protocol is the database description.
	Protocol string `json:"protocol"`
	// Type is the database type, self-hosted or cloud-hosted.
	Type string `json:"type"`
	// Labels is a map of static and dynamic labels associated with a database.
	Labels []ui.Label `json:"labels"`
	// Hostname is the database connection endpoint (URI) hostname (without port and protocol).
	Hostname string `json:"hostname"`
	// URI of the database.
	URI string `json:"uri"`
	// DatabaseUsers is the list of allowed Database RBAC users that the user can login.
	DatabaseUsers []string `json:"database_users,omitempty"`
	// DatabaseNames is the list of allowed Database RBAC names that the user can login.
	DatabaseNames []string `json:"database_names,omitempty"`
	// AWS contains AWS specific fields.
	AWS *AWS `json:"aws,omitempty"`
	// RequireRequest indicates if a returned resource is only accessible after an access request
	RequiresRequest bool `json:"requiresRequest,omitempty"`
	// SupportsInteractive is a flag to indicate the database supports
	// interactive sessions using database REPLs.
	SupportsInteractive bool `json:"supports_interactive,omitempty"`
}

// AWS contains AWS specific fields.
type AWS struct {
	// embeds types.AWS fields into this struct when des/serializing.
	types.AWS `json:""`
	// Status describes the current server status as reported by AWS.
	// Currently this field is populated for AWS RDS Databases when Listing Databases using the AWS OIDC Integration
	Status string `json:"status,omitempty"`
}

const (
	// LabelStatus is the label key containing the database status, e.g. "available"
	LabelStatus = "status"
)

// DatabaseInteractiveChecker is used to check if the database supports
// interactive sessions using database REPLs.
type DatabaseInteractiveChecker interface {
	IsSupported(protocol string) bool
}

// MakeDatabase creates database objects.
func MakeDatabase(database types.Database, accessChecker services.AccessChecker, interactiveChecker DatabaseInteractiveChecker, requiresRequest bool) Database {
	dbNames := accessChecker.EnumerateDatabaseNames(database)
	var dbUsers []string
	if res, err := accessChecker.EnumerateDatabaseUsers(database); err == nil {
		dbUsers = res.Allowed()
	}

	uiLabels := ui.MakeLabelsWithoutInternalPrefixes(database.GetAllLabels())

	db := Database{
		Kind:                database.GetKind(),
		Name:                database.GetName(),
		Desc:                database.GetDescription(),
		Protocol:            database.GetProtocol(),
		Type:                database.GetType(),
		Labels:              uiLabels,
		DatabaseUsers:       dbUsers,
		DatabaseNames:       dbNames.Allowed(),
		Hostname:            stripProtocolAndPort(database.GetURI()),
		URI:                 database.GetURI(),
		RequiresRequest:     requiresRequest,
		SupportsInteractive: interactiveChecker.IsSupported(database.GetProtocol()),
	}

	if database.IsAWSHosted() {
		dbStatus := ""
		if statusLabel, ok := database.GetAllLabels()[LabelStatus]; ok {
			dbStatus = statusLabel
		}
		db.AWS = &AWS{
			AWS:    database.GetAWS(),
			Status: dbStatus,
		}
	}

	return db
}

// MakeDatabases creates database objects.
func MakeDatabases(databases []*types.DatabaseV3, accessChecker services.AccessChecker, interactiveChecker DatabaseInteractiveChecker) []Database {
	uiServers := make([]Database, 0, len(databases))
	for _, database := range databases {
		db := MakeDatabase(database, accessChecker, interactiveChecker, false /* requiresRequest */)
		uiServers = append(uiServers, db)
	}

	return uiServers
}

// DatabaseService describes a DatabaseService resource.
type DatabaseService struct {
	// Name is the name of the database.
	Name string `json:"name"`
	// ResourceMatchers is a list of resource matchers of the DatabaseService.
	ResourceMatchers []*types.DatabaseResourceMatcher `json:"resource_matchers"`
}

// MakeDatabaseService creates DatabaseService resource.
func MakeDatabaseService(databaseService types.DatabaseService) DatabaseService {
	return DatabaseService{
		Name:             databaseService.GetName(),
		ResourceMatchers: databaseService.GetResourceMatchers(),
	}
}

// MakeDatabaseServices creates database service objects.
func MakeDatabaseServices(databaseServices []types.DatabaseService) []DatabaseService {
	dbServices := make([]DatabaseService, len(databaseServices))
	for i, database := range databaseServices {
		db := MakeDatabaseService(database)
		dbServices[i] = db
	}

	return dbServices
}

// Desktop describes a desktop to pass to the ui.
type Desktop struct {
	// Kind is the kind of resource. Used to parse which kind in a list of unified resources in the UI
	Kind string `json:"kind"`
	// OS is the os of this desktop. Should be one of constants.WindowsOS, constants.LinuxOS, or constants.DarwinOS.
	OS string `json:"os"`
	// Name is name (uuid) of the windows desktop.
	Name string `json:"name"`
	// Addr is the network address the desktop can be reached at.
	Addr string `json:"addr"`
	// Labels is a map of static and dynamic labels associated with a desktop.
	Labels []ui.Label `json:"labels"`
	// HostID is the ID of the Windows Desktop Service reporting the desktop.
	HostID string `json:"host_id"`
	// Logins is the list of logins this user can use on this desktop.
	Logins []string `json:"logins"`
	// RequireRequest indicates if a returned resource is only accessible after an access request
	RequiresRequest bool `json:"requiresRequest,omitempty"`
}

// MakeDesktop converts a desktop from its API form to a type the UI can display.
func MakeDesktop(windowsDesktop types.WindowsDesktop, logins []string, requiresRequest bool) Desktop {
	// stripRdpPort strips the default rdp port from an ip address since it is unimportant to display
	stripRdpPort := func(addr string) string {
		splitAddr := strings.Split(addr, ":")
		if len(splitAddr) > 1 && splitAddr[1] == strconv.Itoa(defaults.RDPListenPort) {
			return splitAddr[0]
		}
		return addr
	}

	uiLabels := ui.MakeLabelsWithoutInternalPrefixes(windowsDesktop.GetAllLabels())

	return Desktop{
		Kind:            windowsDesktop.GetKind(),
		OS:              constants.WindowsOS,
		Name:            windowsDesktop.GetName(),
		Addr:            stripRdpPort(windowsDesktop.GetAddr()),
		Labels:          uiLabels,
		HostID:          windowsDesktop.GetHostID(),
		Logins:          logins,
		RequiresRequest: requiresRequest,
	}
}

// DesktopService describes a desktop service to pass to the ui.
type DesktopService struct {
	// Name is hostname of the Windows Desktop Service.
	Name string `json:"name"`
	// Hostname is hostname of the Windows Desktop Service.
	Hostname string `json:"hostname"`
	// Addr is the network address the Windows Desktop Service can be reached at.
	Addr string `json:"addr"`
	// Labels is a map of static and dynamic labels associated with a desktop.
	Labels []ui.Label `json:"labels"`
}

// MakeDesktopService converts a desktop from its API form to a type the UI can display.
func MakeDesktopService(desktopService types.WindowsDesktopService) DesktopService {
	uiLabels := ui.MakeLabelsWithoutInternalPrefixes(desktopService.GetAllLabels())

	return DesktopService{
		Name:     desktopService.GetName(),
		Hostname: desktopService.GetHostname(),
		Addr:     desktopService.GetAddr(),
		Labels:   uiLabels,
	}
}

// MakeDesktopServices converts desktops from their API form to a type the UI can display.
func MakeDesktopServices(windowsDesktopServices []types.WindowsDesktopService) []DesktopService {
	desktopServices := make([]DesktopService, 0, len(windowsDesktopServices))

	for _, desktopService := range windowsDesktopServices {
		desktopServices = append(desktopServices, MakeDesktopService(desktopService))
	}

	return desktopServices
}

// stripProtocolAndPort returns only the hostname of the URI.
// Handles URIs with no protocol eg: for some database connection
// endpoint the URI can be in the format "hostname:port".
func stripProtocolAndPort(uri string) string {
	stripPort := func(uri string) string {
		splitURI := strings.Split(uri, ":")

		if len(splitURI) > 1 {
			return splitURI[0]
		}

		return uri
	}

	// Ignore protocol.
	// eg: "rediss://some-hostname" or "mongodb+srv://some-hostname"
	splitURI := strings.Split(uri, "//")
	if len(splitURI) > 1 {
		return stripPort(splitURI[1])
	}

	return stripPort(uri)
}
