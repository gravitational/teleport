import Logger, { DefaultService } from 'teleterm/logger';
import { ElectronGlobals } from 'teleterm/types';

export default Logger;

export function initLogger(globals: ElectronGlobals) {
  const settings = globals.mainProcessClient.getRuntimeSettings();
  if (settings.dev) {
    Logger.init(new DefaultService());
  } else {
    Logger.init(globals.loggerService);
  }
}
