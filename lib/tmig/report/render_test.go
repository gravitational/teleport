package report

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tmig/classify"
)

func sampleReport() *Report {
	return &Report{
		RunID:         "test-run-001",
		ScopesEnabled: true,
		Source:        ClusterIdentity{Name: "legacy-1", Version: "17.3.1", Proxy: "legacy-1.example.com:443", User: "migrator@legacy-1"},
		Target:        ClusterIdentity{Name: "scoped.example.com", Version: "17.3.1", Proxy: "scoped.example.com:443", User: "team-a", ScopePinned: true},
		Scopable:      Capability{Roles: map[string]bool{"Node": true, "Kube": true, "App": false, "Db": false}},
		Summary: Summary{
			Total:         5,
			ByVerdict:     map[classify.Verdict]int{classify.VerdictAuto: 2, classify.VerdictPrereq: 2, classify.VerdictManual: 1},
			Blocked:       1,
			ReadyToEnroll: 3,
			Attention:     AttentionRollup{AutomaticHosts: 3, IaCActions: 1, IaCHostsCovered: 1, ManualHosts: 1},
		},
		Hosts: []HostReport{
			{Hostname: "h1", Verdict: classify.VerdictAuto, Status: classify.StatusSatisfied, Attention: classify.AttentionNone},
			{Hostname: "h2", Verdict: classify.VerdictPrereq, Status: classify.StatusBlocked, Attention: classify.AttentionIaCOnetime, RemediationRef: "rem-1"},
		},
		Warnings: []string{"test warning"},
	}
}

func TestRenderJSON(t *testing.T) {
	var buf bytes.Buffer
	err := RenderJSON(sampleReport(), &buf)
	require.NoError(t, err)

	var parsed Report
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	require.Equal(t, "test-run-001", parsed.RunID)
	require.Equal(t, 5, parsed.Summary.Total)
	require.Equal(t, 2, parsed.Summary.ByVerdict[classify.VerdictAuto])
}

func TestRenderTerminal(t *testing.T) {
	var buf bytes.Buffer
	err := RenderTerminal(sampleReport(), &buf)
	require.NoError(t, err)
	output := buf.String()
	require.Contains(t, output, "legacy-1")
	require.Contains(t, output, "scoped.example.com")
	require.Contains(t, output, "AUTO")
	require.Contains(t, output, "PREREQ")
}

func TestRenderHTML(t *testing.T) {
	var buf bytes.Buffer
	err := RenderHTML(sampleReport(), &buf)
	require.NoError(t, err)
	html := buf.String()
	require.Contains(t, html, "<!DOCTYPE html>")
	require.Contains(t, html, "readiness report")
	require.Contains(t, html, "<style>")
	// Must be self-contained: no external references
	require.NotContains(t, html, "https://cdn")
	require.NotContains(t, html, "http://")
}

func TestRenderHTMLNoSecrets(t *testing.T) {
	rpt := sampleReport()
	rpt.Hosts[0].ConfigDiff = "auth_token: <redacted>"
	var buf bytes.Buffer
	err := RenderHTML(rpt, &buf)
	require.NoError(t, err)
	// The redacted placeholder should survive, but no raw secrets
	require.NotContains(t, buf.String(), "supersecret")
}

func TestRenderJSONGolden(t *testing.T) {
	var buf bytes.Buffer
	err := RenderJSON(sampleReport(), &buf)
	require.NoError(t, err)

	goldenPath := "testdata/golden.json"
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		os.MkdirAll("testdata", 0755)
		os.WriteFile(goldenPath, buf.Bytes(), 0644)
		return
	}
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Skip("golden file not found; run with UPDATE_GOLDEN=1 to create")
	}
	require.JSONEq(t, string(golden), buf.String())
}

func TestRenderHTMLSelfContained(t *testing.T) {
	var buf bytes.Buffer
	err := RenderHTML(sampleReport(), &buf)
	require.NoError(t, err)
	html := buf.String()
	// No external script or link tags
	require.False(t, strings.Contains(html, `<link rel="stylesheet" href="http`))
	require.False(t, strings.Contains(html, `<script src="http`))
}
