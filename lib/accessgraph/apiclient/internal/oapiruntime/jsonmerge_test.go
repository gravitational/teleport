// Copied verbatim from github.com/oapi-codegen/runtime v1.4.0
// (jsonmerge_test.go), with only the package declaration adjusted for
// Teleport's module layout. Licensed under the Apache License, Version 2.0;
// see LICENSE in this directory.

package oapiruntime

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJSONMerge(t *testing.T) {
	t.Run("when object", func(t *testing.T) {
		t.Run("Merges properties defined in both objects", func(t *testing.T) {
			data := `{"foo": 1}`
			patch := `{"foo": null}`
			expected := `{"foo":null}`

			actual, err := JSONMerge([]byte(data), []byte(patch))
			assert.NoError(t, err)
			assert.Equal(t, expected, string(actual))
		})

		t.Run("Sets property defined in only src object", func(t *testing.T) {
			data := `{}`
			patch := `{"source":"merge-me"}`
			expected := `{"source":"merge-me"}`

			actual, err := JSONMerge([]byte(data), []byte(patch))
			assert.NoError(t, err)
			assert.Equal(t, expected, string(actual))
		})

		t.Run("Handles child objects", func(t *testing.T) {
			data := `{"channel":{"status":"valid"}}`
			patch := `{"channel":{"id":1}}`
			expected := `{"channel":{"id":1,"status":"valid"}}`

			actual, err := JSONMerge([]byte(data), []byte(patch))
			assert.NoError(t, err)
			assert.Equal(t, expected, string(actual))
		})

		t.Run("Handles empty objects", func(t *testing.T) {
			data := `{}`
			patch := `{}`
			expected := `{}`

			actual, err := JSONMerge([]byte(data), []byte(patch))
			assert.NoError(t, err)
			assert.Equal(t, expected, string(actual))
		})

		t.Run("Handles nil data", func(t *testing.T) {
			patch := `{"foo":"bar"}`
			expected := `{"foo":"bar"}`

			actual, err := JSONMerge(nil, []byte(patch))
			assert.NoError(t, err)
			assert.Equal(t, expected, string(actual))
		})

		t.Run("Handles nil patch", func(t *testing.T) {
			data := `{"foo":"bar"}`
			expected := `{"foo":"bar"}`

			actual, err := JSONMerge([]byte(data), nil)
			assert.NoError(t, err)
			assert.Equal(t, expected, string(actual))
		})
	})
	t.Run("when array", func(t *testing.T) {
		t.Run("it does not merge", func(t *testing.T) {
			data := `[{"foo": 1}]`
			patch := `[{"foo": null}]`
			expected := `[{"foo":1}]`

			actual, err := JSONMerge([]byte(data), []byte(patch))
			assert.NoError(t, err)
			assert.Equal(t, expected, string(actual))
		})
	})
}
