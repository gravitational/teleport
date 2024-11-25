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

import Logger from 'teleterm/logger';
import { FileStorage } from 'teleterm/services/fileStorage';

const logger = new Logger('ConfigMigration');

// TODO(gzdunek): when a migration fails the user should be notified.
// Currently, `FileStorage.write` catches all IO errors.
// It should have a mode of interaction that does not do this and allows callers to handle the errors themselves.
// Related discussion in the PR https://github.com/gravitational/teleport/pull/24051#discussion_r1160753228.

// Additionally, `runConfigFileMigration`
// should be an async function that does not resolve until the migration is complete.
export function runConfigFileMigration(configFile: FileStorage): void {
  if (configFile.getFileLoadingError()) {
    return;
  }
  const configFileContent = configFile.get() as object;

  // DELETE IN 14.0
  // This migration renames 'keymap.openCommandBar' config key to 'keymap.openSearchBar'.
  // We do it because we replaced the 'command bar' feature with the 'search bar',
  // but we want to preserve the keyboard shortcut.
  const openCommandBarProperty = 'keymap.openCommandBar';
  const openSearchBarProperty = 'keymap.openSearchBar';

  // check if the old property exists
  if (openCommandBarProperty in configFileContent) {
    // migrate only if the new property does not exist
    if (!(openSearchBarProperty in configFileContent)) {
      logger.info(
        `Running migration, renaming ${openCommandBarProperty} -> ${openSearchBarProperty}`
      );
      configFileContent[openSearchBarProperty] =
        configFileContent[openCommandBarProperty];
    }

    // remove the old property
    delete configFileContent[openCommandBarProperty];
    configFile.replace(configFileContent);
  }
}
