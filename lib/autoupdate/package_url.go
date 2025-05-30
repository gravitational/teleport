/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package autoupdate

import (
	"bytes"
	"encoding/json"
	"runtime"
	"text/template"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
)

// InstallFlags sets flags for the Teleport installation.
type InstallFlags int

const (
	// FlagEnterprise installs enterprise Teleport.
	FlagEnterprise InstallFlags = 1 << iota
	// FlagFIPS installs FIPS Teleport
	FlagFIPS
)

const (
	// DefaultBaseURL is CDN URL for downloading official Teleport packages.
	DefaultBaseURL = "https://cdn.teleport.dev"
	// DefaultPackage is the name of Teleport package.
	DefaultPackage = "teleport"
	// DefaultCDNURITemplate is the default template for the Teleport CDN download URL.
	DefaultCDNURITemplate = `{{ .BaseURL }}/
	{{- if eq .OS "darwin" }}
	{{- .Package }}{{ if and .Enterprise (eq .Package "teleport") }}-ent{{ end }}-{{ .Version }}.pkg
	{{- else if eq .OS "windows" }}
	{{- .Package }}-v{{ .Version }}-{{ .OS }}-amd64-bin.zip
	{{- else }}
	{{- .Package }}{{ if .Enterprise }}-ent{{ end }}-v{{ .Version }}-{{ .OS }}-{{ .Arch }}{{ if .FIPS }}-fips{{ end }}-bin.tar.gz
	{{- end }}`
	// BaseURLEnvVar allows to override base URL for the Teleport package URL via env var.
	BaseURLEnvVar = "TELEPORT_CDN_BASE_URL"
)

// NewInstallFlagsFromStrings returns InstallFlags given a slice of human-readable strings.
func NewInstallFlagsFromStrings(s []string) InstallFlags {
	var out InstallFlags
	for _, f := range s {
		for _, flag := range []InstallFlags{
			FlagEnterprise,
			FlagFIPS,
		} {
			if f == flag.String() {
				out |= flag
			}
		}
	}
	return out
}

// Strings converts InstallFlags to a slice of human-readable strings.
func (i InstallFlags) Strings() []string {
	var out []string
	for _, flag := range []InstallFlags{
		FlagEnterprise,
		FlagFIPS,
	} {
		if i&flag != 0 {
			out = append(out, flag.String())
		}
	}
	return out
}

// String returns the string representation of a single InstallFlag flag, or "Unknown".
func (i InstallFlags) String() string {
	switch i {
	case 0:
		return ""
	case FlagEnterprise:
		return "Enterprise"
	case FlagFIPS:
		return "FIPS"
	}
	return "Unknown"
}

// DirFlag returns the directory path representation of a single InstallFlag flag, or "unknown".
func (i InstallFlags) DirFlag() string {
	switch i {
	case 0:
		return ""
	case FlagEnterprise:
		return "ent"
	case FlagFIPS:
		return "fips"
	}
	return "unknown"
}

func (i InstallFlags) MarshalYAML() (any, error) {
	return i.Strings(), nil
}

func (i InstallFlags) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.Strings())
}

func (i *InstallFlags) UnmarshalYAML(n *yaml.Node) error {
	var s []string
	if err := n.Decode(&s); err != nil {
		return trace.Wrap(err)
	}
	if i == nil {
		return trace.BadParameter("nil install flags while parsing YAML")
	}
	*i = NewInstallFlagsFromStrings(s)
	return nil
}

// MakeURL constructs the package download URL from template, base URL and revision.
func MakeURL(uriTmpl string, baseURL string, pkg string, version string, flags InstallFlags) (string, error) {
	tmpl, err := template.New("uri").Parse(uriTmpl)
	if err != nil {
		return "", trace.Wrap(err)
	}
	var uriBuf bytes.Buffer
	params := struct {
		BaseURL, OS, Version, Arch, Package string
		FIPS, Enterprise                    bool
	}{
		BaseURL:    baseURL,
		OS:         runtime.GOOS,
		Version:    version,
		Arch:       runtime.GOARCH,
		FIPS:       flags&FlagFIPS != 0,
		Enterprise: flags&(FlagEnterprise|FlagFIPS) != 0,
		Package:    pkg,
	}
	err = tmpl.Execute(&uriBuf, params)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return uriBuf.String(), nil
}
