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
package service

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"text/template"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/configure/cstrings"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
)

func renderTemplate(data []byte) ([]byte, error) {
	t, err := template.New("tpl").Parse(string(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	buf := &bytes.Buffer{}
	c, err := newCtx()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := t.Execute(buf, c); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func newCtx() (*ctx, error) {
	values := os.Environ()
	c := &ctx{
		env: make(map[string]string, len(values)),
	}
	for _, v := range values {
		vals := strings.SplitN(v, "=", 2)
		if len(vals) != 2 {
			return nil, trace.Errorf("failed to parse variable: '%v'", v)
		}
		c.env[vals[0]] = vals[1]
	}
	return c, nil
}

type ctx struct {
	env map[string]string
}

func (c *ctx) File(path string) (string, error) {
	o, err := ioutil.ReadFile(path)
	if err != nil {
		return "", trace.Wrap(err, fmt.Sprintf("reading file: %v", path))
	}
	return string(o), nil
}

func (c *ctx) Env(key string) (string, error) {
	v, ok := c.env[key]
	if !ok {
		return "", trace.Errorf("environment variable '%v' is not set", key)
	}
	values := cstrings.SplitComma(v)
	out := make([]string, len(values))
	for i, p := range values {
		out[i] = quoteYAML(p)
	}
	return strings.Join(out, ","), nil
}

func quoteYAML(val string) string {
	if len(val) == 0 {
		return val
	}
	if strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'") {
		return val
	}
	if strings.ContainsAny(val, ":") {
		return "'" + val + "'"
	}
	return val
}
