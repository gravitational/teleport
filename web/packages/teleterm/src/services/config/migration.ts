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

// TODO(gzdunek): when a migration fails an error should be thrown
export function runConfigFileMigration(configFile: FileStorage): void {
  if (configFile.getFileLoadingError()) {
    return;
  }
  const configFileContent = configFile.get();

  // TODO(gzdunek): remove this migration in v14
  const openCommandBarProperty = 'keymap.openCommandBar';
  const openSearchBarProperty = 'keymap.openSearchBar';
  if (configFileContent[openCommandBarProperty]) {
    logger.info(
      `Running migration, renaming ${openCommandBarProperty} -> ${openSearchBarProperty}`
    );
    configFileContent[openSearchBarProperty] =
      configFileContent[openCommandBarProperty];
    delete configFileContent[openCommandBarProperty];
    configFile.replace(configFileContent);
  }
}
