package reports

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/secreports"
)

var (
	// PrebuiltReports is the list of prebuild reports.
	PrebuiltReports = []*AuditReportType{
		PrivilegeAccessReport,
	}
)

// AuditReportsType is the type for audit reports.
type AuditReportsType []*AuditReportType

// AuditReportType is the type for audit report.
type AuditReportType struct {
	// Version is the report version.
	Version string
	// Name is the report name.
	Name string
	// Title is the report title.
	Title string
	// Description is the report description.
	Description string
	// Queries is the list of queries.
	Queries []AuditQueryType
}

// AuditQueryType is the type for audit query.
type AuditQueryType struct {
	// Name is the query name.
	Name string
	// Title is the query title.
	Title string
	// Query is the query.
	Query string
	// Description is the query description.
	Description string
}

// ToSecurityReportType converts the audit report type to security report type.
func ToSecurityReportType(in *AuditReportType) (*secreports.Report, error) {
	var auditQuerySpec []*secreports.AuditQuerySpec
	for _, v := range in.Queries {
		auditQuerySpec = append(auditQuerySpec, &secreports.AuditQuerySpec{
			Name:        v.Name,
			Title:       v.Title,
			Query:       v.Query,
			Description: v.Description,
		})
	}
	accessReport, err := secreports.NewReport(header.Metadata{Name: in.Name}, secreports.ReportSpec{
		Name:         in.Name,
		Title:        in.Title,
		Description:  in.Description,
		AuditQueries: auditQuerySpec,
		Version:      in.Version,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return accessReport, nil
}

// IsPreBuiltReport returns true if the report is Build-In and not modifiable.
func IsPreBuiltReport(name string) bool {
	for _, v := range PrebuiltReports {
		if v.Name == name {
			return true
		}
	}
	return false
}
