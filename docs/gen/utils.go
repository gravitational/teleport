/*
Copyright 2023 Gravitational, Inc.

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
package main

import (
	"fmt"
	"html/template"
	"os"
	"path"
	"strings"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

func outputPath(paths ...string) string {
	return path.Join("docs", "pages", "includes", "generated", path.Join(paths...))
}

func generateMDX(targetPath string, templateStr string, data any) error {
	logrus.Infof("Genereating %s", targetPath)

	if err := os.MkdirAll(path.Dir(targetPath), 0755); err != nil {
		return trace.Wrap(err)
	}
	f, err := os.OpenFile(targetPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return trace.Wrap(err)
	}

	tmpl, err := template.New("").Funcs(templateFuncs).Parse(templateStr)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(tmpl.Execute(f, data))
}

var templateFuncs = template.FuncMap{
	"andSlice":    andSlice,
	"flagName":    flagName,
	"flagDefault": flagDefault,
}

func andSlice(s []string) string {
	switch len(s) {
	case 0:
		return ""
	case 1:
		return s[0]
	default:
		// Example: aa, bb and cc
		return fmt.Sprintf("%s and %s", strings.Join(s[:len(s)-1], ", "), s[len(s)-1])
	}
}

func flagName(flag *kingpin.FlagModel) string {
	if flag.Short != 0 {
		return fmt.Sprintf("`-%s/--%s`", string(flag.Short), flag.Name)
	}
	return fmt.Sprintf("`--%s`", flag.Name)
}

func flagDefault(flag *kingpin.FlagModel) string {
	if len(flag.Default) == 0 {
		return "none"
	}
	values := make([]string, 0, len(flag.Default))
	for i := range flag.Default {
		values = append(values, fmt.Sprintf("`%s`", flag.Default[i]))
	}
	return strings.Join(values, ",")
}
