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

import Logger from 'teleterm/logger';
import { Struct } from 'teleterm/sharedProcess/api/protogen/google/protobuf/struct_pb';
import { unique } from 'teleterm/ui/utils';

import { PtyCwd, PtyId } from './../api/protogen/ptyHostService_pb';
import { IPtyHost } from './../api/protogen/ptyHostService_pb.grpc-server';
import { PtyEventsStreamHandler } from './ptyEventsStreamHandler';
import { PtyProcess } from './ptyProcess';

export function createPtyHostService(): IPtyHost & {
  dispose(): Promise<void>;
} {
  const logger = new Logger('PtyHostService');
  const ptyProcesses = new Map<string, PtyProcess>();

  return {
    createPtyProcess: (call, callback) => {
      const ptyOptions = call.request;
      const ptyId = unique();
      try {
        const ptyProcess = new PtyProcess({
          path: ptyOptions.path,
          args: ptyOptions.args,
          cwd: ptyOptions.cwd,
          ptyId,
          env: Struct.toJson(call.request.env!) as Record<string, string>,
          initMessage: ptyOptions.initMessage,
          useConpty: ptyOptions.useConpty,
        });
        ptyProcesses.set(ptyId, ptyProcess);
      } catch (error) {
        logger.error(`failed to create PTY process for id ${ptyId}`, error);
        callback(error);
        return;
      }
      callback(null, PtyId.create({ id: ptyId }));
      logger.info(`created PTY process for id ${ptyId}`);
    },
    getCwd: (call, callback) => {
      const id = call.request.id;
      const ptyProcess = ptyProcesses.get(id);
      if (!ptyProcess) {
        const message = `PTY process with id: ${id} does not exist`;
        logger.warn(message);
        return callback(new Error(message));
      }
      ptyProcess
        .getCwd()
        .then(cwd => {
          const response = PtyCwd.create({ cwd });
          callback(null, response);
        })
        .catch(error => {
          logger.error(`could not read CWD for id: ${id}`, error);
          callback(error);
        });
    },
    exchangeEvents: stream => new PtyEventsStreamHandler(stream, ptyProcesses),
    dispose: async () => {
      await Promise.all(
        Array.from(ptyProcesses.values()).map(ptyProcess =>
          ptyProcess.dispose()
        )
      );
    },
  };
}
