import { NsisUpdater } from 'electron-updater';
import { InstallOptions } from 'electron-updater/out/BaseUpdater';

export class NsisCustomUpdater extends NsisUpdater {
  constructor(private opts: { getProxyHosts(): string[] }) {
    super();
  }

  protected doInstall(options: InstallOptions): boolean {
    if (!options.isAdminRightsRequired) {
      return super.doInstall(options);
    }
    void this.spawnLog('sc', [
      'start',
      'TeleportUpdateService',
      'update-service',
      `--path=${super.installerPath}`,
      `--proxy-hosts=${this.opts.getProxyHosts()}`,
    ]);
    return true;
  }
}
