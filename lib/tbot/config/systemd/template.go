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

package systemd

import (
	_ "embed"
	"text/template"
)

var (
	//go:embed systemd.tmpl
	templateData string

	// Template is the systemd unit template for tbot..
	Template = template.Must(template.New("").Parse(templateData))
)

// TemplateParams are the parameters for the systemd unit template.
type TemplateParams struct {
	// UnitName is the name of the systemd unit.
	UnitName string
	// User is the user to run the service as.
	User string
	// Group is the group to run the service as.
	Group string
	// AnonymousTelemetry is whether to enable anonymous telemetry.
	AnonymousTelemetry bool
	// ConfigPath is the path to the tbot config file.
	ConfigPath string
	// TBotPath is the path to the tbot binary.
	TBotPath string
	// DiagSocketForUpdater is the path to the diag socket for the updater.
	DiagSocketForUpdater string
}
