package web

import (
	_ "embed"
	"text/template"
)

//go:embed scripts/install-deb.sh.tmpl
var debInstaller string

// DebInstallerTemplate is a templated ubuntu/debian installer script
var DebInstallerTemplate = template.Must(template.New("deb-installer").Parse(debInstaller))

//go:embed scripts/install-rpm.sh.tmpl
var rpmInstaller string

// RpmInstallerTemplate is a templated ubuntu/rpmian installer script
var RpmInstallerTemplate = template.Must(template.New("rpm-installer").Parse(rpmInstaller))
