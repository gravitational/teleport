/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import crypto from 'crypto';
import fs from 'fs';

const UUID_V4_REGEX =
  /^[0-9A-F]{8}-[0-9A-F]{4}-4[0-9A-F]{3}-[89AB][0-9A-F]{3}-[0-9A-F]{12}$/i;

/**
 * Returns a unique ID (UUIDv4) of the installed app. The ID is stored in a file
 * under a specified path. If the file containing the value does not exist or has
 * an invalid format then it is automatically (re-)generated.
 */
export function loadInstallationId(filePath: string): string {
  let id = '';
  try {
    id = fs.readFileSync(filePath, 'utf-8');
  } catch {
    return writeInstallationId(filePath);
  }
  if (!UUID_V4_REGEX.test(id)) {
    return writeInstallationId(filePath);
  }
  return id;
}

function writeInstallationId(filePath: string): string {
  const newId = crypto.randomUUID();
  try {
    fs.writeFileSync(filePath, newId);
  } catch (error) {
    throw new Error(
      `Could not write installation_id to ${filePath}, ${error.message}`
    );
  }
  return newId;
}
