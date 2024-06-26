package main

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
)

//go:embed testdata/expectedCL.md
var expectedCL string

//go:embed testdata/listedPRs.json
var listedprs string

//go:embed testdata/entExpectedCL.md
var entExpectedCL string

//go:embed testdata/entListedPRs.json
var entListedPRs string

func TestToChangelog(t *testing.T) {
	gen := &changelogGenerator{
		isEnt: false,
	}
	got, err := gen.toChangelog(listedprs)
	assert.NoError(t, err)
	assert.Equal(t, expectedCL, got)
}

func TestToChangelogEnterprise(t *testing.T) {
	gen := &changelogGenerator{
		isEnt: true,
	}
	got, err := gen.toChangelog(entListedPRs)
	assert.NoError(t, err)
	assert.Equal(t, entExpectedCL, got)
}
