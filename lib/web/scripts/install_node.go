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

package scripts

import (
	_ "embed"
	"net/http"
	"sort"
	"strings"
	"text/template"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/httplib"
)

// SetScriptHeaders sets response headers to plain text.
func SetScriptHeaders(h http.Header) {
	httplib.SetNoCacheHeaders(h)
	httplib.SetNoSniff(h)
	h.Set("Content-Type", "text/plain")
}

// ErrorBashScript is used to display friendly error message when
// there is an error prepping the actual script.
var ErrorBashScript = []byte(`
#!/bin/sh
echo -e "An error has occurred. \nThe token may be expired or invalid. \nPlease check log for further details."
exit 1
`)

// InstallNodeBashScript is the script that will run on user's machine
// to install teleport and join a teleport cluster.
//
//go:embed node-join/install.sh
var installNodeBashScript string

var InstallNodeBashScript = template.Must(template.New("nodejoin").Parse(installNodeBashScript))

// MarshalLabelsYAML returns a list of strings, each one containing a label key/value pair.
// This is used to create yaml sections within the join scripts.
func MarshalLabelsYAML(resourceMatcherLabels types.Labels) ([]string, error) {
	if len(resourceMatcherLabels) == 0 {
		return []string{"{}"}, nil
	}

	ret := []string{}

	// Consistently iterate over fields
	labelKeys := make([]string, 0, len(resourceMatcherLabels))
	for k := range resourceMatcherLabels {
		labelKeys = append(labelKeys, k)
	}

	sort.Strings(labelKeys)

	for _, labelName := range labelKeys {
		labelValue := resourceMatcherLabels[labelName]
		bs, err := yaml.Marshal(map[string]utils.Strings{labelName: labelValue})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		ret = append(ret, strings.TrimSpace(string(bs)))
	}

	return ret, nil
}
