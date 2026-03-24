package sdk

import (
	"errors"

	identitystoretypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/gravitational/trace"

	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
)

// traceError converts an AWS sdk v2 error type to a [trace] error value.
//
//	AWS Error Type             Trace Error Type
//	----------------------     --------------------------
//	AccessDeniedException      BadParameterError
//	ResourceNotFoundException  NotFoundError
//
// All other errors are translated as per [libcloudaws.ConvertRequestFailureError].
func traceError(err error) error {
	if err == nil {
		return nil
	}

	var ssoAdminErr *ssoadmintypes.AccessDeniedException
	if errors.As(err, &ssoAdminErr) {
		return trace.BadParameter("Invalid credential. %s", err.Error())
	}

	var idStoreErr *identitystoretypes.AccessDeniedException
	if errors.As(err, &idStoreErr) {
		return trace.BadParameter("Invalid credential. %s", err.Error())
	}

	var ssoAdminResourceNotFound *ssoadmintypes.ResourceNotFoundException
	if errors.As(err, &ssoAdminResourceNotFound) {
		return trace.NotFound("%s", err.Error())
	}

	var idStoreResourceNotFound *identitystoretypes.ResourceNotFoundException
	if errors.As(err, &idStoreResourceNotFound) {
		return trace.NotFound("%s", err.Error())
	}

	return libcloudaws.ConvertRequestFailureError(err)
}
