package entraid

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"
	"github.com/microsoftgraph/msgraph-sdk-go/models/odataerrors"

	"github.com/gravitational/teleport/api/types"
)

func getErrorDetails(err error) (types.PluginStatusCode, string) {
	if err == nil {
		return types.PluginStatusCode_RUNNING, ""
	}

	var messages []string
	var traceErr *trace.TraceErr
	if errors.As(err, &traceErr) {
		messages = append(messages, traceErr.Messages...)
	}
	code, entraMsg := getEntraErrorDetails(err)
	if entraMsg != "" {
		messages = append(messages, entraMsg)
	}
	return code, strings.Join(messages, ": ")
}

func getEntraErrorDetails(err error) (types.PluginStatusCode, string) {
	var oDataErr *odataerrors.ODataError
	if errors.As(err, &oDataErr) {
		var messages []string
		if mainErr := oDataErr.GetErrorEscaped(); mainErr != nil {
			if mainErr.GetCode() != nil {
				messages = append(messages, strings.TrimPrefix(*mainErr.GetCode(), "Request_"))
			}
		}
		mainErr := oDataErr.GetErrorEscaped()
		if mainErr != nil && mainErr.GetMessage() != nil {
			messages = append(messages, *mainErr.GetMessage())
		}

		// Fall back on generic error
		if len(messages) == 0 {
			messages = []string{oDataErr.Error()}
		}

		return types.PluginStatusCode_OTHER_ERROR, strings.Join(messages, ": ")
	}

	var azIdentityErr *azidentity.AuthenticationFailedError
	if errors.As(err, &azIdentityErr) {
		messages := []string{"Authentication to Azure failed"}
		var response struct {
			// Error is the OAuth error code. OAuth error codes are more broad than AADSTS error codes.
			// See https://learn.microsoft.com/en-us/entra/identity-platform/reference-error-codes#handling-error-codes-in-your-application
			Error string `json:"error"`
			// ErrorDescription is a short human-readable explanation of the error.
			// It is prefixed with an AADSTS error code, e.g. "AADSTS7000215: Invalid client secret provided <...>"
			// It is suffixed with a "Trace ID: ... Correlation ID: <...> Timestamp: <...>", which we strip.
			ErrorDescription string `json:"error_description"`
		}
		body := azIdentityErr.RawResponse.Body
		defer body.Close()
		if err := json.NewDecoder(body).Decode(&response); err == nil {
			if response.Error != "" {
				messages[0] += " (" + response.Error + ")"
			}
			if description := response.ErrorDescription; description != "" {
				extraDataIdx := strings.Index(description, " Trace ID:")
				if extraDataIdx != -1 {
					description = description[:extraDataIdx]
				}
				messages = append(messages, description)
			}
		}
		return types.PluginStatusCode_UNAUTHORIZED, strings.Join(messages, ": ")
	}

	return types.PluginStatusCode_OTHER_ERROR, trace.Unwrap(err).Error()
}
