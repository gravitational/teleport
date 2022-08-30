import { unique } from 'teleterm/ui/utils';

import Logger from 'teleterm/logger';

import { PtyProcess } from './ptyProcess';
import { IPtyHostServer } from './../api/protogen/ptyHostService_grpc_pb';
import { PtyCwd, PtyId } from './../api/protogen/ptyHostService_pb';
import { PtyEventsStreamHandler } from './ptyEventsStreamHandler';

export function createPtyHostService(): IPtyHostServer {
  const logger = new Logger('PtyHostService');
  const ptyProcesses = new Map<string, PtyProcess>();

  return {
    createPtyProcess: (call, callback) => {
      const ptyOptions = call.request.toObject();
      const ptyId = unique();
      try {
        const ptyProcess = new PtyProcess({
          ...ptyOptions,
          ptyId,
          args: ptyOptions.argsList,
          env: call.request.getEnv()?.toJavaScript() as Record<string, string>,
        });
        ptyProcesses.set(ptyId, ptyProcess);
      } catch (error) {
        logger.error(`failed to create PTY process for id ${ptyId}`, error);
        callback(error);
      }
      callback(null, new PtyId().setId(ptyId));
      logger.info(`created PTY process for id ${ptyId}`);
    },
    getCwd: (call, callback) => {
      const id = call.request.getId();
      const ptyProcess = ptyProcesses.get(id);
      if (!ptyProcess) {
        const message = `PTY process with id: ${id} does not exist`;
        logger.warn(message);
        return callback(new Error(message));
      }
      ptyProcess
        .getCwd()
        .then(cwd => {
          const response = new PtyCwd().setCwd(cwd);
          callback(null, response);
        })
        .catch(error => {
          logger.error(`could not read CWD for id: ${id}`, error);
          callback(error);
        });
    },
    exchangeEvents: stream => new PtyEventsStreamHandler(stream, ptyProcesses),
  };
}
