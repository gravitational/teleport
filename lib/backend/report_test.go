package backend

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildKeyLabel(t *testing.T) {
	sensitivePrefixes := []string{"secret"}
	testCases := []struct {
		input     string
		scrambled string
	}{
		{"/secret/", "/secret/"},
		{"/secret/a", "/secret/a"},
		{"/secret/ab", "/secret/*b"},
		{"/secret/1b4d2844-f0e3-4255-94db-bf0e91883205", "/secret/***************************e91883205"},
		{"/secret/secret-role", "/secret/********ole"},
		{"/secret/graviton-leaf", "/secret/*********leaf"},
		{"/secret/graviton-leaf/sub1/sub2", "/secret/*********leaf"},
		{"/public/graviton-leaf", "/public/graviton-leaf"},
		{"/public/graviton-leaf/sub1/sub2", "/public/graviton-leaf"},
		{".data/secret/graviton-leaf", ".data/secret/graviton-leaf"},
	}
	for _, tc := range testCases {
		assert.Equal(t, tc.scrambled, buildKeyLabel([]byte(tc.input), sensitivePrefixes))
	}
}
