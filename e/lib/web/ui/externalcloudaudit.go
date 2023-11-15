package ui

// GenerateDraftExternalCloudAuditRequest is a request to generate a new draft
// ExternalCloudAudit configuration.
type GenerateDraftExternalCloudAuditRequest struct {
	// IntegrationName is the name of an existing AWS OIDC integration used to
	// authenticate to the customer AWS account.
	IntegrationName string `json:"integration_name"`
}

// ExternalCloudAudit represents an external cloud audit instance
type ExternalCloudAudit struct {
	// IntegrationName is name of existing OIDC integration used to
	// generate AWS credentials.
	IntegrationName string `json:"integration_name" yaml:"integration_name"`
}
