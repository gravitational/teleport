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

import path from 'node:path';

import { app, shell } from 'electron';
import { NsisUpdater } from 'electron-updater';
import { InstallOptions } from 'electron-updater/out/BaseUpdater';

/**
 * Extends the standard NSIS to ensure that a per-user installation won't attempt
 * to update the per-machine one.
 */
export class NsisDualModeUpdater extends NsisUpdater {
  constructor() {
    super();
  }

  protected doInstall(options: InstallOptions): boolean {
    if (isInstalledPerMachine()) {
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
 * Teleport Connect assumes a per-machine installation when it is installed under
 * Program Files.
 * Although the NSIS installer allows the installation path to be changed using
 * the \D argument, our custom installation script does not fully support this.
 * As a result, installing to a location other than Program Files causes errors
 * when setting up system services.
 */
function isInstalledPerMachine(): boolean {
  const exePath = app.getPath('exe');
  // Users could modify this env var to force a per-machine update.
  // This is not a security concern as the service can be invoked manually anyway.
  const programFiles = process.env.ProgramFiles || 'C:\\Program Files';
  // Normalize paths to lowercase for comparison (Windows is case-insensitive)
  const currentPath = exePath.toLowerCase();

  return currentPath.startsWith(
    path.join(programFiles.toLowerCase(), path.sep)
  );
}
