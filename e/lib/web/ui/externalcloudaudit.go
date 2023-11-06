package ui

// GenerateDraftExternalCloudAuditRequest is a request to generate a new draft
// ExternalCloudAudit configuration.
type GenerateDraftExternalCloudAuditRequest struct {
	// IntegrationName is the name of an existing AWS OIDC integration used to
	// authenticate to the customer AWS account.
	IntegrationName string `json:"integration_name"`
}
