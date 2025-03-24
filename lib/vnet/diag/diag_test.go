// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package diag

import (
	"context"
	"os/exec"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	diagv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/diag/v1"
)

func TestGenerateReport_FailedCheck(t *testing.T) {
	diagCheck := &FakeDiagCheck{shouldFail: true}

	report, err := GenerateReport(context.Background(), ReportPrerequisites{
		Clock:               clockwork.NewFakeClock(),
		NetworkStackAttempt: &diagv1.NetworkStackAttempt{},
		DiagChecks:          []DiagCheck{diagCheck},
	})

	require.NoError(t, err)
	require.Len(t, report.Checks, 1)
	checkAttempt := report.Checks[0]
	require.Equal(t, diagv1.CheckAttemptStatus_CHECK_ATTEMPT_STATUS_ERROR, checkAttempt.Status)

	// Verify that commands are still included even if the check itself failed.
	require.Len(t, checkAttempt.Commands, 1)
	// Verify that CheckReport is not empty, as otherwise it'd be impossible to tell the kind of the
	// check that failed.
	require.NotNil(t, checkAttempt.CheckReport)
}

func TestGenerateReport_SkipCommands(t *testing.T) {
	diagCheck := &FakeDiagCheck{}

	report, err := GenerateReport(context.Background(), ReportPrerequisites{
		Clock:               clockwork.NewFakeClock(),
		NetworkStackAttempt: &diagv1.NetworkStackAttempt{},
		DiagChecks:          []DiagCheck{diagCheck},
		SkipCommands:        true,
	})

	require.NoError(t, err)
	require.Len(t, report.Checks, 1)
	checkAttempt := report.Checks[0]
	require.Equal(t, diagv1.CheckAttemptStatus_CHECK_ATTEMPT_STATUS_OK, checkAttempt.Status)
	require.Empty(t, checkAttempt.Commands)
}

type FakeDiagCheck struct {
	shouldFail bool
}

func (f *FakeDiagCheck) Run(ctx context.Context) (*diagv1.CheckReport, error) {
	if f.shouldFail {
		return nil, trace.Errorf("something went wrong")
	}

	return &diagv1.CheckReport{
		Report: &diagv1.CheckReport_RouteConflictReport{
			RouteConflictReport: &diagv1.RouteConflictReport{
				RouteConflicts: []*diagv1.RouteConflict{},
			},
		},
	}, nil
}

func (f *FakeDiagCheck) Commands(ctx context.Context) []*exec.Cmd {
	return []*exec.Cmd{exec.CommandContext(ctx, "echo", "foo")}
}

func (f *FakeDiagCheck) EmptyCheckReport() *diagv1.CheckReport {
	return &diagv1.CheckReport{}
}
