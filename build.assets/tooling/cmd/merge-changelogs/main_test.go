package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_merge(t *testing.T) {
	cases := []struct {
		description string
		changelog1  string
		changelog2  string
		expected    string
	}{}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			actual, err := merge(strings.NewReader(c.changelog1), strings.NewReader(c.changelog2))
			assert.NoError(t, err)
			assert.Equal(t, c.expected, actual)
		})
	}
}
