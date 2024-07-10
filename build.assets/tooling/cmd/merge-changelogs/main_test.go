package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_merge(t *testing.T) {
	cases := []struct {
		description      string
		changelog1       string
		changelog2       string
		expected         string
		expectedMessages []string
	}{
		{
			description: "Two complete version headings",
			changelog1: `## 1.2.3

### Database access

We added the ability to connect to a new database.

`,

			changelog2: `## 4.5.6

### Application access

We added the ability to connect to more web applications.
`,
			expected: `## 1.2.3

### Database access

We added the ability to connect to a new database.

## 4.5.6

### Application access

We added the ability to connect to more web applications.
`,
		},
	}

	// TODO: clashing versions (combine H3s but add a warning message)
	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			actual, msg := merge(strings.NewReader(c.changelog1), strings.NewReader(c.changelog2))
			assert.Equal(t, c.expected, actual)
			assert.Equal(t, c.expectedMessages, msg)
		})
	}
}
