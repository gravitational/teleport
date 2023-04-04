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
