// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package azidentity

import (
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/internal/diag"
	"github.com/Azure/azure-sdk-for-go/sdk/internal/log"
)

// LogCredential entries contain information about authentication.
// This includes information like the names of environment variables
// used when obtaining credentials and the type of credential used.
const LogCredential log.Classification = "Credential"

// log environment variables that can be used for credential types
func logEnvVars() {
	if !log.Should(LogCredential) {
		return
	}
	// Log available environment variables
	envVars := []string{}
	if envCheck := os.Getenv("AZURE_TENANT_ID"); len(envCheck) > 0 {
		envVars = append(envVars, "AZURE_TENANT_ID")
	}
	if envCheck := os.Getenv("AZURE_CLIENT_ID"); len(envCheck) > 0 {
		envVars = append(envVars, "AZURE_CLIENT_ID")
	}
	if envCheck := os.Getenv("AZURE_CLIENT_SECRET"); len(envCheck) > 0 {
		envVars = append(envVars, "AZURE_CLIENT_SECRET")
	}
	if envCheck := os.Getenv("AZURE_AUTHORITY_HOST"); len(envCheck) > 0 {
		envVars = append(envVars, "AZURE_AUTHORITY_HOST")
	}
	if envCheck := os.Getenv("AZURE_CLI_PATH"); len(envCheck) > 0 {
		envVars = append(envVars, "AZURE_CLI_PATH")
	}
	if len(envVars) > 0 {
		log.Writef(LogCredential, "Azure Identity => Found the following environment variables:\n\t%s", strings.Join(envVars, ", "))
	}
}

func logGetTokenSuccess(cred azcore.TokenCredential, opts policy.TokenRequestOptions) {
	if !log.Should(LogCredential) {
		return
	}
	msg := fmt.Sprintf("Azure Identity => GetToken() result for %T: SUCCESS\n", cred)
	msg += fmt.Sprintf("\tCredential Scopes: [%s]", strings.Join(opts.Scopes, ", "))
	log.Write(LogCredential, msg)
}

func logCredentialError(credName string, err error) {
	log.Writef(LogCredential, "Azure Identity => ERROR in %s: %s", credName, err.Error())
}

func logMSIEnv(msi msiType) {
	if !log.Should(LogCredential) {
		return
	}
	var msg string
	switch msi {
	case msiTypeIMDS:
		msg = "Azure Identity => Managed Identity environment: IMDS"
	case msiTypeAppServiceV20170901, msiTypeCloudShell, msiTypeAppServiceV20190801:
		msg = "Azure Identity => Managed Identity environment: MSI_ENDPOINT"
	case msiTypeUnavailable:
		msg = "Azure Identity => Managed Identity environment: Unavailable"
	default:
		msg = "Azure Identity => Managed Identity environment: Unknown"
	}
	log.Write(LogCredential, msg)
}

func addGetTokenFailureLogs(credName string, err error, includeStack bool) {
	if !log.Should(LogCredential) {
		return
	}
	stack := ""
	if includeStack {
		// skip the stack trace frames and ourself
		stack = "\n" + diag.StackTrace(3, 32)
	}
	log.Writef(LogCredential, "Azure Identity => ERROR in GetToken() call for %s: %s%s", credName, err.Error(), stack)
}
