import { NsisUpdater } from 'electron-updater';
import { InstallOptions } from 'electron-updater/out/BaseUpdater';

export class NsisCustomUpdater extends NsisUpdater {
  constructor(
    private opts: { installUpdate: (path: string) => Promise<void> }
  ) {
    super();
  }

  protected doInstall(options: InstallOptions): boolean {
    if (!options.isAdminRightsRequired) {
      return super.doInstall(options);
    }
    void this.opts.installUpdate(super.installerPath);
  }
}
