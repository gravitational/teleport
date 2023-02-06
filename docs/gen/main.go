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

	"github.com/gravitational/teleport/lib/utils"
)

func main() {
	if err := generateAll(); err != nil {
		utils.FatalError(err)
	}
}

func generateAll() error {
	return trace.NewAggregate(
		genDBCreateUserDBNameWarning(),
		genDBReferenceTCLAuthSign(),
	)
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

	tmpl, err := template.New("").Funcs(template.FuncMap{
		"and":      andSlice,
		"flagName": flagName,
	}).Parse(templateStr)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(tmpl.Execute(f, data))
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
