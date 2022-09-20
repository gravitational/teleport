// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package azidentity

import (
	"context"
	"errors"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
)

// ChainedTokenCredential provides a TokenCredential implementation that chains multiple TokenCredential sources to be tried in order
// and returns the token from the first successful call to GetToken().
type ChainedTokenCredential struct {
	sources []azcore.TokenCredential
}

// NewChainedTokenCredential creates an instance of ChainedTokenCredential with the specified TokenCredential sources.
func NewChainedTokenCredential(sources ...azcore.TokenCredential) (*ChainedTokenCredential, error) {
	if len(sources) == 0 {
		credErr := &CredentialUnavailableError{credentialType: "Chained Token Credential", message: "Length of sources cannot be 0"}
		logCredentialError(credErr.credentialType, credErr)
		return nil, credErr
	}
	for _, source := range sources {
		if source == nil { // cannot have a nil credential in the chain or else the application will panic when GetToken() is called on nil
			credErr := &CredentialUnavailableError{credentialType: "Chained Token Credential", message: "Sources cannot contain a nil TokenCredential"}
			logCredentialError(credErr.credentialType, credErr)
			return nil, credErr
		}
	}
	return &ChainedTokenCredential{sources: sources}, nil
}

// GetToken sequentially calls TokenCredential.GetToken on all the specified sources, returning the token from the first successful call to GetToken().
func (c *ChainedTokenCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (token *azcore.AccessToken, err error) {
	var errList []*CredentialUnavailableError
	// loop through all of the credentials provided in sources
	for _, cred := range c.sources {
		// make a GetToken request for the current credential in the loop
		token, err = cred.GetToken(ctx, opts)
		// check if we received a CredentialUnavailableError
		var credErr *CredentialUnavailableError
		if errors.As(err, &credErr) {
			// if we did receive a CredentialUnavailableError then we append it to our error slice and continue looping for a good credential
			errList = append(errList, credErr)
		} else if err != nil {
			// if we receive some other type of error then we must stop looping and process the error accordingly
			var authenticationFailed *AuthenticationFailedError
			if errors.As(err, &authenticationFailed) {
				// if the error is an AuthenticationFailedError we return the error related to the invalid credential and append all of the other error messages received prior to this point
				authErr := &AuthenticationFailedError{msg: "Received an AuthenticationFailedError, there is an invalid credential in the chain. " + createChainedErrorMessage(errList), inner: err}
				return nil, authErr
			}
			// if we receive some other error type this is unexpected and we simple return the unexpected error
			return nil, err
		} else {
			logGetTokenSuccess(c, opts)
			// if we did not receive an error then we return the token
			return token, nil
		}
	}
	// if we reach this point it means that all of the credentials in the chain returned CredentialUnavailableErrors
	credErr := &CredentialUnavailableError{credentialType: "Chained Token Credential", message: createChainedErrorMessage(errList)}
	// skip adding the stack trace here as it was already logged by other calls to GetToken()
	addGetTokenFailureLogs("Chained Token Credential", credErr, false)
	return nil, credErr
}

// NewAuthenticationPolicy implements the azcore.Credential interface on ChainedTokenCredential and sets the bearer token
func (c *ChainedTokenCredential) NewAuthenticationPolicy(options runtime.AuthenticationOptions) policy.Policy {
	return newBearerTokenPolicy(c, options)
}

// helper function used to chain the error messages of the CredentialUnavailableError slice
func createChainedErrorMessage(errList []*CredentialUnavailableError) string {
	msg := ""
	for _, err := range errList {
		msg += err.Error()
	}

	return msg
}

var _ azcore.TokenCredential = (*ChainedTokenCredential)(nil)
