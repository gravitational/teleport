package ui

import "github.com/gravitational/teleport/api/types/secreports"

// RunReportRequest is a request to run a report.
type RunReportRequest struct {
	// Name is the name of the report.
	Name string `json:"name"`
	// Desc is the description of the report.
	Days int `json:"days"`
}

// RunAuditQueryRequest is a request to run an audit query.
type RunAuditQueryRequest struct {
	// Name is the name of the report.
	Query string `json:"query"`
	// Desc is the description of the report.
	Days int `json:"days"`
}

// ConvAuditQueries converts audit queries.
func ConvAuditQueries(auditQueries []*secreports.AuditQuery) []secreports.AuditQuerySpec {
	var out []secreports.AuditQuerySpec
	for _, v := range auditQueries {
		out = append(out, v.Spec)
	}
	return out
}

// SecurityReportState is a security report state.
type SecurityReportState struct {
	// Status is the status of the report.
	Status string `json:"status"`
	// UpdatedAt is the time the report was updated.
	UpdatedAt string `json:"updated_at"`
}

// SecurityReportSchemaView is a security report schema view.
type SecurityReportSchemaView struct {
	// Name is the name of the view.
	Name string `json:"name,omitempty"`
	// Desc is the description of the view.
	Desc string `json:"desc,omitempty"`
	// Columns is the list of columns.
	Columns []*SecurityReportSchemaColumns `json:"columns,omitempty"`
}

// SecurityReportSchemaColumns is a security report schema columns.
type SecurityReportSchemaColumns struct {
	// Name is the name of the column.
	Name string `json:"name,omitempty"`
	// Type is the type of the column.
	Type string `json:"type,omitempty"`
	// Desc is the description of the column.
	Desc string `json:"desc,omitempty"`
}

// SecurityReportSchema is a security report schema.
type SecurityReportSchema struct {
	// Views is the list of views.
	Views []*SecurityReportSchemaView `json:"views,omitempty"`
}

// SecurityAuditQueries is a list of security audit queries.
type SecurityAuditQueries []SecurityAuditQuery

// SecurityAuditQuery is a security audit query.
type SecurityAuditQuery struct {
	// Name is the name of the query.
	Name string `json:"name,omitempty"`
	// Title is the title of the query.
	Title string `json:"title,omitempty"`
	// Description is the description of the query.
	Description string `json:"description,omitempty"`
	// Query is the query.
	Query string `json:"query,omitempty"`
}
