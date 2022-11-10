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
	"sort"
	"strconv"
	"strings"

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
func MakeServers(clusterName string, servers []types.Server, userRoles services.RoleSet) []Server {
	uiServers := []Server{}
	for _, server := range servers {
		uiLabels := []Label{}
		serverLabels := server.GetStaticLabels()
		for name, value := range serverLabels {
			uiLabels = append(uiLabels, Label{
				Name:  name,
				Value: value,
			})
		}

		serverCmdLabels := server.GetCmdLabels()
		for name, cmd := range serverCmdLabels {
			uiLabels = append(uiLabels, Label{
				Name:  name,
				Value: cmd.GetResult(),
			})
		}

		sort.Sort(sortedLabels(uiLabels))

		serverLogins := userRoles.EnumerateServerLogins(server)

		uiServers = append(uiServers, Server{
			ClusterName: clusterName,
			Labels:      uiLabels,
			Name:        server.GetName(),
			Hostname:    server.GetHostname(),
			Addr:        server.GetAddr(),
			Tunnel:      server.GetUseTunnel(),
			SSHLogins:   serverLogins.Allowed(),
		})
	}

	return uiServers
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
		uiLabels := make([]Label, 0, len(staticLabels)+len(dynamicLabels))

		for name, value := range staticLabels {
			uiLabels = append(uiLabels, Label{
				Name:  name,
				Value: value,
			})
		}

		for name, cmd := range dynamicLabels {
			uiLabels = append(uiLabels, Label{
				Name:  name,
				Value: cmd.GetResult(),
			})
		}

		sort.Sort(sortedLabels(uiLabels))

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
	// DatabaseUsers is the list of allowed Database RBAC users that the user can login.
	DatabaseUsers []string `json:"database_users,omitempty"`
	// DatabaseNames is the list of allowed Database RBAC names that the user can login.
	DatabaseNames []string `json:"database_names,omitempty"`
}

// MakeDatabase creates database objects.
func MakeDatabase(database types.Database, dbUsers, dbNames []string) Database {

	uiLabels := []Label{}

	for name, value := range database.GetAllLabels() {
		uiLabels = append(uiLabels, Label{
			Name:  name,
			Value: value,
		})
	}

	sort.Sort(sortedLabels(uiLabels))

	return Database{
		Name:          database.GetName(),
		Desc:          database.GetDescription(),
		Protocol:      database.GetProtocol(),
		Type:          database.GetType(),
		Labels:        uiLabels,
		DatabaseUsers: dbUsers,
		DatabaseNames: dbNames,
	}
}

// MakeDatabases creates database objects.
func MakeDatabases(databases []types.Database) []Database {
	uiServers := make([]Database, 0, len(databases))
	for _, database := range databases {
		db := MakeDatabase(database, nil /* database Users */, nil /* database Names */)
		uiServers = append(uiServers, db)
	}

	return uiServers
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
}

// MakeDesktop converts a desktop from its API form to a type the UI can display.
func MakeDesktop(windowsDesktop types.WindowsDesktop) Desktop {
	// stripRdpPort strips the default rdp port from an ip address since it is unimportant to display
	stripRdpPort := func(addr string) string {
		splitAddr := strings.Split(addr, ":")
		if len(splitAddr) > 1 && splitAddr[1] == strconv.Itoa(defaults.RDPListenPort) {
			return splitAddr[0]
		}
		return addr
	}
	uiLabels := []Label{}

	for name, value := range windowsDesktop.GetAllLabels() {
		uiLabels = append(uiLabels, Label{
			Name:  name,
			Value: value,
		})
	}

	sort.Sort(sortedLabels(uiLabels))

	return Desktop{
		OS:     constants.WindowsOS,
		Name:   windowsDesktop.GetName(),
		Addr:   stripRdpPort(windowsDesktop.GetAddr()),
		Labels: uiLabels,
		HostID: windowsDesktop.GetHostID(),
	}
}

// MakeDesktops converts desktops from their API form to a type the UI can display.
func MakeDesktops(windowsDesktops []types.WindowsDesktop) []Desktop {
	uiDesktops := make([]Desktop, 0, len(windowsDesktops))

	for _, windowsDesktop := range windowsDesktops {
		uiDesktops = append(uiDesktops, MakeDesktop(windowsDesktop))
	}

	return uiDesktops
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
	uiLabels := []Label{}

	for name, value := range desktopService.GetAllLabels() {
		uiLabels = append(uiLabels, Label{
			Name:  name,
			Value: value,
		})
	}

	sort.Sort(sortedLabels(uiLabels))

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
