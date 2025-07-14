/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import {
  autoUpdater,
  AppUpdater as ElectronAppUpdater,
} from 'electron-updater';
import { ProviderRuntimeOptions } from 'electron-updater/out/providers/Provider';

import Logger from 'teleterm/logger';

import { ClientToolsUpdateProvider } from './clientToolsUpdateProvider';

export class AppUpdater {
  private readonly logger = new Logger('AppUpdater');

  constructor() {
    autoUpdater.setFeedURL({
      provider: 'custom',
      // Wraps ClientToolsUpdateProvider to allow passing getClientToolsVersion.
      updateProvider: class extends ClientToolsUpdateProvider {
        constructor(
          options: unknown,
          updater: ElectronAppUpdater,
          runtimeOptions: ProviderRuntimeOptions
        ) {
          super(() => undefined, updater, runtimeOptions);
        }
      },
    });

    autoUpdater.logger = this.logger;
    autoUpdater.autoDownload = true;
    autoUpdater.allowDowngrade = true;
    autoUpdater.autoInstallOnAppQuit = true;
    // Enables checking for updates and downloading them in dev mode.
    // It makes testing this feature easier.
    // Only installing updates requires the packaged app.
    // Downloads are saved to the path specified in dev-app-update.yml.
    autoUpdater.forceDevUpdateConfig = true;
  }
}
