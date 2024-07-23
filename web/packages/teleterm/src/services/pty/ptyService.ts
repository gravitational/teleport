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

import { buildPtyOptions } from './ptyHost/buildPtyOptions';
import { createPtyHostClient } from './ptyHost/ptyHostClient';
import { createPtyProcess } from './ptyHost/ptyProcess';
import { PtyServiceClient, PtyOptions } from './types';
import { getWindowsPty } from './ptyHost/windowsPty';

export function createPtyService(
  address: string,
  credentials: ChannelCredentials,
  runtimeSettings: RuntimeSettings,
  options: PtyOptions
): PtyServiceClient {
  const ptyHostClient = createPtyHostClient(address, credentials);

  return {
    createPtyProcess: async command => {
      const windowsPty = getWindowsPty(runtimeSettings, options.terminal);
      const { processOptions, creationStatus } = await buildPtyOptions(
        runtimeSettings,
        {
          ssh: options.ssh,
          windowsPty,
        },
        command
      );
      const ptyId = await ptyHostClient.createPtyProcess(processOptions);

      // Electron's context bridge doesn't allow to return a class here
      return {
        process: createPtyProcess(ptyHostClient, ptyId),
        creationStatus,
        windowsPty,
      };
    },
  };
}
