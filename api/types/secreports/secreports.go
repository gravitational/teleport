/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package secreports

import (
	"fmt"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/header/convert/legacy"
)

// Report is security report.
type Report struct {
	// ResourceHeader is the resource header.
	header.ResourceHeader
	// Spec is the security report spec.
	Spec ReportSpec `json:"spec" yaml:"spec"`
}

func (a *Report) Clone() *Report {
	if a == nil {
		return nil
	}
	return &Report{
		ResourceHeader: *a.ResourceHeader.Clone(),
		Spec:           *a.Spec.Clone(),
	}
}

// ReportSpec is the security report spec.
type ReportSpec struct {
	// Name is the Report name.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Title is the Report title.
	Title string `json:"title,omitempty" yaml:"title,omitempty"`
	// Description is the Report description.
	Description string `json:"desc,omitempty" yaml:"desc,omitempty"`
	// AuditQueries is the Report audit query.
	AuditQueries []*AuditQuerySpec `json:"audit_queries,omitempty" yaml:"queries,omitempty"`
	// Version is the Reports version.
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
}

func (s *ReportSpec) Clone() *ReportSpec {
	if s == nil {
		return nil
	}
	var auditQueries []*AuditQuerySpec
	if s.AuditQueries != nil {
		auditQueries = make([]*AuditQuerySpec, 0, len(s.AuditQueries))
		for _, auditQuery := range s.AuditQueries {
			auditQueries = append(auditQueries, auditQuery.Clone())
		}
	}
	return &ReportSpec{
		Name:         s.Name,
		Title:        s.Title,
		Description:  s.Description,
		AuditQueries: auditQueries,
		Version:      s.Version,
	}
}

// AuditQuery is the audit query resource.
type AuditQuery struct {
	// ResourceHeader is the resource header.
	header.ResourceHeader
	// Spec is the audit query specification.
	Spec AuditQuerySpec `json:"spec" yaml:"spec"`
}

func (a *AuditQuery) Clone() *AuditQuery {
	if a == nil {
		return nil
	}
	return &AuditQuery{
		ResourceHeader: *a.ResourceHeader.Clone(),
		Spec:           *a.Spec.Clone(),
	}
}

// AuditQuerySpec is the audit query specification.
type AuditQuerySpec struct {
	// Name is the AuditQuery name.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Title is the AuditQuery title.
	Title string `json:"title,omitempty" yaml:"title,omitempty"`
	// Description is the AuditQuery description.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Query is the AuditQuery query.
	Query string `json:"query,omitempty" yaml:"query,omitempty"`
}

func (s *AuditQuerySpec) Clone() *AuditQuerySpec {
	if s == nil {
		return nil
	}
	return &AuditQuerySpec{
		Name:        s.Name,
		Title:       s.Title,
		Description: s.Description,
		Query:       s.Query,
	}
}

// CheckAndSetDefaults validates fields and populates empty fields with default values.
func (a *AuditQuery) CheckAndSetDefaults() error {
	a.SetKind(types.KindAuditQuery)
	a.SetVersion(types.V1)

	if err := a.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// NewAuditQuery creates a new audit query.
func NewAuditQuery(metadata header.Metadata, spec AuditQuerySpec) (*AuditQuery, error) {
	secReport := &AuditQuery{
		ResourceHeader: header.ResourceHeaderFromMetadata(metadata),
		Spec:           spec,
	}
	if err := secReport.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return secReport, nil
}

// NewReport creates a new security report.
func NewReport(metadata header.Metadata, spec ReportSpec) (*Report, error) {
	secReport := &Report{
		ResourceHeader: header.ResourceHeaderFromMetadata(metadata),
		Spec:           spec,
	}
	if err := secReport.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return secReport, nil
}

// CheckAndSetDefaults validates fields and populates empty fields with default values.
func (a *Report) CheckAndSetDefaults() error {
	a.SetKind(types.KindSecurityReport)
	a.SetVersion(types.V1)

	if err := a.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetMetadata returns metadata. This is specifically for conforming to the Resource interface,
// and should be removed when possible.
func (a *Report) GetMetadata() types.Metadata {
	return legacy.FromHeaderMetadata(a.Metadata)
}

// GetMetadata returns metadata. This is specifically for conforming to the Resource interface,
// and should be removed when possible.
func (a *AuditQuery) GetMetadata() types.Metadata {
	return legacy.FromHeaderMetadata(a.Metadata)
}

// Status is the status of the security report.
type Status string

const (
	// Running is the running status. This means the report is currently running.
	Running Status = "RUNNING"
	// Failed is the failed status. This means the report failed to run.
	Failed Status = "FAILED"
	// Ready is the ready status. This means the report is ready to be viewed.
	Ready Status = "READY"
)

// ReportState is the security report state.
type ReportState struct {
	// ResourceHeader is the resource header.
	header.ResourceHeader
	// Spec is the security report state specification.
	Spec ReportStateSpec `json:"spec,omitempty" yaml:"spec,omitempty"`
}

func (a *ReportState) Clone() *ReportState {
	if a == nil {
		return nil
	}

	return &ReportState{
		ResourceHeader: *a.ResourceHeader.Clone(),
		Spec:           *a.Spec.Clone(),
	}
}

// ReportStateSpec is the security report state specification.
type ReportStateSpec struct {
	// Name is the Report name.
	Status Status `json:"status,omitempty" yaml:"status,omitempty"`
	// UpdatedAt is the time the report was updated.
	UpdatedAt time.Time `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

func (s *ReportStateSpec) Clone() *ReportStateSpec {
	if s == nil {
		return nil
	}
	return &ReportStateSpec{
		Status:    s.Status,
		UpdatedAt: s.UpdatedAt,
	}
}

// GetMetadata returns metadata. This is specifically for conforming to the Resource interface,
// and should be removed when possible.
func (a *ReportState) GetMetadata() types.Metadata {
	return legacy.FromHeaderMetadata(a.Metadata)
}

// CheckAndSetDefaults validates fields and populates empty fields with default values.
func (a *ReportState) CheckAndSetDefaults() error {
	a.SetKind(types.KindSecurityReportState)
	a.SetVersion(types.V1)

	if err := a.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// NewReportState creates a new security report state.
func NewReportState(metadata header.Metadata, spec ReportStateSpec) (*ReportState, error) {
	secReport := &ReportState{
		ResourceHeader: header.ResourceHeaderFromMetadata(metadata),
		Spec:           spec,
	}
	if err := secReport.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return secReport, nil
}

// ReportExecutionName returns the name of the report execution.
func ReportExecutionName(name string, days int32) string {
	return fmt.Sprintf("%s_%d_days", name, days)
}

// NewCostLimiter creates a new const limiter.
func NewCostLimiter(metadata header.Metadata, spec CostLimiterSpec) (*CostLimiter, error) {
	item := &CostLimiter{
		ResourceHeader: header.ResourceHeaderFromMetadata(metadata),
		Spec:           spec,
	}
	if err := item.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return item, nil
}

// CostLimiter is the security report state.
type CostLimiter struct {
	// ResourceHeader is the resource header.
	header.ResourceHeader
	// Spec is the security report state specification.
	Spec CostLimiterSpec `json:"spec,omitempty" yaml:"spec,omitempty"`
}

// CostLimiterSpec  is the security report cost limiter specification.
type CostLimiterSpec struct {
	// BytesScanned is the number of bytes scanned.
	BytesScanned uint64 `json:"bytes_scanned,omitempty"`
	// RefillAt is the time when the limiter should be refilled.
	RefillAt time.Time `json:"refill_at,omitempty"`
	// RefillAfter is time duration after which the limiter should be refilled.
	RefillAfter time.Duration `json:"refill_after,omitempty"`
}

func (a *CostLimiter) Reset(refillAt time.Time) {
	a.Spec.RefillAt = refillAt
	a.Spec.BytesScanned = 0
}

// GetMetadata returns metadata. This is specifically for conforming to the Resource interface,
// and should be removed when possible.
func (a *CostLimiter) GetMetadata() types.Metadata {
	return legacy.FromHeaderMetadata(a.Metadata)
}

// CheckAndSetDefaults validates fields and populates empty fields with default values.
func (a *CostLimiter) CheckAndSetDefaults() error {
	a.SetKind(types.KindSecurityReportCostLimiter)
	a.SetVersion(types.V1)

	if err := a.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
