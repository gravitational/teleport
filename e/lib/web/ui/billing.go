package ui

// NonBillableUsageSummary is used to report usage summary for resources that are not tracked in Stripe.
type NonBillableUsageSummary struct {
	TrustedDeviceUsage TrustedDeviceUsage `json:"trustedDeviceUsage"`
}

// TrustedDeviceUsage represents devicetrustv1.DevicesUsage in the Web UI.
type TrustedDeviceUsage struct {
	DevicesUsageLimit int32 `json:"devicesUsageLimit,omitempty"`
	DevicesInUse      int32 `json:"devicesInUse,omitempty"`
}
