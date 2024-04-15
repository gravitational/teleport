package vnet

type osConfig struct {
	tunName               string
	tunIPv4               string
	tunIPv6               string
	vnetNetmasks          []string
	vnetNameserverAddress string
	dnsZones              []string
}

type osStatus struct {
	upstreamNameserverAddesses []string
}
