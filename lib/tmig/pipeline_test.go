package tmig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tmig/classify"
	"github.com/gravitational/teleport/lib/tmig/config"
	"github.com/gravitational/teleport/lib/tmig/report"
	"github.com/gravitational/teleport/lib/tmig/rewrite"
)

func TestBuildPreflightReport(t *testing.T) {
	cfg := &config.Config{
		Target:       config.TargetConfig{Proxy: "scoped.example.com:443"},
		Concurrency:  32,
		MarkerPrefix: "tmig.teleport.dev",
		Migrations: []config.Migration{{
			Source: config.SourceConfig{Proxy: "legacy-1.example.com:443"},
			SSH:    config.SSHConfig{Login: "ec2-user"},
			Mappings: []config.Mapping{
				{Selector: map[string]string{"resource_group": "team-a"}, Scope: "/dgxc/team-a", InstallSuffix: "dgxc-team-a"},
			},
		}},
	}

	hosts := []HostData{
		{
			UUID:     "uuid-1",
			Hostname: "auto-host",
			Labels:   map[string]string{"resource_group": "team-a"},
			Recon: classify.ReconResult{
				Reachable:         true,
				OS:                "Linux",
				HasSystemd:        true,
				HasTeleportUpdate: true,
				ConfigPath:        "/etc/teleport.yaml",
				RootPath:          true,
				JoinMethod:        "token",
				Services:          []string{"ssh_service"},
				InstallKind:       classify.InstallSystemd,
			},
		},
		{
			UUID:     "uuid-2",
			Hostname: "prereq-host",
			Labels:   map[string]string{"resource_group": "team-a"},
			Recon: classify.ReconResult{
				Reachable:         true,
				OS:                "Linux",
				HasSystemd:        true,
				HasTeleportUpdate: true,
				ConfigPath:        "/etc/teleport.yaml",
				RootPath:          true,
				JoinMethod:        "iam",
				Services:          []string{"ssh_service"},
				InstallKind:       classify.InstallSystemd,
			},
		},
		{
			UUID:     "uuid-3",
			Hostname: "orphan-host",
			Labels:   map[string]string{"resource_group": "team-b"},
			Recon: classify.ReconResult{
				Reachable:         true,
				OS:                "Linux",
				HasSystemd:        true,
				HasTeleportUpdate: true,
				ConfigPath:        "/etc/teleport.yaml",
				RootPath:          true,
				JoinMethod:        "token",
				Services:          []string{"ssh_service"},
				InstallKind:       classify.InstallSystemd,
			},
		},
	}

	scopableMethods := []string{"token", "iam", "ec2", "gcp", "azure", "azure_devops", "oracle", "kubernetes", "bound_keypair"}
	scopableRoles := map[string]bool{"Node": true, "Kube": true, "App": false, "Db": false}
	rewriter := rewrite.NewStubRewriter()

	meta := report.ReportMeta{
		RunID:         "test-pipeline",
		ScopesEnabled: true,
		Source:        report.ClusterIdentity{Name: "legacy-1", Version: "17.3.1"},
		Target:        report.ClusterIdentity{Name: "scoped.example.com", Version: "17.3.1", ScopePinned: true},
		Scopable:      report.Capability{Roles: scopableRoles},
	}

	rpt, err := BuildPreflightReport(
		hosts,
		cfg.Migrations[0].Mappings,
		scopableMethods,
		scopableRoles,
		rewriter,
		cfg.Target.Proxy,
		meta,
	)
	require.NoError(t, err)
	require.NotNil(t, rpt)

	// Verify aggregate counts
	require.Equal(t, 3, rpt.Summary.Total)
	require.Equal(t, 1, rpt.Summary.ByVerdict[classify.VerdictAuto])
	require.Equal(t, 1, rpt.Summary.ByVerdict[classify.VerdictPrereq])
	require.Equal(t, 1, rpt.Summary.ByVerdict[classify.VerdictManual])

	// Verify orphan count
	require.Equal(t, 1, rpt.Summary.Orphans)

	// Verify host details
	require.Len(t, rpt.Hosts, 3)
	require.Equal(t, classify.VerdictAuto, rpt.Hosts[0].Verdict)
	require.Equal(t, classify.VerdictPrereq, rpt.Hosts[1].Verdict)
	require.Equal(t, classify.VerdictManual, rpt.Hosts[2].Verdict)

	// AUTO host should have config diff
	require.NotEmpty(t, rpt.Hosts[0].ConfigDiff)
	require.Equal(t, "stub", rpt.Hosts[0].RewriteMode)

	// Orphan should have no config diff
	require.Empty(t, rpt.Hosts[2].ConfigDiff)
}

func TestWriteReportFiles(t *testing.T) {
	rpt := &report.Report{
		RunID: "write-test",
		Summary: report.Summary{
			Total:     1,
			ByVerdict: map[classify.Verdict]int{classify.VerdictAuto: 1},
		},
		Hosts: []report.HostReport{
			{
				HostUUID: "uuid-1",
				Hostname: "test-host",
				Verdict:  classify.VerdictAuto,
				Status:   classify.StatusSatisfied,
			},
		},
	}

	outDir := filepath.Join(t.TempDir(), "report")
	err := WriteReportFiles(rpt, outDir)
	require.NoError(t, err)

	// Verify JSON file
	jsonPath := filepath.Join(outDir, "readiness.json")
	_, err = os.Stat(jsonPath)
	require.NoError(t, err)

	jsonData, err := os.ReadFile(jsonPath)
	require.NoError(t, err)
	require.Contains(t, string(jsonData), "write-test")

	// Verify HTML file
	htmlPath := filepath.Join(outDir, "readiness.html")
	_, err = os.Stat(htmlPath)
	require.NoError(t, err)

	htmlData, err := os.ReadFile(htmlPath)
	require.NoError(t, err)
	require.Contains(t, string(htmlData), "test-host")
}

func TestBuildPreflightReportConflict(t *testing.T) {
	// Test with overlapping mappings that cause a conflict
	mappings := []config.Mapping{
		{Selector: map[string]string{"env": "prod"}, Scope: "/scope-a", InstallSuffix: "scope-a"},
		{Selector: map[string]string{"env": "prod"}, Scope: "/scope-b", InstallSuffix: "scope-b"},
	}

	hosts := []HostData{
		{
			UUID:     "uuid-conflict",
			Hostname: "conflict-host",
			Labels:   map[string]string{"env": "prod"},
			Recon: classify.ReconResult{
				Reachable:         true,
				OS:                "Linux",
				HasSystemd:        true,
				HasTeleportUpdate: true,
				ConfigPath:        "/etc/teleport.yaml",
				RootPath:          true,
				JoinMethod:        "token",
				Services:          []string{"ssh_service"},
				InstallKind:       classify.InstallSystemd,
			},
		},
	}

	scopableMethods := []string{"token"}
	scopableRoles := map[string]bool{"Node": true}
	rewriter := rewrite.NewStubRewriter()

	meta := report.ReportMeta{
		RunID:         "conflict-test",
		ScopesEnabled: true,
	}

	rpt, err := BuildPreflightReport(hosts, mappings, scopableMethods, scopableRoles, rewriter, "proxy:443", meta)
	require.NoError(t, err)
	require.Equal(t, 1, rpt.Summary.Total)

	// Conflict results in Orphan=false, Matched=nil, so the mapping resolution
	// gives Conflict non-nil. The host gets "(conflict)" mapping string.
	require.Equal(t, "(conflict)", rpt.Hosts[0].Mapping)
}
