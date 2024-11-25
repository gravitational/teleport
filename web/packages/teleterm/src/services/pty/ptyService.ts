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

import { ChannelCredentials } from '@grpc/grpc-js';

import { RuntimeSettings } from 'teleterm/mainProcess/types';
import { ConfigService } from 'teleterm/services/config';

import { buildPtyOptions } from './ptyHost/buildPtyOptions';
import { createPtyHostClient } from './ptyHost/ptyHostClient';
import { createPtyProcess } from './ptyHost/ptyProcess';
import { getWindowsPty } from './ptyHost/windowsPty';
import { PtyServiceClient } from './types';

export function createPtyService(
  address: string,
  credentials: ChannelCredentials,
  runtimeSettings: RuntimeSettings,
  configService: ConfigService
): PtyServiceClient {
  const ptyHostClient = createPtyHostClient(address, credentials);

  return {
    createPtyProcess: async command => {
      const windowsPty = getWindowsPty(runtimeSettings, {
        windowsBackend: configService.get('terminal.windowsBackend').value,
      });
      const { processOptions, creationStatus, shell } = await buildPtyOptions({
        settings: runtimeSettings,
        options: {
          ssh: {
            noResume: configService.get('ssh.noResume').value,
            forwardAgent: configService.get('ssh.forwardAgent').value,
          },
          customShellPath: configService.get('terminal.customShell').value,
          windowsPty,
        },
        cmd: command,
      });
      const ptyId = await ptyHostClient.createPtyProcess(processOptions);

      // Electron's context bridge doesn't allow to return a class here
      return {
        process: createPtyProcess(ptyHostClient, ptyId),
        creationStatus,
        shell,
        windowsPty,
      };
    },
  };
}
