package ui

// NonBillableUsageSummary is used to report usage summary for resources that are not tracked in Stripe.
type NonBillableUsageSummary struct {
	TrustedDeviceUsage TrustedDeviceUsage `json:"trustedDeviceUsage"`
	AccessRequestUsage AccessRequestUsage `json:"accessRequestUsage"`
}

// TrustedDeviceUsage represents devicetrustv1.DevicesUsage in the Web UI.
type TrustedDeviceUsage struct {
	DevicesUsageLimit int32 `json:"devicesUsageLimit,omitempty"`
	DevicesInUse      int32 `json:"devicesInUse,omitempty"`
}

// AccessRequestUsage access requets usage in the Web UI.
type AccessRequestUsage struct {
	MonthlyLimit int32 `json:"monthlyLimit"`
	MonthlyUsed  int32 `json:"monthlyUsed"`
}
