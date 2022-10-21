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
	"bytes"
	_ "embed"
	"net/http"
	"text/template"

	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/trace"
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

// DBServiceConfig is partial configuration of the Teleport config file.
// It only contains the top level `db_service` field.
type DBServiceConfig struct {
	DBService DBService `yaml:"db_service,omitempty"`
}

// DBService contains the configurable fields in the context of install node script generation.
type DBService struct {
	Enabled          string            `yaml:"enabled"`
	ResourceMatchers []ResourceMatcher `yaml:"resources"`
}

// ResourceMatcher has a set of labels used to match the resources to be monitored by the database agent.
type ResourceMatcher struct {
	// Labels match resource labels.
	Labels map[string]utils.Strings `yaml:"labels"`
}

// MarshalDBServiceConfigSection returns a yaml marshaled representation of `db_service` config property.
// It adds the resourceMatcherLabels as `db_service.resources.0.labels`.
// This should only be used to generate the configuration part of the script.
// New properties should be added as necessary.
func MarshalDBServiceConfigSection(resourceMatcherLabels types.Labels) (string, error) {
	var dbServiceEncoded bytes.Buffer
	dbServiceEncoder := yaml.NewEncoder(&dbServiceEncoded)
	dbServiceEncoder.SetIndent(2)

	dbService := DBServiceConfig{
		DBService: DBService{
			Enabled: "yes",
			ResourceMatchers: []ResourceMatcher{
				{
					Labels: resourceMatcherLabels,
				},
			},
		},
	}

	if err := dbServiceEncoder.Encode(dbService); err != nil {
		return "", trace.Wrap(err)
	}

	if err := dbServiceEncoder.Close(); err != nil {
		return "", trace.Wrap(err)
	}

	return dbServiceEncoded.String(), nil
}
