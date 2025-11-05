/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

/**
 * Thrown when the app cannot be updated to unsupported version.
 *
 * Kept in a separate file to allow importing in the renderer process.
 */
export class UnsupportedVersionError extends Error {
  constructor(wantedVersion: string, minVersion: string) {
    super(
      `Teleport Connect cannot update to version ${wantedVersion}. Managed updates are supported in version ${minVersion} and later.`
    );
    this.name = 'UnsupportedVersionError';
  }
}
