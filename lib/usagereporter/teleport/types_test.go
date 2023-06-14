package usagereporter

import (
	"strings"
	"testing"

	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/stretchr/testify/require"
)

func TestTermsOfServiceClickEventAnonymize(t *testing.T) {
	e := TermsOfServiceClickEvent{
		UserName: "username",
		Origin:   "https://teleport-proxy/server",
	}

	anonymizer, err := utils.NewHMACAnonymizer("test-key")
	require.NoError(t, err)

	anonymousEvent := e.Anonymize(anonymizer)
	result, ok := anonymousEvent.Event.(*prehogv1a.SubmitEventRequest_UiTermsOfServiceClickEvent)
	require.True(t, ok)

	require.NotEqual(t, e.UserName, result.UiTermsOfServiceClickEvent.UserName)
	require.NotEqual(t, e.Origin, result.UiTermsOfServiceClickEvent.Origin)

	// assert it doesn't contain the original host name
	require.False(t, strings.Contains(result.UiTermsOfServiceClickEvent.Origin, "teleport-proxy"))
}
