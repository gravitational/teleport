package ui

// GenerateDraftExternalAuditStorageRequest is a request to generate a new draft
// ExternalAuditStorage configuration.
type GenerateDraftExternalAuditStorageRequest struct {
	// IntegrationName is the name of an existing AWS OIDC integration used to
	// authenticate to the customer AWS account.
	IntegrationName string `json:"integration_name"`
}

// ExternalAuditStorage represents an External Audit Storage instance.
type ExternalAuditStorage struct {
	// IntegrationName is name of existing OIDC integration used to
	// generate AWS credentials.
	IntegrationName string `json:"integration_name" yaml:"integration_name"`
}
