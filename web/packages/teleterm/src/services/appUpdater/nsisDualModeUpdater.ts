/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { execFileSync } from 'node:child_process';
import path from 'node:path';

import { app, shell } from 'electron';
import { NsisUpdater } from 'electron-updater';
import { InstallOptions } from 'electron-updater/out/BaseUpdater';

/** Defined in electron-builder-config.js. */
const TELEPORT_CONNECT_NSIS_GUID = '22539266-67e8-54a3-83b9-dfdca7b33ee1';

/**
 * Extends the standard NSIS to ensure that a per-user installation won't attempt
 * to update the per-machine one.
 */
export class NsisDualModeUpdater extends NsisUpdater {
  constructor() {
    super();
  }

  protected override doInstall(options: InstallOptions): boolean {
    let installedPerMachine = false;
    try {
      installedPerMachine = isInstalledPerMachine();
    } catch (e) {
      this.logger.error(
        `Could not check if app is installed per machine, defaulting to per-user update. ${e}`
      );
    }
    if (installedPerMachine) {
      // TODO(gzdunek): Call the privileged update service.
      return super.doInstall(options);
    } else {
      return this.doInstallPerUser(options);
    }
  }

  /**
   * Copied from the NSIS updater.
   * The only change is the addition of the `/currentuser` flag to prevent attempts
   * to update an existing per-machine installation.
   */
  protected doInstallPerUser(options: InstallOptions): boolean {
    const installerPath = this.installerPath;
    if (installerPath == null) {
      this.dispatchError(
        new Error("No update filepath provided, can't quit and install")
      );
      return false;
    }

    const args = ['--updated'];

    // Do not attempt to update the per-machine version if it exists.
    args.push('/currentuser');

    if (options.isSilent) {
      args.push('/S');
    }

    if (options.isForceRunAfter) {
      args.push('--force-run');
    }

    if (this.installDirectory) {
      // maybe check if folder exists
      args.push(`/D=${this.installDirectory}`);
    }

    const packagePath =
      this.downloadedUpdateHelper == null
        ? null
        : this.downloadedUpdateHelper.packageFile;
    if (packagePath != null) {
      // only = form is supported
      args.push(`--package-file=${packagePath}`);
    }

    const callUsingElevation = (): void => {
      this.spawnLog(
        path.join(process.resourcesPath, 'elevate.exe'),
        [installerPath].concat(args)
      ).catch(e => this.dispatchError(e));
    };

    if (options.isAdminRightsRequired) {
      this._logger.info(
        'isAdminRightsRequired is set to true, run installer using elevate.exe'
      );
      callUsingElevation();
      return true;
    }

    this.spawnLog(installerPath, args).catch((e: Error) => {
      // https://github.com/electron-userland/electron-builder/issues/1129
      // Node 8 sends errors: https://nodejs.org/dist/latest-v8.x/docs/api/errors.html#errors_common_system_errors
      const errorCode = (e as NodeJS.ErrnoException).code;
      this._logger.info(
        `Cannot run installer: error code: ${errorCode}, error message: "${e.message}", will be executed again using elevate if EACCES, and will try to use electron.shell.openItem if ENOENT`
      );
      if (errorCode === 'UNKNOWN' || errorCode === 'EACCES') {
        callUsingElevation();
      } else if (errorCode === 'ENOENT') {
        shell
          .openPath(installerPath)
          .catch((err: Error) => this.dispatchError(err));
      } else {
        this.dispatchError(e);
      }
    });
    return true;
  }
}

/**
 * Checks if Teleport Connect is installed per-machine by comparing the executable
 * directory with InstallLocation in Connect's HKLM hive.
 */
function isInstalledPerMachine(): boolean {
  const exeDirPath = path.dirname(app.getPath('exe'));
  const perMachinePath = readPerMachineLocationFromRegistry();
  return path.resolve(perMachinePath) === path.resolve(exeDirPath);
}

function readPerMachineLocationFromRegistry(): string {
  return execFileSync(
    'powershell.exe',
    [
      '-NoProfile',
      '-NonInteractive',
      '-Command',
      `Get-ItemPropertyValue "HKLM:\\SOFTWARE\\${TELEPORT_CONNECT_NSIS_GUID}" -Name "InstallLocation" -ErrorAction Stop`,
    ],
    { encoding: 'utf8', windowsHide: true }
  ).trim();
}
