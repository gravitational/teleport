package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateVersionChannel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc               string
		config             *Config
		shouldErr          bool
		expectedFieldValue string
	}{
		{
			desc:      "Error if versionChannel field is empty",
			config:    &Config{},
			shouldErr: true,
		},
		{
			desc: "Version channel updated to v<majorVersion> for semver",
			config: &Config{
				versionChannel: "1.2.3",
			},
			expectedFieldValue: "v1",
		},
		{
			desc: "Version channel updated to v<majorVersion> for coerced v<semver>",
			config: &Config{
				versionChannel: "v1.2.3",
			},
			expectedFieldValue: "v1",
		},
		{
			desc: "Version channel updated to v<majorVersion> for coerced dev tag semver",
			config: &Config{
				versionChannel: "v1.2.3-dev.tag+abcd",
			},
			expectedFieldValue: "v1",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.config.validateVersionChannel()
			if test.shouldErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectedFieldValue, test.config.versionChannel)
			}
		})
	}
}
