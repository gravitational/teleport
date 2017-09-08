package ui

import (
	"sort"

	"github.com/gravitational/teleport/lib/services"
)

// LabelDTO describes a label
type LabelDTO struct {
	// Name is this label name
	Name string `json:"name"`
	// Value is this label value
	Value string `json:"value"`
}

// ServerDTO describes a server
type ServerDTO struct {
	// Name is this server name
	Name string `json:"id"`
	// ClusterName is this server cluster name
	ClusterName string `json:"siteId"`
	// Hostname is this server hostname
	Hostname string `json:"hostname"`
	// Addrr is this server ip address
	Addr string `json:"addr"`
	// Labels is this server list of labels
	Labels []LabelDTO `json:"tags"`
}

// MakeServerDTOs creates DTO object for Server
func MakeServerDTOs(clusterName string, servers []services.Server) []ServerDTO {
	serverDTOs := []ServerDTO{}
	for _, server := range servers {
		serverLabels := server.GetLabels()
		labelNames := []string{}
		for name := range serverLabels {
			labelNames = append(labelNames, name)
		}

		// sort labels by name and create their dtos
		sort.Strings(labelNames)
		labelDTOs := []LabelDTO{}
		for _, name := range labelNames {
			labelDTOs = append(labelDTOs, LabelDTO{
				Name:  name,
				Value: serverLabels[name],
			})
		}

		serverDTOs = append(serverDTOs, ServerDTO{
			ClusterName: clusterName,
			Name:        server.GetName(),
			Hostname:    server.GetHostname(),
			Addr:        server.GetAddr(),
			Labels:      labelDTOs,
		})
	}

	return serverDTOs
}
