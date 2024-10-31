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

/**
 * saveOnDisk saves content to local disk.
 * @param content content to download (string or Blob).
 * @param filename preset file name.
 * @param fileType file type.
 */
export function saveOnDisk(
  content: string | Blob,
  filename: string,
  fileType: string
): void {
  const a = document.createElement('a');

  const blob =
    content instanceof Blob ? content : new Blob([content], { type: fileType });
  a.href = window.URL.createObjectURL(blob);
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
}
