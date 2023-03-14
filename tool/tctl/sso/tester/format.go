// Copyright 2022 Gravitational, Inc
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

package tester

import (
	"encoding/json"
	"fmt"

	"github.com/ghodss/yaml"

	"github.com/gravitational/teleport/api/types"
)

func FormatString(description string, msg string) string {
	return fmt.Sprintf("%v:\n%v\n", description, msg)
}

func FormatYAML(description string, object interface{}) string {
	output, err := yaml.Marshal(object)
	if err != nil {
		return formatError(description, err)
	}
	return fmt.Sprintf("%v:\n%v", description, string(output))
}

func FormatJSON(description string, object interface{}) string {
	output, err := json.MarshalIndent(object, "", "    ")
	if err != nil {
		return formatError(description, err)
	}
	return fmt.Sprintf("%v:\n%v\n", description, string(output))
}

func formatUserDetails(description string, info *types.CreateUserParams) string {
	if info == nil {
		return ""
	}

	// Skip fields: connector_name, session_ttl
	info.ConnectorName = ""
	info.SessionTTL = 0

	output, err := yaml.Marshal(info)
	if err != nil {
		return formatError(description, err)
	}
	return fmt.Sprintf("%v:\n%v", description, Indent(string(output), 3))
}

func formatError(fieldDesc string, err error) string {
	return fmt.Sprintf("%v: error rendering field: %v\n", fieldDesc, err)
}
