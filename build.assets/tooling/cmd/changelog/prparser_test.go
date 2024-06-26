package main

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
)

//go:embed testdata/expectedcl.md
var expectedcl string

//go:embed testdata/listedprs.json
var listedprs string

func TestToChangelog(t *testing.T) {
	got, err := toChangelog(listedprs)
	assert.NoError(t, err)
	assert.Equal(t, expectedcl, got)
}
