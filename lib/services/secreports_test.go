/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
