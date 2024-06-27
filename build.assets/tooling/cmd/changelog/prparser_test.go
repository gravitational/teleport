package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToChangelog(t *testing.T) {
	prsText, err := os.ReadFile(filepath.Join("testdata", "listed-prs.json"))
	require.NoError(t, err)
	expectedCL, err := os.ReadFile(filepath.Join("testdata", "expected-cl.md"))
	require.NoError(t, err)

	gen := &changelogGenerator{
		isEnt: false,
	}
	got, err := gen.toChangelog(string(prsText))
	assert.NoError(t, err)
	assert.Equal(t, string(expectedCL), got)
}

func TestToChangelogEnterprise(t *testing.T) {
	prsText, err := os.ReadFile(filepath.Join("testdata", "ent-listed-prs.json"))
	require.NoError(t, err)
	expectedCL, err := os.ReadFile(filepath.Join("testdata", "ent-expected-cl.md"))
	require.NoError(t, err)
	gen := &changelogGenerator{
		isEnt: true,
	}
	got, err := gen.toChangelog(string(prsText))
	require.NoError(t, err)
	assert.Equal(t, string(expectedCL), got)
}
