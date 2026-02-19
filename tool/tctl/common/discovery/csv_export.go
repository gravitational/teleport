package discovery

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"
)

// csvPrefix returns the file name prefix for CSV export files.
// Format: discovery-<subcommand>-<YYYYMMDDTHHMMZ>
func csvPrefix(subcommand string, now time.Time) string {
	return fmt.Sprintf("discovery-%s-%s", subcommand, now.UTC().Format("20060102T1504Z"))
}

// writeCSVFile writes a single CSV file with the given headers and rows.
// Returns the number of data rows written (excluding header).
func writeCSVFile(path string, headers []string, rows [][]string) (int, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, trace.Wrap(err, "creating %s", path)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(headers); err != nil {
		return 0, trace.Wrap(err)
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return 0, trace.Wrap(err)
		}
	}
	w.Flush()
	return len(rows), trace.Wrap(w.Error())
}

// writeCSVFileSet writes multiple CSV files into dir using the given prefix.
// Each entry in files maps a suffix to its headers and rows.
// A summary of written files is printed to w.
func writeCSVFileSet(w io.Writer, dir, prefix string, files []csvFileSpec) error {
	for _, f := range files {
		path := filepath.Join(dir, prefix+"-"+f.suffix+".csv")
		n, err := writeCSVFile(path, f.headers, f.rows)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintf(w, "Wrote %s (%d %s)\n", path, n, pluralize(n, "row", "rows"))
	}
	return nil
}

type csvFileSpec struct {
	suffix  string
	headers []string
	rows    [][]string
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// itoa and btoa are tiny CSV formatting helpers.
func itoa(n int) string  { return fmt.Sprintf("%d", n) }
func btoa(b bool) string { return fmt.Sprintf("%t", b) }

func writeSSMRunsCSV(w io.Writer, dir string, output ssmRunsOutput, now time.Time) error {
	prefix := csvPrefix("ssm-runs", now)
	files := []csvFileSpec{
		{suffix: "summary", headers: ssmRunsSummaryHeaders, rows: ssmRunsSummaryRows(output)},
		{suffix: "vms", headers: ssmRunsVMHeaders, rows: ssmRunsVMRows(output)},
	}
	if len(output.ErrorGroups) > 0 || len(output.SuccessGroups) > 0 {
		files = append(files, csvFileSpec{
			suffix:  "groups",
			headers: ssmRunsGroupHeaders,
			rows:    ssmRunsGroupRows(output),
		})
	}
	return writeCSVFileSet(w, dir, prefix, files)
}

var ssmRunsSummaryHeaders = []string{
	"from", "to", "total_runs", "success_runs", "failed_runs",
	"total_vms", "failing_vms", "fetch_limit", "limit_reached", "suggested_limit",
}

func ssmRunsSummaryRows(o ssmRunsOutput) [][]string {
	return [][]string{{
		formatTime(o.From), formatTime(o.To),
		itoa(o.TotalRuns), itoa(o.SuccessRuns), itoa(o.FailedRuns),
		itoa(o.TotalVMs), itoa(o.FailingVMs),
		itoa(o.FetchLimit), btoa(o.LimitReached), itoa(o.SuggestedLimit),
	}}
}

var ssmRunsVMHeaders = []string{
	"account_id", "instance_id", "region",
	"total_runs", "failed_runs", "success_runs",
	"most_recent_failed", "most_recent_time", "most_recent_exit_code",
	"error_group_id", "most_recent_status",
}

func ssmRunsVMRows(o ssmRunsOutput) [][]string {
	vms := o.VMs
	if vms == nil {
		for _, v := range o.VMsByAccount {
			vms = append(vms, v...)
		}
	}
	rows := make([][]string, 0, len(vms))
	for _, vm := range vms {
		rows = append(rows, []string{
			vm.MostRecent.AccountID, vm.InstanceID, vm.MostRecent.Region,
			itoa(vm.TotalRuns), itoa(vm.FailedRuns), itoa(vm.SuccessRuns),
			btoa(vm.MostRecentFailed), vm.MostRecent.EventTime, vm.MostRecent.ExitCode,
			itoa(vm.ErrorGroupID), vm.MostRecent.Status,
		})
	}
	return rows
}

var ssmRunsGroupHeaders = []string{
	"account_id", "group_type", "group_id", "instance_id", "region", "run_count", "template",
}

func ssmRunsGroupRows(o ssmRunsOutput) [][]string {
	var rows [][]string
	for _, g := range o.ErrorGroups {
		for _, inst := range g.Instances {
			rows = append(rows, []string{
				inst.AccountID, "error", itoa(g.ID),
				inst.InstanceID, inst.Region, itoa(inst.RunCount),
				g.Template,
			})
		}
	}
	for _, g := range o.SuccessGroups {
		for _, inst := range g.Instances {
			rows = append(rows, []string{
				inst.AccountID, "success", itoa(g.ID),
				inst.InstanceID, inst.Region, itoa(inst.RunCount),
				g.Template,
			})
		}
	}
	return rows
}

func writeJoinsCSV(w io.Writer, dir string, output joinsOutput, now time.Time) error {
	prefix := csvPrefix("joins", now)
	files := []csvFileSpec{
		{suffix: "summary", headers: joinsSummaryHeaders, rows: joinsSummaryRows(output)},
		{suffix: "hosts", headers: joinsHostHeaders, rows: joinsHostRows(output)},
	}
	return writeCSVFileSet(w, dir, prefix, files)
}

var joinsSummaryHeaders = []string{
	"from", "to", "total_joins", "success_joins", "failed_joins",
	"total_hosts", "failing_hosts", "fetch_limit", "limit_reached", "suggested_limit",
}

func joinsSummaryRows(o joinsOutput) [][]string {
	return [][]string{{
		formatTime(o.From), formatTime(o.To),
		itoa(o.TotalJoins), itoa(o.SuccessJoins), itoa(o.FailedJoins),
		itoa(o.TotalHosts), itoa(o.FailingHosts),
		itoa(o.FetchLimit), btoa(o.LimitReached), itoa(o.SuggestedLimit),
	}}
}

var joinsHostHeaders = []string{
	"account_id", "host_id", "node_name",
	"total_joins", "failed_joins", "success_joins",
	"most_recent_failed", "most_recent_time", "most_recent_method", "most_recent_success",
}

func joinsHostRows(o joinsOutput) [][]string {
	hosts := o.Hosts
	if hosts == nil {
		for _, v := range o.HostsByAccount {
			hosts = append(hosts, v...)
		}
	}
	rows := make([][]string, 0, len(hosts))
	for _, h := range hosts {
		rows = append(rows, []string{
			h.MostRecent.AccountID, h.HostID, h.NodeName,
			itoa(h.TotalJoins), itoa(h.FailedJoins), itoa(h.SuccessJoins),
			btoa(h.MostRecentFailed), h.MostRecent.EventTime, h.MostRecent.Method, btoa(h.MostRecent.Success),
		})
	}
	return rows
}

func writeInventoryCSV(w io.Writer, dir string, output inventoryOutput, now time.Time) error {
	prefix := csvPrefix("inventory", now)
	files := []csvFileSpec{
		{suffix: "summary", headers: inventorySummaryHeaders, rows: inventorySummaryRows(output)},
		{suffix: "hosts", headers: inventoryHostHeaders, rows: inventoryHostRows(output)},
	}
	return writeCSVFileSet(w, dir, prefix, files)
}

var inventorySummaryHeaders = []string{
	"from", "to", "total_hosts", "online_hosts", "offline_hosts", "failed_hosts",
	"fetch_limit", "limit_reached", "suggested_limit",
}

func inventorySummaryRows(o inventoryOutput) [][]string {
	return [][]string{{
		formatTime(o.From), formatTime(o.To),
		itoa(o.TotalHosts), itoa(o.OnlineHosts), itoa(o.OfflineHosts), itoa(o.FailedHosts),
		itoa(o.FetchLimit), btoa(o.LimitReached), itoa(o.SuggestedLimit),
	}}
}

var inventoryHostHeaders = []string{
	"account_id", "display_id", "host_id", "instance_id", "region", "node_name",
	"state", "method", "is_online",
	"last_ssm_run", "last_join", "last_seen",
	"ssm_runs", "ssm_success", "ssm_failed",
	"joins", "join_success", "join_failed",
}

func inventoryHostRows(o inventoryOutput) [][]string {
	hosts := o.Hosts
	if hosts == nil {
		for _, v := range o.HostsByAccount {
			hosts = append(hosts, v...)
		}
	}
	rows := make([][]string, 0, len(hosts))
	for _, h := range hosts {
		rows = append(rows, []string{
			h.AccountID, h.DisplayID, h.HostID, h.InstanceID, h.Region, h.NodeName,
			string(h.State), h.Method, btoa(h.IsOnline),
			formatTime(h.LastSSMRun), formatTime(h.LastJoin), formatTime(h.LastSeen),
			itoa(h.SSMRuns), itoa(h.SSMSuccess), itoa(h.SSMFailed),
			itoa(h.Joins), itoa(h.JoinSuccess), itoa(h.JoinFailed),
		})
	}
	return rows
}
