package ui

// NonBillableUsageSummary is used to report usage summary for resources that are not tracked in Stripe.
type NonBillableUsageSummary struct {
	AccessRequestUsage AccessRequestUsage `json:"accessRequestUsage"`
}

// AccessRequestUsage access requets usage in the Web UI.
type AccessRequestUsage struct {
	MonthlyLimit int32 `json:"monthlyLimit"`
	MonthlyUsed  int32 `json:"monthlyUsed"`
}
