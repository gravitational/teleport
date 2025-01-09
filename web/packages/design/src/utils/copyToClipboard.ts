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
 * Copies text to clipboard.
 *
 * @param textToCopy the text to copy to clipboard
 */
export async function copyToClipboard(textToCopy: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(textToCopy);
  } catch (error) {
    // This can happen if the user denies clipboard permissions.
    showErrorInBrowser(error, textToCopy);
  }
}

function showErrorInBrowser(error: unknown, textToCopy: string): void {
  const errorMessage = error instanceof Error ? error.message : `${error}`;
  const message = `Cannot copy to clipboard. Use Ctrl/Cmd + C.\n${errorMessage}`;
  try {
    // window.prompt doesn't work in Electron.
    window.prompt(message, textToCopy);
  } catch {
    window.alert(message);
  }
}
