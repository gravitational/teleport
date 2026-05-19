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
	"path/filepath"
	"slices"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
)

const (
	guideURL = "https://goteleport.com/docs/admin-guides/access-controls/access-request-plugins/ssh-approval-msteams/"
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

// ConfigTemplatePayload represents template payloads used to generate config files
// used by the Microsoft Teams plugin.
type ConfigTemplatePayload struct {
	// AppID is the Microsoft application ID.
	AppID string
	// AppSecret is the Microsoft application secret.
	AppSecret string
	// TenantID is the Microsoft Azure tenant ID.
	TenantID string
	// TeamsAppID is the Microsoft Teams application ID.
	TeamsAppID string
}

// Configure creates required template files
func Configure(targetDir, appID, appSecret, tenantID string) error {
	var step byte = 1

	p := ConfigTemplatePayload{
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

	configWriter, err := os.Create(filepath.Join(targetDir, "teleport-msteams.toml"))
	if err != nil {
		return trace.Wrap(err)
	}
	defer configWriter.Close()
	if err := renderTemplateTo(configWriter, confTpl, p); err != nil {
		return trace.Wrap(err)
	}

	appZipFile, err := os.Create(filepath.Join(targetDir, "app.zip"))
	if err != nil {
		return trace.Wrap(err)
	}
	defer appZipFile.Close()

	WriteAppZipTo(appZipFile, p)

	printStep(&step, "Created %v", appZipFile.Name())
	fmt.Println()
	fmt.Printf("TeamsAppID: %v\n", p.TeamsAppID)
	fmt.Println()
	fmt.Println("Follow-along with our getting started guide:")
	fmt.Println()
	fmt.Println(guideURL)

	return nil
}

// WriteAppZipTo creates the manifest.json from the template using the provided payload, then writes the app.zip to the provided writer including the needed assets.
func WriteAppZipTo(zipWriter io.Writer, p ConfigTemplatePayload) error {
	w := zip.NewWriter(zipWriter)
	defer w.Close()

	manifestWriter, err := w.Create("manifest.json")
	if err != nil {
		return trace.Wrap(err)
	}
	if err := renderTemplateTo(manifestWriter, manifestTpl, p); err != nil {
		return trace.Wrap(err)
	}

	copyAssets(w)
	return nil
}

func copyAssets(zipWriter *zip.Writer) error {
	a, err := assets.ReadDir("_tpl")
	if err != nil {
		return trace.Wrap(err)
	}

	for _, d := range a {
		if !slices.Contains(zipFiles, d.Name()) {
			continue
		}
		in, err := assets.Open(filepath.Join("_tpl", d.Name()))
		if err != nil {
			return trace.Wrap(err)
		}
		defer in.Close()

		out, err := zipWriter.Create(d.Name())
		if err != nil {
			return trace.Wrap(err)
		}

		_, err = io.Copy(out, in)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// printStep prints formatted string leaded with step number
func printStep(step *byte, message string, args ...interface{}) {
	p := append([]interface{}{*step}, args...)
	fmt.Printf("[%v] "+message+"\n", p...)
	*step++
}

// renderTemplateTo renders template from a string and writes file to targetPath
func renderTemplateTo(w io.Writer, content string, payload interface{}) error {
	tpl, err := template.New("template").Parse(content)
	if err != nil {
		return trace.Wrap(err)
	}

	err = tpl.ExecuteTemplate(w, "template", payload)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
