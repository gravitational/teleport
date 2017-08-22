/*
Copyright 2015 Gravitational, Inc.

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
package configure

import (
	"gopkg.in/yaml.v2"

	"github.com/gravitational/trace"
)

// ParseYAML parses yaml-encoded byte string into the struct
// passed to the function.
// EnableTemplating() argument allows to treat configuration file as a template
// for example, it will support {{env "VAR"}} - that will substitute
// environment variable "VAR" and pass it to YAML file parser
func ParseYAML(data []byte, cfg interface{}, funcArgs ...ParseOption) error {
	var opts parseOptions
	for _, fn := range funcArgs {
		fn(&opts)
	}
	var err error
	if opts.templating {
		if data, err = renderTemplate(data); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

type parseOptions struct {
	// templating turns on templating mode when
	// parsing yaml file
	templating bool
}

// ParseOption is a functional argument type
type ParseOption func(p *parseOptions)

// EnableTemplating allows to treat configuration file as a template
// for example, it will support {{env "VAR"}} - that will substitute
// environment variable "VAR" and pass it to YAML file parser
func EnableTemplating() ParseOption {
	return func(p *parseOptions) {
		p.templating = true
	}
}
