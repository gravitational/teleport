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
	"strings"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/timestamppb"

	diagv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/diag/v1"
)

// Diagnostician runs individual diag checks along with their accompanying commands and produces a
// report.
type Diagnostician struct{}

// DiagCheck is an individual diag check run by [Diagnostician].
type DiagCheck interface {
	// Run performs the check.
	Run(context.Context) (*diagv1.CheckReport, error)
	// Commands returns commands accompanying the check which are supposed to help inspect the state of
	// the OS relevant to the given check even if the check itself fails.
	Commands(context.Context) []*exec.Cmd
	// EmptyCheckReport is supposed to return an empty version of [diagv1.CheckReport] belonging to
	// this DiagCheck. If Run fails, it's used to set the correct kind of [diagv1.CheckReport] on
	// [diagv1.CheckResult].
	EmptyCheckReport() *diagv1.CheckReport
}

// ReportPrerequisites are items needed by [Diagnostician] to generate a report.
type ReportPrerequisites struct {
	Clock               clockwork.Clock
	NetworkStackAttempt *diagv1.NetworkStackAttempt
	DiagChecks          []DiagCheck
	// SkipCommands controls whether the report provided by [Diagnostician] is going to include extra
	// commands accompanying each diagnostic check. Useful in contexts where there's no place to
	// display output of those commands.
	SkipCommands bool
}

func (rp *ReportPrerequisites) check() error {
	if rp.Clock == nil {
		return trace.BadParameter("missing clock")
	}

	if rp.NetworkStackAttempt == nil {
		return trace.BadParameter("missing network stack result")
	}

	if len(rp.DiagChecks) == 0 {
		return trace.BadParameter("no diag checks provided")
	}

	return nil
}

// GenerateReport generates a report using the output of the checks provided through [rp].
func (d *Diagnostician) GenerateReport(ctx context.Context, rp ReportPrerequisites) (*diagv1.Report, error) {
	if err := rp.check(); err != nil {
		return nil, trace.Wrap(err)
	}

	report := &diagv1.Report{}
	report.CreatedAt = timestamppb.New(rp.Clock.Now().UTC())
	report.NetworkStackAttempt = rp.NetworkStackAttempt

	for _, diagCheck := range rp.DiagChecks {
		checkAttempt := d.runCheck(ctx, diagCheck, rp.SkipCommands)

		report.Checks = append(report.Checks, checkAttempt)
	}

	return report, nil
}

func (d *Diagnostician) runCheck(ctx context.Context, diagCheck DiagCheck, skipCommands bool) *diagv1.CheckAttempt {
	attempt := &diagv1.CheckAttempt{}

	report, err := diagCheck.Run(ctx)
	if err != nil {
		attempt.Status = diagv1.CheckAttemptStatus_CHECK_ATTEMPT_STATUS_ERROR
		attempt.Error = err.Error()
		// In case of an error, CheckReport needs to be set to an empty value. Otherwise it'd be
		// impossible to identify the type of a failed check.
		attempt.CheckReport = diagCheck.EmptyCheckReport()
	} else {
		attempt.Status = diagv1.CheckAttemptStatus_CHECK_ATTEMPT_STATUS_OK
		attempt.CheckReport = report
	}

	if !skipCommands {
		for _, cmd := range diagCheck.Commands(ctx) {
			attempt.Commands = append(attempt.Commands, d.runCommand(cmd))
		}
	}

	return attempt
}

func (d *Diagnostician) runCommand(cmd *exec.Cmd) *diagv1.CommandAttempt {
	command := strings.Join(cmd.Args, " ")

	output, err := cmd.Output()
	if err != nil {
		return &diagv1.CommandAttempt{
			Status:  diagv1.CommandAttemptStatus_COMMAND_ATTEMPT_STATUS_ERROR,
			Error:   err.Error(),
			Command: command,
		}
	}

	return &diagv1.CommandAttempt{
		Status:  diagv1.CommandAttemptStatus_COMMAND_ATTEMPT_STATUS_OK,
		Command: command,
		Output:  string(output),
	}
}
