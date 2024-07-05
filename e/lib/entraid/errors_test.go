package entraid

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"
	"github.com/microsoftgraph/msgraph-sdk-go/models/odataerrors"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestGetErrorDetailsRaw(t *testing.T) {
	t.Parallel()

	code, msg := getErrorDetails(nil)
	require.Equal(t, types.PluginStatusCode_RUNNING, code)
	require.Empty(t, msg)

	err := errors.New("something bad happened")
	code, msg = getErrorDetails(err)
	require.Equal(t, types.PluginStatusCode_OTHER_ERROR, code)
	require.Equal(t, "something bad happened", msg)

	err = trace.Wrap(err, "failed to fetch foo")
	code, msg = getErrorDetails(err)
	require.Equal(t, types.PluginStatusCode_OTHER_ERROR, code)
	require.Equal(t, "failed to fetch foo: something bad happened", msg)
}

func TestGetErrorDetailsOData(t *testing.T) {
	t.Parallel()

	oDataErr := odataerrors.NewODataError()
	mainErr := odataerrors.NewMainError()
	errCode := "Request_ResourceNotFound"
	mainErr.SetCode(&errCode)
	message := "Resource 'a716e6e4-0aca-474a-8ae1-d46ae1de209e' does not exist or one of its queried reference-property objects are not present."
	mainErr.SetMessage(&message)
	oDataErr.SetErrorEscaped(mainErr)

	err := trace.Wrap(oDataErr, "failed to fetch foo")
	code, msg := getErrorDetails(err)
	require.Equal(t, types.PluginStatusCode_OTHER_ERROR, code)
	require.Equal(t, "failed to fetch foo: ResourceNotFound: Resource 'a716e6e4-0aca-474a-8ae1-d46ae1de209e' does not exist or one of its queried reference-property objects are not present.", msg)
}

func TestGetErrorDetailsAzIdentity(t *testing.T) {
	t.Parallel()

	buffer := bytes.NewBufferString(`{"error":"invalid_client","error_description":"AADSTS700211: No matching federated identity record found for presented assertion issuer 'https://example.com'. Please check your federated identity credential Subject, Audience and Issuer against the presented assertion. https://docs.microsoft.com/en-us/azure/active-directory/develop/workload-identity-federation Trace ID: 80049d39-82c0-4144-9235-cc0744d7e900 Correlation ID: bb80fb88-ecea-4292-8785-08ea8c957d07 Timestamp: 2024-07-04 13:48:17Z","error_codes":[700211],"timestamp":"2024-07-04 13:48:17Z","trace_id":"80049d39-82c0-4144-9235-cc0744d7e900","correlation_id":"bb80fb88-ecea-4292-8785-08ea8c957d07"}`)
	response := &http.Response{
		Body: io.NopCloser(buffer),
	}
	azErr := &azidentity.AuthenticationFailedError{
		RawResponse: response,
	}

	err := trace.Wrap(azErr, "failed to fetch foo")
	code, msg := getErrorDetails(err)
	require.Equal(t, types.PluginStatusCode_UNAUTHORIZED, code)
	require.Equal(t, "failed to fetch foo: Authentication to Azure failed (invalid_client): AADSTS700211: No matching federated identity record found for presented assertion issuer 'https://example.com'. Please check your federated identity credential Subject, Audience and Issuer against the presented assertion. https://docs.microsoft.com/en-us/azure/active-directory/develop/workload-identity-federation", msg)
}
