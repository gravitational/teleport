// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package kubeconfig manages teleport entries in a local kubeconfig file.
package kubeconfig

import (
	"bytes"
	"text/template"

	"github.com/gravitational/trace"
)

const (
	// supportedFunctionsMsg is a message that lists all supported template
	// vars.
	supportedFunctionsMsg = "Supported template functions:\n" +
		"  - `{{ .KubeName }}` - the name of the Kubernetes cluster\n" +
		"  - `{{ .ClusterName }}` - the name of the Teleport cluster\n"
)

// CheckContextOverrideTemplate tests if the given template is valid and can
// be used to generate different context names for different clusters.
func CheckContextOverrideTemplate(temp string) error {
	if temp == "" {
		return nil
	}
	tmpl, err := parseContextOverrideTemplate(temp)
	if err != nil {
		return trace.Wrap(parseContextOverrideError(err))
	}
	val1, err1 := executeKubeContextTemplate(tmpl, "cluster", "kube1")
	val2, err2 := executeKubeContextTemplate(tmpl, "cluster", "kube2")
	if err1 != nil || err2 != nil {
		return trace.Wrap(parseContextOverrideError(nil))
	}

	if val1 != val2 {
		return nil
	}

	return trace.BadParameter(
		"using the same context override template for different clusters is not allowed.\n" +
			"Please ensure the template syntax includes {{ .KubeName }} and try again.\n" +
			supportedFunctionsMsg,
	)
}

// parseContextOverrideTemplate parses the given template and returns a
// template object that can be used to generate different context names for
// different clusters.
// Otherwise, it returns an error.
func parseContextOverrideTemplate(temp string) (*template.Template, error) {
	if temp == "" {
		return nil, nil
	}
	tmpl, err := template.New("context_override").Parse(temp)
	if err != nil {
		return nil, trace.Wrap(parseContextOverrideError(err))
	}
	return tmpl, nil
}

// parseContextOverrideError returns a formatted error message for the given
// error.
func parseContextOverrideError(err error) error {
	msg := "failed to parse context override template.\n" +
		"Please check the template syntax and try again.\n" +
		supportedFunctionsMsg
	if err == nil {
		return trace.BadParameter(msg)
	}
	return trace.BadParameter(
		msg+
			"Error: %v", err,
	)
}

// executeKubeContextTemplate executes the given template and returns the
// generated context name.
func executeKubeContextTemplate(tmpl *template.Template, clusterName, kubeName string) (string, error) {
	contextEntry := struct {
		ClusterName string
		KubeName    string
	}{
		ClusterName: clusterName,
		KubeName:    kubeName,
	}
	var buf bytes.Buffer
	err := tmpl.Execute(&buf, contextEntry)
	return buf.String(), trace.Wrap(err)
}
