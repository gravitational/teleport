package constants

import (
	"fmt"
	"net"
	"os"
)

// GetControlPlaneAPIHost returns the host name of the Houston API
// server
func GetControlPlaneAPIHost() string {
	host, _, _ := net.SplitHostPort(GetControlPlaneAPIAddr())
	return host
}

// GetControlPlaneAPIAddr returns the host name + port of the houston
// API server
func GetControlPlaneAPIAddr() string {
	var (
		host, port string
	)
	// check if HOUSTON_HOSTPORT environment variable is set
	// and return it if so:
	customHostPort := os.Getenv(APIHostEnvVar)
	if customHostPort != "" {
		host, port, _ = net.SplitHostPort(customHostPort)
		// port not specified -> use the default
		if port == "" {
			host = customHostPort
			port = controlPlaneAPIPort
		}
	} else {
		// return 100% default (production) values:
		host = controlPlaneAPIHost
		port = controlPlaneAPIPort
	}
	return net.JoinHostPort(host, port)
}

// GetControlPlaneAPIURL returns the full URL of the houston API
// server
func GetControlPlaneAPIURL() string {
	return fmt.Sprintf("https://%v/api", GetControlPlaneAPIAddr())
}
