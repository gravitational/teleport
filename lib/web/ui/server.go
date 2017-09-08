package ui

import (
	"sort"

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
}

// MakeServers creates server objects for webapp
func MakeServers(clusterName string, servers []services.Server) []Server {
	uiServers := []Server{}
	for _, server := range servers {
		serverLabels := server.GetLabels()
		labelNames := []string{}
		for name := range serverLabels {
			labelNames = append(labelNames, name)
		}

		// sort labels by name
		sort.Strings(labelNames)
		uiLabels := []Label{}
		for _, name := range labelNames {
			uiLabels = append(uiLabels, Label{
				Name:  name,
				Value: serverLabels[name],
			})
		}

		uiServers = append(uiServers, Server{
			ClusterName: clusterName,
			Name:        server.GetName(),
			Hostname:    server.GetHostname(),
			Addr:        server.GetAddr(),
			Labels:      uiLabels,
		})
	}

	return uiServers
}
