//go:build windows

/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package packaging

// ReplaceToolsBinaries extracts executables specified by execNames from archivePath into
// extractDir. After each executable is extracted, it is symlinked from extractDir/[name] to
// toolsDir/[name].
//
// For Windows, archivePath must be a .zip file.
func ReplaceToolsBinaries(toolsDir string, archivePath string, extractPath string, execNames []string) error {
	return replaceZip(toolsDir, archivePath, extractPath, execNames)
}
