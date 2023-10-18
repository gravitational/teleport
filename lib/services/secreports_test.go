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

package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/secreports"
)

func TestMarshalUnmarshalAuditQuery(t *testing.T) {
	want, err := secreports.NewAuditQuery(
		header.Metadata{Name: "audit_query"},
		secreports.AuditQuerySpec{
			Name:        "audit_query_example",
			Title:       "Audit Query Example Title",
			Description: "Audit Query Description",
			Query:       "SELECT 1",
		},
	)
	require.NoError(t, err)
	data, err := MarshalAuditQuery(want)
	require.NoError(t, err)
	got, err := UnmarshalAuditQuery(data)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestMarshalUnmarshalSecurityReport(t *testing.T) {
	want, err := secreports.NewReport(
		header.Metadata{Name: "security_report"},
		secreports.ReportSpec{
			Name:        "Security Report",
			Title:       "Security Report Name",
			Description: "Description",
			AuditQueries: []*secreports.AuditQuerySpec{
				{
					Name:        "audit_query_example",
					Title:       "Audit Query Example Title",
					Description: "Audit Query Description",
					Query:       "SELECT 1",
				},
			},
			Version: "0.0.1",
		},
	)

	require.NoError(t, err)
	data, err := MarshalSecurityReport(want)
	require.NoError(t, err)
	got, err := UnmarshalSecurityReport(data)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestMarshalUnmarshalSecurityReportState(t *testing.T) {
	want, err := secreports.NewReportState(
		header.Metadata{Name: "security_report_state"},
		secreports.ReportStateSpec{
			Status:    secreports.Ready,
			UpdatedAt: time.Now().UTC(),
		},
	)

	require.NoError(t, err)
	data, err := MarshalSecurityReportState(want)
	require.NoError(t, err)
	got, err := UnmarshalSecurityReportState(data)
	require.NoError(t, err)
	require.Equal(t, want, got)
}
