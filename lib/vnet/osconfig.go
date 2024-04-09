package vnet

type osConfig struct {
	tunName               string
	tunIP                 string
	vnetNetmasks          []string
	vnetNameserverAddress string
	dnsZones              []string
}

type osStatus struct {
	upstreamNameserverAddesses []string
}
