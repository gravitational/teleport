package service

import (
	"errors"
	"net/http"

	"github.com/gravitational/teleport/e/lib/netiq/client"
)

// pollError is a wrapper for errors that occur during the polling process.
// It wraps the original error and provides information about when the error occurred.
type pollError struct {
	err error
}

func (p *pollError) Error() string {
	return "polling netIQ: " + p.err.Error()
}

func (p *pollError) Unwrap() error {
	return p.err
}

// accessGraphPushError is a wrapper for errors that occur during the push process
// to the Access Graph service.
type accessGraphPushError struct {
	err error
}

func (p *accessGraphPushError) Error() string {
	return "netIQ access graph push: " + p.err.Error()
}

func (p *accessGraphPushError) Unwrap() error {
	return p.err
}

func isUnauthorized(err error) bool {
	var netIQErr *client.ErrorResponse
	if errors.As(err, &netIQErr) {
		return netIQErr.StatusCode == http.StatusUnauthorized ||
			netIQErr.StatusCode == http.StatusForbidden
	}
	return false
}

// NetIQHumanReadableError returns a human-readable description of a NetIQ error.
func NetIQHumanReadableError(err error) string {
	if err == nil {
		return ""
	}
	var pollErr *pollError
	if errors.As(err, &pollErr) {
		var netIQ *client.ErrorResponse
		if errors.As(err, &netIQ) {
			return handleNetIQError(netIQ)
		}

		return "Failed to access NetIQ. Please check that your NetIQ instance is reachable from the Teleport Auth Service and try again."
	}

	var pushErr *accessGraphPushError
	if errors.As(err, &pushErr) {
		return "Failed to push NetIQ resources to Access Graph. Please check the Access Graph configuration and try again."
	}

	return "Failed to access NetIQ. Please check the full response for more details."
}

// NetIQRawError returns the error message from a NetIQ error or the error message itself.
func NetIQRawError(err error) (str string) {
	const maxErrorLength = 200 * 1024 /* 200KB */

	defer func() {
		if len(str) > maxErrorLength {
			str = str[:maxErrorLength]
		}
	}()
	if err == nil {
		return ""
	}
	var netIQErr *client.ErrorResponse
	if errors.As(err, &netIQErr) {
		if netIQErr.Payload != nil {
			return netIQErr.Payload.Reason.Text
		}
		if netIQErr.Err != nil {
			return netIQErr.Err.Error()
		}
		return netIQErr.Error()
	}
	return err.Error()
}

func handleNetIQError(netIQErr *client.ErrorResponse) string {
	switch netIQErr.StatusCode {
	case http.StatusUnauthorized:
		return "Failed to authenticate with NetIQ. Please check your access token and try again."
	case http.StatusForbidden:
		return "Failed to access NetIQ resources. Please check your access permissions and try again."
	case http.StatusNotFound:
		return "Failed to access NetIQ resources. Please check your NetIQ URL and try again."
	case http.StatusServiceUnavailable, http.StatusInternalServerError, http.StatusBadGateway, http.StatusGatewayTimeout, http.StatusConflict:
		return "Failed to access NetIQ. Please check your NetIQ instance state and try again."
	case http.StatusTooManyRequests:
		return "The client was rate-limited and couldn't complete the requests. Please ensure the client is not being rate-limited and try again."
	case http.StatusRequestTimeout:
		return "The request to NetIQ timed out. Please check your network connection and try again."
	default:
		return "Failed to access NetIQ. Please check the full response for more details."
	}
}
