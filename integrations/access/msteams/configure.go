// Copyright 2024 Gravitational, Inc
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

package msteams

import (
	"archive/zip"
	"embed"
	"fmt"
	"html/template"
	"io"
	"os"
	"path"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
)

const (
	guideURL = "https://goteleport.com/docs/enterprise/workflow/"
)

var (
	//go:embed _tpl/teleport-msteams.toml
	confTpl string

	//go:embed _tpl/manifest.json
	manifestTpl string

	//go:embed _tpl/outline.png _tpl/color.png _tpl/teleport-msteams-role.yaml
	assets embed.FS

	// zipFiles represents file names which should be compressed into app.zip
	zipFiles = []string{"manifest.json", "outline.png", "color.png"}
)

// payload represents template payload
type payload struct {
	AppID      string
	AppSecret  string
	TenantID   string
	TeamsAppID string
}

// Configure creates required template files
func Configure(targetDir, appID, appSecret, tenantID string) error {
	var step byte = 1

	p := payload{
		AppID:      appID,
		AppSecret:  appSecret,
		TenantID:   tenantID,
		TeamsAppID: uuid.New().String(),
	}

	fi, err := os.Stat(targetDir)
	if err != nil && !os.IsNotExist(err) {
		return trace.Wrap(err)
	}
	if fi != nil {
		return trace.Errorf("%v exists! Please, specify an empty folder", targetDir)
	}

	err = os.MkdirAll(targetDir, 0777)
	if err != nil {
		return trace.Wrap(err)
	}

	printStep(&step, "Created target directory: %s", targetDir)

	if err := renderTemplateTo(confTpl, p, path.Join(targetDir, "teleport-msteams.toml")); err != nil {
		return trace.Wrap(err)
	}
	if err := renderTemplateTo(manifestTpl, p, path.Join(targetDir, "manifest.json")); err != nil {
		return trace.Wrap(err)
	}

	printStep(&step, "Generated configuration files")

	a, err := assets.ReadDir("_tpl")
	if err != nil {
		return trace.Wrap(err)
	}

	for _, d := range a {
		in, err := assets.Open(path.Join("_tpl", d.Name()))
		if err != nil {
			return trace.Wrap(err)
		}
		defer in.Close()

		out, err := os.Create(path.Join(targetDir, d.Name()))
		if err != nil {
			return trace.Wrap(err)
		}
		defer out.Close()

		_, err = io.Copy(out, in)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	printStep(&step, "Copied assets")

	z, err := os.Create(path.Join(targetDir, "app.zip"))
	if err != nil {
		return trace.Wrap(err)
	}
	defer z.Close()

	w := zip.NewWriter(z)
	defer w.Close()

	for _, n := range zipFiles {
		in, err := os.Open(path.Join(targetDir, n))
		if err != nil {
			return trace.Wrap(err)
		}
		defer in.Close()

		out, err := w.Create(n)
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = io.Copy(out, in)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	printStep(&step, "Created app.zip")

	fmt.Println()
	fmt.Printf("TeamsAppID: %v\n", p.TeamsAppID)
	fmt.Println()
	fmt.Println("Follow-along with our getting started guide:")
	fmt.Println()
	fmt.Println(guideURL)

	return nil
}

// printStep prints formatted string leaded with step number
func printStep(step *byte, message string, args ...interface{}) {
	p := append([]interface{}{*step}, args...)
	fmt.Printf("[%v] "+message+"\n", p...)
	*step++
}

// renderTemplateTo renders template from a string and writes file to targetPath
func renderTemplateTo(content string, payload interface{}, targetPath string) error {
	tpl, err := template.New("template").Parse(content)
	if err != nil {
		return trace.Wrap(err)
	}

	w, err := os.Create(targetPath)
	if err != nil {
		return trace.Wrap(err)
	}
	defer w.Close()

	err = tpl.ExecuteTemplate(w, "template", payload)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
