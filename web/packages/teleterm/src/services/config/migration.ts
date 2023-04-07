/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { FileStorage } from 'teleterm/services/fileStorage';
import Logger from 'teleterm/logger';

const logger = new Logger('ConfigMigration');

// TODO(gzdunek): when a migration fails an error should be thrown.
// Currently, `FileStorage.write` catches all IO errors,
// but instead they should be propagated and handled at higher levels.
// If an error occurs, we should show it to the user and stop the app from working.
//
// Additionally, `runConfigFileMigration`
// should be an async function that does not resolve until the migration is complete.
export function runConfigFileMigration(configFile: FileStorage): void {
  if (configFile.getFileLoadingError()) {
    return;
  }
  const configFileContent = configFile.get() as object;

  // DELETE IN 14.0
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
