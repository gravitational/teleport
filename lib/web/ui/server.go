/*
Copyright 2015 Gravitational, Inc.

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

package ui

import (
	"strconv"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

// Label describes label for webapp
type Label struct {
	// Name is this label name
	Name string `json:"name"`
	// Value is this label value
	Value string `json:"value"`
}

// Server describes a server for webapp
type Server struct {
	// Tunnel indicates of this server is connected over a reverse tunnel.
	Tunnel bool `json:"tunnel"`
	// Name is this server name
	Name string `json:"id"`
	// ClusterName is this server cluster name
	ClusterName string `json:"siteId"`
	// Hostname is this server hostname
	Hostname string `json:"hostname"`
	// Addrr is this server ip address
	Addr string `json:"addr"`
	// Labels is this server list of labels
	Labels []Label `json:"tags"`
	// SSHLogins is the list of logins this user can use on this server
	SSHLogins []string `json:"sshLogins"`
}

// sortedLabels is a sort wrapper that sorts labels by name
type sortedLabels []Label

func (s sortedLabels) Len() int {
	return len(s)
}

func (s sortedLabels) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

func (s sortedLabels) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// MakeServers creates server objects for webapp
func MakeServers(clusterName string, servers []types.Server, userRoles services.RoleSet) ([]Server, error) {
	uiServers := []Server{}
	for _, server := range servers {
		serverLabels := server.GetStaticLabels()
		serverCmdLabels := server.GetCmdLabels()
		uiLabels := makeLabels(serverLabels, transformCommandLabels(serverCmdLabels))

		serverLogins, err := userRoles.GetAllowedLoginsForResource(server)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		uiServers = append(uiServers, Server{
			ClusterName: clusterName,
			Labels:      uiLabels,
			Name:        server.GetName(),
			Hostname:    server.GetHostname(),
			Addr:        server.GetAddr(),
			Tunnel:      server.GetUseTunnel(),
			SSHLogins:   serverLogins,
		})
	}

	return uiServers, nil
}

// KubeCluster describes a kube cluster.
type KubeCluster struct {
	// Name is the name of the kube cluster.
	Name string `json:"name"`
	// Labels is a map of static and dynamic labels associated with an kube cluster.
	Labels []Label `json:"labels"`
	// KubeUsers is the list of allowed Kubernetes RBAC users that the user can impersonate.
	KubeUsers []string `json:"kubernetes_users"`
	// KubeGroups is the list of allowed Kubernetes RBAC groups that the user can impersonate.
	KubeGroups []string `json:"kubernetes_groups"`
}

// MakeKubeClusters creates ui kube objects and returns a list.
func MakeKubeClusters(clusters []types.KubeCluster, userRoles services.RoleSet) []KubeCluster {
	uiKubeClusters := make([]KubeCluster, 0, len(clusters))
	for _, cluster := range clusters {
		staticLabels := cluster.GetStaticLabels()
		dynamicLabels := cluster.GetDynamicLabels()
		uiLabels := makeLabels(staticLabels, transformCommandLabels(dynamicLabels))

		kubeUsers, kubeGroups := getAllowedKubeUsersAndGroupsForCluster(userRoles, cluster)

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
	Labels []Label `json:"labels"`
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
		uiLabels := makeLabels(staticLabels)

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
func getAllowedKubeUsersAndGroupsForCluster(roles services.RoleSet, kube types.KubeCluster) (kubeUsers []string, kubeGroups []string) {
	matcher := services.NewKubernetesClusterLabelMatcher(kube.GetAllLabels())
	// We ignore the TTL verification because we want to include every possibility.
	// Later, if the user certificate expiration is longer than the maximum allowed TTL
	// for the role that defines the `kubernetes_*` principals the request will be
	// denied by Kubernetes Service.
	// We ignore the returning error since we are only interested in allowed users and groups.
	kubeGroups, kubeUsers, _ = roles.CheckKubeGroupsAndUsers(0, true /* force ttl override*/, matcher)
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
	// Name is the name of the database.
	Name string `json:"name"`
	// Desc is the database description.
	Desc string `json:"desc"`
	// Protocol is the database description.
	Protocol string `json:"protocol"`
	// Type is the database type, self-hosted or cloud-hosted.
	Type string `json:"type"`
	// Labels is a map of static and dynamic labels associated with a database.
	Labels []Label `json:"labels"`
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

// MakeDatabase creates database objects.
func MakeDatabase(database types.Database, dbUsers, dbNames []string) Database {
	uiLabels := makeLabels(database.GetAllLabels())

	db := Database{
		Name:          database.GetName(),
		Desc:          database.GetDescription(),
		Protocol:      database.GetProtocol(),
		Type:          database.GetType(),
		Labels:        uiLabels,
		DatabaseUsers: dbUsers,
		DatabaseNames: dbNames,
		Hostname:      stripProtocolAndPort(database.GetURI()),
		URI:           database.GetURI(),
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
func MakeDatabases(databases []types.Database, dbUsers, dbNames []string) []Database {
	uiServers := make([]Database, 0, len(databases))
	for _, database := range databases {
		db := MakeDatabase(database, dbUsers, dbNames)
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
	// OS is the os of this desktop. Should be one of constants.WindowsOS, constants.LinuxOS, or constants.DarwinOS.
	OS string `json:"os"`
	// Name is name (uuid) of the windows desktop.
	Name string `json:"name"`
	// Addr is the network address the desktop can be reached at.
	Addr string `json:"addr"`
	// Labels is a map of static and dynamic labels associated with a desktop.
	Labels []Label `json:"labels"`
	// HostID is the ID of the Windows Desktop Service reporting the desktop.
	HostID string `json:"host_id"`
	// Logins is the list of logins this user can use on this desktop.
	Logins []string `json:"logins"`
}

// MakeDesktop converts a desktop from its API form to a type the UI can display.
func MakeDesktop(windowsDesktop types.WindowsDesktop, userRoles services.RoleSet) (Desktop, error) {
	// stripRdpPort strips the default rdp port from an ip address since it is unimportant to display
	stripRdpPort := func(addr string) string {
		splitAddr := strings.Split(addr, ":")
		if len(splitAddr) > 1 && splitAddr[1] == strconv.Itoa(defaults.RDPListenPort) {
			return splitAddr[0]
		}
		return addr
	}

	uiLabels := makeLabels(windowsDesktop.GetAllLabels())

	logins, err := userRoles.GetAllowedLoginsForResource(windowsDesktop)
	if err != nil {
		return Desktop{}, trace.Wrap(err)
	}

	return Desktop{
		OS:     constants.WindowsOS,
		Name:   windowsDesktop.GetName(),
		Addr:   stripRdpPort(windowsDesktop.GetAddr()),
		Labels: uiLabels,
		HostID: windowsDesktop.GetHostID(),
		Logins: logins,
	}, nil
}

// MakeDesktops converts desktops from their API form to a type the UI can display.
func MakeDesktops(windowsDesktops []types.WindowsDesktop, userRoles services.RoleSet) ([]Desktop, error) {
	uiDesktops := make([]Desktop, 0, len(windowsDesktops))

	for _, windowsDesktop := range windowsDesktops {
		uiDesktop, err := MakeDesktop(windowsDesktop, userRoles)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		uiDesktops = append(uiDesktops, uiDesktop)
	}

	return uiDesktops, nil
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
	Labels []Label `json:"labels"`
}

// MakeDesktop converts a desktop from its API form to a type the UI can display.
func MakeDesktopService(desktopService types.WindowsDesktopService) DesktopService {
	uiLabels := makeLabels(desktopService.GetAllLabels())

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
