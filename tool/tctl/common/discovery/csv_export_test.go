package discovery

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCSVPrefix(t *testing.T) {
	ts := time.Date(2026, 2, 18, 12, 32, 0, 0, time.UTC)
	require.Equal(t, "discovery-ssm-runs-20260218T1232Z", csvPrefix("ssm-runs", ts))
	require.Equal(t, "discovery-joins-20260218T1232Z", csvPrefix("joins", ts))
	require.Equal(t, "discovery-inventory-20260218T1232Z", csvPrefix("inventory", ts))
}

func TestWriteCSVFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.csv")
	headers := []string{"name", "value"}
	rows := [][]string{
		{"alpha", "1"},
		{"beta", "2"},
	}

	n, err := writeCSVFile(path, headers, rows)
	require.NoError(t, err)
	require.Equal(t, 2, n)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	requireContainsAll(t, string(data), `
		name,value
		alpha,1
		beta,2
	`)
}

func TestWriteCSVFile_EmptyRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.csv")

	n, err := writeCSVFile(path, []string{"a", "b"}, nil)
	require.NoError(t, err)
	require.Equal(t, 0, n)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), "a,b")
}

func TestSSMRunsCSV(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 2, 18, 12, 32, 0, 0, time.UTC)
	output := ssmRunsOutput{
		From:        now.Add(-24 * time.Hour),
		To:          now,
		FetchLimit:  10000,
		TotalRuns:   100,
		SuccessRuns: 10,
		FailedRuns:  90,
		TotalVMs:    3,
		FailingVMs:  2,
		VMs: []ssmVMGroup{
			{
				InstanceID:       "i-001",
				MostRecent:       ssmRunRecord{EventTime: "2026-02-18T12:00:00Z", AccountID: "111", Region: "us-east-1", Status: "Failed", ExitCode: "1"},
				MostRecentFailed: true,
				TotalRuns:        50,
				FailedRuns:       50,
			},
			{
				InstanceID:       "i-002",
				MostRecent:       ssmRunRecord{EventTime: "2026-02-18T11:00:00Z", AccountID: "222", Region: "us-west-2", Status: "Success", ExitCode: "0"},
				MostRecentFailed: false,
				TotalRuns:        50,
				FailedRuns:       40,
				SuccessRuns:      10,
			},
		},
	}

	var buf bytes.Buffer
	err := writeSSMRunsCSV(&buf, dir, output, now)
	require.NoError(t, err)

	summary, err := os.ReadFile(filepath.Join(dir, "discovery-ssm-runs-20260218T1232Z-summary.csv"))
	require.NoError(t, err)
	requireContainsAll(t, string(summary), `
		total_runs
		100
	`)

	vms, err := os.ReadFile(filepath.Join(dir, "discovery-ssm-runs-20260218T1232Z-vms.csv"))
	require.NoError(t, err)
	requireContainsAll(t, string(vms), `
		instance_id
		i-001
		i-002
	`)

	// No groups file without --group.
	_, err = os.Stat(filepath.Join(dir, "discovery-ssm-runs-20260218T1232Z-groups.csv"))
	require.True(t, os.IsNotExist(err))

	requireContainsAll(t, buf.String(), `
		summary.csv
		vms.csv
	`)
}

func TestSSMRunsCSV_WithGroups(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 2, 18, 12, 32, 0, 0, time.UTC)
	output := ssmRunsOutput{
		From:        now.Add(-24 * time.Hour),
		To:          now,
		FetchLimit:  10000,
		TotalRuns:   10,
		FailedRuns:  8,
		SuccessRuns: 2,
		TotalVMs:    2,
		FailingVMs:  1,
		ErrorGroups: []ssmRunGroup{
			{
				ID:       0,
				Template: "EC2 not registered",
				Instances: []ssmRunGroupInstance{
					{InstanceID: "i-001", AccountID: "111", Region: "us-east-1", RunCount: 5},
					{InstanceID: "i-002", AccountID: "111", Region: "us-east-1", RunCount: 3},
				},
			},
		},
		SuccessGroups: []ssmRunGroup{
			{
				ID:       0,
				Template: "Install success",
				Instances: []ssmRunGroupInstance{
					{InstanceID: "i-003", AccountID: "222", Region: "us-west-2", RunCount: 2},
				},
			},
		},
	}

	var buf bytes.Buffer
	err := writeSSMRunsCSV(&buf, dir, output, now)
	require.NoError(t, err)

	groups, err := os.ReadFile(filepath.Join(dir, "discovery-ssm-runs-20260218T1232Z-groups.csv"))
	require.NoError(t, err)
	requireContainsAll(t, string(groups), `
		account_id,group_type,group_id,instance_id,region,run_count,template
		error
		success
		i-001
		i-003
	`)

	require.Contains(t, buf.String(), "groups.csv")
}

func TestJoinsCSV(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 2, 18, 12, 32, 0, 0, time.UTC)
	output := joinsOutput{
		From:         now.Add(-24 * time.Hour),
		To:           now,
		FetchLimit:   10000,
		TotalJoins:   20,
		SuccessJoins: 15,
		FailedJoins:  5,
		TotalHosts:   2,
		FailingHosts: 1,
		Hosts: []joinGroup{
			{
				HostID:           "host-001",
				NodeName:         "node-1",
				MostRecent:       joinRecord{EventTime: "2026-02-18T12:00:00Z", Method: "ec2", Success: true},
				MostRecentFailed: false,
				TotalJoins:       10,
				FailedJoins:      2,
				SuccessJoins:     8,
			},
			{
				HostID:           "host-002",
				NodeName:         "node-2",
				MostRecent:       joinRecord{EventTime: "2026-02-18T11:00:00Z", Method: "iam", Success: false},
				MostRecentFailed: true,
				TotalJoins:       10,
				FailedJoins:      3,
				SuccessJoins:     7,
			},
		},
	}

	var buf bytes.Buffer
	err := writeJoinsCSV(&buf, dir, output, now)
	require.NoError(t, err)

	summary, err := os.ReadFile(filepath.Join(dir, "discovery-joins-20260218T1232Z-summary.csv"))
	require.NoError(t, err)
	requireContainsAll(t, string(summary), `
		total_joins
		20
	`)

	hosts, err := os.ReadFile(filepath.Join(dir, "discovery-joins-20260218T1232Z-hosts.csv"))
	require.NoError(t, err)
	requireContainsAll(t, string(hosts), `
		host_id
		host-001
		host-002
	`)

	requireContainsAll(t, buf.String(), `
		summary.csv
		hosts.csv
	`)
}

func TestInventoryCSV(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 2, 18, 12, 32, 0, 0, time.UTC)
	output := inventoryOutput{
		From:         now.Add(-24 * time.Hour),
		To:           now,
		FetchLimit:   10000,
		TotalHosts:   2,
		OnlineHosts:  1,
		OfflineHosts: 1,
		Hosts: []inventoryHost{
			{
				DisplayID:  "i-001",
				HostID:     "uuid-1",
				InstanceID: "i-001",
				AccountID:  "111",
				Region:     "us-east-1",
				NodeName:   "node-1",
				State:      inventoryStateOnline,
				Method:     "ec2",
				IsOnline:   true,
				LastSeen:   now.Add(-5 * time.Minute),
				SSMRuns:    10,
				SSMSuccess: 10,
			},
			{
				DisplayID:  "i-002",
				HostID:     "uuid-2",
				InstanceID: "i-002",
				AccountID:  "222",
				Region:     "us-west-2",
				NodeName:   "node-2",
				State:      inventoryStateOffline,
				Method:     "iam",
				IsOnline:   false,
				LastJoin:   now.Add(-1 * time.Hour),
				Joins:      5,
				JoinSuccess: 5,
			},
		},
	}

	var buf bytes.Buffer
	err := writeInventoryCSV(&buf, dir, output, now)
	require.NoError(t, err)

	summary, err := os.ReadFile(filepath.Join(dir, "discovery-inventory-20260218T1232Z-summary.csv"))
	require.NoError(t, err)
	requireContainsAll(t, string(summary), `
		total_hosts
		2
	`)

	hosts, err := os.ReadFile(filepath.Join(dir, "discovery-inventory-20260218T1232Z-hosts.csv"))
	require.NoError(t, err)
	requireContainsAll(t, string(hosts), `
		account_id,display_id,host_id,instance_id
		i-001
		i-002
		Online
		Offline
	`)

	requireContainsAll(t, buf.String(), `
		summary.csv
		hosts.csv
	`)
}
