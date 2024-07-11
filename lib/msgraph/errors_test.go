package msgraph

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const msGraphErrorPayload = `{
  "error": {
    "code": "Error_BadRequest",
    "message": "Uploaded fragment overlaps with existing data.",
    "innerError": {
      "code": "invalidRange",
      "request-id": "request-id",
      "date": "date-time"
    }
  }
}`

func TestUnmarshalGraphError(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		graphError, err := readError(strings.NewReader(msGraphErrorPayload))
		require.NoError(t, err)
		require.NotNil(t, graphError)
		expected := &GraphError{
			Code:    "Error_BadRequest",
			Message: "Uploaded fragment overlaps with existing data.",
			InnerError: &GraphError{
				Code: "invalidRange",
			},
		}
		require.Equal(t, expected, graphError)
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := readError(strings.NewReader("invalid json"))
		require.Error(t, err)
	})

	t.Run("empty", func(t *testing.T) {
		graphError, err := readError(strings.NewReader("{}"))
		require.NoError(t, err)
		require.Nil(t, graphError)
	})
}
