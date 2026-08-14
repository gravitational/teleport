package athena

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestConfigFromURI(t *testing.T) {
	t.Run("valid uri", func(t *testing.T) {
		config, err := ConfigFromURI("athena://database.table?accessMonitoringResultsS3=s3://foo/bar")
		require.NoError(t, err)
		require.Equal(t, "s3://foo/bar", config.ReportResults)
		require.Equal(t, "database", config.Database)
		require.Equal(t, "table", config.Table)
	})

	t.Run("invalid database table param", func(t *testing.T) {
		_, err := ConfigFromURI("athena://database-table")
		require.True(t, trace.IsBadParameter(err))
	})

	t.Run("invalid schema", func(t *testing.T) {
		_, err := ConfigFromURI("postgres://localhost:5432")
		require.True(t, trace.IsBadParameter(err))
	})
	t.Run("region param", func(t *testing.T) {
		config, err := ConfigFromURI("athena://database.table?accessMonitoringResultsS3=s3://foo/bar&region=us-west-2")
		require.NoError(t, err)
		require.Equal(t, "us-west-2", config.Region)
	})
}
