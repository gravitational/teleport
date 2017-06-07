package ui

import teleservices "github.com/gravitational/teleport/lib/services"

// TrustedCluster describes trusted cluster properties for WEB client
type TrustedCluster struct {
	// Enabled is this cluster enabled flag
	Enabled bool `json:"enabled"`
	// Name is this cluster name
	Name string `json:"name"`
	// ProxyAddress is this cluster proxy address
	ProxyAddress string `json:"proxyAddress"`
	// Roles is this cluster list of roles
	Roles []string `json:"roles"`
	// ReverseTunnelAddress is this cluster reverse tunnel address
	ReverseTunnelAddress string `json:"reverseTunnelAddress"`
}

// NewTrustedCluster creates a new instance of UI Trusted Cluster
func NewTrustedCluster(teleTrustedClr teleservices.TrustedCluster) TrustedCluster {
	uiTrustedClr := TrustedCluster{
		Enabled:              teleTrustedClr.GetEnabled(),
		Name:                 teleTrustedClr.GetName(),
		Roles:                teleTrustedClr.GetRoles(),
		ProxyAddress:         teleTrustedClr.GetProxyAddress(),
		ReverseTunnelAddress: teleTrustedClr.GetReverseTunnelAddress(),
	}

	return uiTrustedClr
}
