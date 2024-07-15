/*
Copyright 2015-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package provider

import (
	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	log "github.com/sirupsen/logrus"
)

// diagFromWrappedErr wraps error with additional information
func diagFromWrappedErr(summary string, err error, kind string) diag.Diagnostic {
	var notFoundErr = "Terraform user has no rights to perform this action. Check that Terraform user role has " +
		"['list','create','read','update','delete'] verbs for '" + kind + "' resource."

	var accessDeniedErr = "Terraform user is missing on the Teleport side. Check that your auth credentials (certs) " +
		"specified in provider configuration belong to existing user and are not expired."

	if trace.IsNotFound(err) {
		return diagFromErr(summary, trace.WrapWithMessage(err, notFoundErr))
	}

	if trace.IsAccessDenied(err) {
		return diagFromErr(summary, trace.WrapWithMessage(err, accessDeniedErr))
	}

	return diagFromErr(summary, trace.Wrap(err))
}

// diagFromErr converts error to diag.Diagnostics. If logging level is debug, provides trace.DebugReport instead of short text.
func diagFromErr(summary string, err error) diag.Diagnostic {
	if log.GetLevel() >= log.DebugLevel {
		return diag.NewErrorDiagnostic(err.Error(), trace.DebugReport(err))
	}

	return diag.NewErrorDiagnostic(summary, err.Error())
}
