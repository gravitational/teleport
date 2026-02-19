// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package discovery

import (
	"cmp"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	apievents "github.com/gravitational/teleport/api/types/events"
)

type ssmRunRecord struct {
	EventTime     string `json:"event_time"`
	Code          string `json:"code"`
	InstanceID    string `json:"instance_id"`
	Status        string `json:"status"`
	ExitCode      string `json:"exit_code"`
	AccountID     string `json:"account_id"`
	Region        string `json:"region"`
	CommandID     string `json:"command_id"`
	InvocationURL string `json:"invocation_url"`
	Stdout        string `json:"stdout,omitempty"`
	Stderr        string `json:"stderr,omitempty"`

	parsedEventTime time.Time
}

type ssmRunEventFilters struct {
	InstanceID string
}

func parseSSMRunEvents(eventList []apievents.AuditEvent, filters ssmRunEventFilters) []ssmRunRecord {
	records := make([]ssmRunRecord, 0, len(eventList))
	for _, event := range eventList {
		run, ok := event.(*apievents.SSMRun)
		if !ok {
			continue
		}

		record := ssmRunRecord{
			Code:          run.Code,
			InstanceID:    run.InstanceID,
			Status:        run.Status,
			ExitCode:      fmt.Sprintf("%d", run.ExitCode),
			AccountID:     run.AccountID,
			Region:        run.Region,
			CommandID:     run.CommandID,
			InvocationURL: run.InvocationURL,
			Stdout:        run.StandardOutput,
			Stderr:        run.StandardError,
		}
		if !run.Time.IsZero() {
			record.parsedEventTime = run.Time.UTC()
			record.EventTime = record.parsedEventTime.Format(time.RFC3339Nano)
		}

		if filters.InstanceID != "" && !strings.EqualFold(strings.TrimSpace(record.InstanceID), strings.TrimSpace(filters.InstanceID)) {
			continue
		}
		records = append(records, record)
	}

	sortSSMRunRecords(records)
	return records
}

type ssmRunAnalysis struct {
	Total            int            `json:"total"`
	Success          int            `json:"success"`
	Failed           int            `json:"failed"`
	ByInstance       map[string]int `json:"by_instance"`
	FailedByInstance map[string]int `json:"failed_by_instance"`
}

func analyzeSSMRuns(records []ssmRunRecord) ssmRunAnalysis {
	analysis := ssmRunAnalysis{
		Total:            len(records),
		ByInstance:       map[string]int{},
		FailedByInstance: map[string]int{},
	}

	for _, record := range records {
		instanceID := cmp.Or(strings.TrimSpace(record.InstanceID), "unknown")
		analysis.ByInstance[instanceID]++

		if isSSMRunFailure(record) {
			analysis.Failed++
			analysis.FailedByInstance[instanceID]++
		} else {
			analysis.Success++
		}
	}

	return analysis
}

func isSSMRunFailure(record ssmRunRecord) bool {
	if strings.EqualFold(strings.TrimSpace(record.Code), "TDS00W") {
		return true
	}
	status := strings.TrimSpace(record.Status)
	if status == "" {
		return false
	}
	return !strings.EqualFold(status, "Success")
}

type ssmVMGroup struct {
	InstanceID         string         `json:"instance_id"`
	MostRecent         ssmRunRecord   `json:"most_recent"`
	MostRecentFailed   bool           `json:"most_recent_failed"`
	TotalRuns          int            `json:"total_runs"`
	FailedRuns         int            `json:"failed_runs"`
	SuccessRuns        int            `json:"success_runs"`
	StatusByMostRecent map[string]int `json:"status_by_most_recent,omitempty"`
	Runs               []ssmRunRecord `json:"-"`

	// ErrorGroupID is the group assignment for this VM's most recent failed run.
	// Set to -1 when grouping is not enabled or no failed runs exist.
	ErrorGroupID int `json:"error_group_id,omitempty"`
}

func groupSSMRunsByVM(records []ssmRunRecord) []ssmVMGroup {
	byVM := map[string][]ssmRunRecord{}
	for _, record := range records {
		instanceID := cmp.Or(strings.TrimSpace(record.InstanceID), "unknown")
		byVM[instanceID] = append(byVM[instanceID], record)
	}

	groups := make([]ssmVMGroup, 0, len(byVM))
	for instanceID, vmRuns := range byVM {
		sortSSMRunRecords(vmRuns)

		group := ssmVMGroup{
			InstanceID:       instanceID,
			MostRecent:       vmRuns[0],
			MostRecentFailed: isSSMRunFailure(vmRuns[0]),
			TotalRuns:        len(vmRuns),
			Runs:             vmRuns,
		}
		for _, run := range vmRuns {
			if isSSMRunFailure(run) {
				group.FailedRuns++
			} else {
				group.SuccessRuns++
			}
		}
		groups = append(groups, group)
	}

	slices.SortFunc(groups, func(a, b ssmVMGroup) int {
		if c := compareTimeDesc(a.MostRecent.parsedEventTime, b.MostRecent.parsedEventTime); c != 0 {
			return c
		}
		return cmp.Compare(a.InstanceID, b.InstanceID)
	})

	return groups
}

func selectFailingVMGroups(groups []ssmVMGroup, limit int) []ssmVMGroup {
	out := make([]ssmVMGroup, 0, len(groups))
	for _, group := range groups {
		if !group.MostRecentFailed {
			continue
		}
		out = append(out, group)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

type ssmRunHistoryRow struct {
	Timestamp string `json:"timestamp"`
	Result    string `json:"result"`
	CommandID string `json:"command_id"`
	ExitCode  string `json:"exit_code"`
	Output    string `json:"output,omitempty"`
}

func buildVMHistoryRows(group ssmVMGroup, showAll bool) []ssmRunHistoryRow {
	runs := group.Runs
	if !showAll && len(runs) > 1 {
		runs = runs[:1]
	}

	rows := make([]ssmRunHistoryRow, 0, len(runs))
	for _, run := range runs {
		timestamp := cmp.Or(formatMaybeParsedTime(run.parsedEventTime), run.EventTime)
		rows = append(rows, ssmRunHistoryRow{
			Timestamp: timestamp,
			Result:    cmp.Or(run.Status, run.Code),
			CommandID: run.CommandID,
			ExitCode:  run.ExitCode,
			Output:    combineOutput(run.Stdout, run.Stderr),
		})
	}
	return rows
}

type ssmRunsOutput struct {
	Window         string            `json:"-"`
	From           time.Time         `json:"from"`
	To             time.Time         `json:"to"`
	FetchLimit     int               `json:"fetch_limit"`
	LimitReached   bool              `json:"limit_reached"`
	SuggestedLimit int               `json:"suggested_limit,omitempty"`
	CacheSummary  string            `json:"cache_summary,omitempty"`
	TotalRuns     int               `json:"total_runs"`
	SuccessRuns   int               `json:"success_runs"`
	FailedRuns    int               `json:"failed_runs"`
	TotalVMs      int               `json:"total_vms"`
	FailingVMs    int               `json:"failing_vms"`
	VMPage        pageInfo          `json:"vm_page"`
	VMs           []ssmVMGroup      `json:"vms"`
	ErrorGroups      []ssmRunGroup `json:"error_groups,omitempty"`
	SuccessGroups    []ssmRunGroup `json:"success_groups,omitempty"`
	ErrorGroupStats  *groupingStats `json:"error_group_stats,omitempty"`
	SuccessGroupStats *groupingStats `json:"success_group_stats,omitempty"`

	// VMsByAccount groups VMs by AWS account ID. Populated when --group-by-account is set.
	// When set, VMs is nil.
	VMsByAccount map[string][]ssmVMGroup `json:"vms_by_account,omitempty"`
}

// ssmRunGroup is an enriched group for JSON output. Instead of
// opaque index arrays, members are grouped by (instance_id, account_id, region)
// with sorted occurrence timestamps.
type ssmRunGroup struct {
	ID        int                   `json:"id"`
	Template  string                `json:"template"`
	Instances []ssmRunGroupInstance `json:"instances"`

	// Debug fields, populated when --group-debug is set.
	UniqueTexts int `json:"unique_texts,omitempty"`
	ShingleSize int `json:"shingle_size,omitempty"`
}

type ssmRunGroupInstance struct {
	InstanceID string   `json:"instance_id"`
	AccountID  string   `json:"account_id"`
	Region     string   `json:"region"`
	RunCount   int      `json:"run_count"`
	Times      []string `json:"-"`
}

// assignGroupIDs sets ErrorGroupID on each VM based on error group membership.
// Group IDs are 1-indexed for display (0 means unassigned).
func assignGroupIDs(vms []ssmVMGroup, errorGroups []ssmRunGroup) {
	instanceToGroup := make(map[string]int)
	for _, g := range errorGroups {
		for _, inst := range g.Instances {
			instanceToGroup[inst.InstanceID] = g.ID + 1 // 1-indexed
		}
	}
	for i := range vms {
		if id, ok := instanceToGroup[vms[i].InstanceID]; ok {
			vms[i].ErrorGroupID = id
		}
	}
}

// indexedRun pairs a run's combined output with its source record.
type indexedRun struct {
	output string
	record ssmRunRecord
}

// groupSSMRuns groups SSM run outputs into groups of similar runs,
// splitting by success/failure first so each bucket gets independent
// Drain training. The opts control the Drain similarity threshold
// and other pipeline parameters.
func groupSSMRuns(vmGroups []ssmVMGroup, opts groupingOptions, debug bool) (errors, successes []ssmRunGroup, errorStats, successStats groupingStats) {
	var errorRuns, successRuns []indexedRun
	for _, vm := range vmGroups {
		for _, run := range vm.Runs {
			output := combineOutput(run.Stdout, run.Stderr)
			if strings.TrimSpace(output) == "" {
				output = strings.TrimSpace(run.Status)
			}
			if output == "" {
				continue
			}
			ir := indexedRun{output: output, record: run}
			if isSSMRunFailure(run) {
				errorRuns = append(errorRuns, ir)
			} else {
				successRuns = append(successRuns, ir)
			}
		}
	}
	slog.Debug("Grouping SSM runs", "error_runs", len(errorRuns), "success_runs", len(successRuns))
	errors, errorStats = enrichGroups(errorRuns, opts, debug)
	successes, successStats = enrichGroups(successRuns, opts, debug)
	return
}

// enrichGroups runs the Drain -> MinHash -> LSH grouping pipeline on a set of
// SSM runs and converts the raw text groups into structured ssmRunGroup values
// with per-instance member details (account, region, timestamps).
func enrichGroups(runs []indexedRun, opts groupingOptions, debug bool) ([]ssmRunGroup, groupingStats) {
	if len(runs) == 0 {
		return nil, groupingStats{}
	}
	outputs := make([]string, len(runs))
	for i, r := range runs {
		outputs[i] = r.output
	}
	rawGroups, stats := groupTexts(outputs, opts)

	enriched := make([]ssmRunGroup, 0, len(rawGroups))
	for _, rc := range rawGroups {
		type instanceKey struct{ instanceID, accountID, region string }
		grouped := map[instanceKey][]string{}
		for _, idx := range rc.Members {
			rec := runs[idx].record
			key := instanceKey{
				instanceID: rec.InstanceID,
				accountID:  rec.AccountID,
				region:     rec.Region,
			}
			ts := rec.EventTime
			if ts == "" && !rec.parsedEventTime.IsZero() {
				ts = rec.parsedEventTime.Format(time.RFC3339)
			}
			grouped[key] = append(grouped[key], ts)
		}

		instances := make([]ssmRunGroupInstance, 0, len(grouped))
		for key, times := range grouped {
			slices.Sort(times)
			instances = append(instances, ssmRunGroupInstance{
				InstanceID: key.instanceID,
				AccountID:  key.accountID,
				Region:     key.region,
				RunCount:   len(times),
				Times:      times,
			})
		}
		slices.SortFunc(instances, func(a, b ssmRunGroupInstance) int {
			return cmp.Compare(a.InstanceID, b.InstanceID)
		})

		c := ssmRunGroup{
			ID:        rc.ID,
			Template:  rc.Template,
			Instances: instances,
		}
		if debug {
			c.UniqueTexts = rc.UniqueTexts
			c.ShingleSize = rc.ShingleSize
		}
		enriched = append(enriched, c)
	}
	return enriched, stats
}

// groupSSMRunsByAccount replaces the flat VMs list with an account-keyed map.
func (o *ssmRunsOutput) groupByAccount() {
	o.VMsByAccount = groupByAccountField(o.VMs, func(vm ssmVMGroup) string {
		return vm.MostRecent.AccountID
	})
	o.VMs = nil
}
