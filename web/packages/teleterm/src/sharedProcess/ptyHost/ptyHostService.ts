import { unique } from 'teleterm/ui/utils';
import { PtyProcess } from './ptyProcess';
import { IPtyHostServer } from './v1/ptyHostService_grpc_pb';
import { PtyCwd, PtyId } from './v1/ptyHostService_pb';
import { PtyEventsStreamHandler } from './ptyEventsStreamHandler';

export function createPtyHostService(): IPtyHostServer {
  const ptyProcesses = new Map<string, PtyProcess>();

  return {
    createPtyProcess: (call, callback) => {
      const ptyOptions = call.request.toObject();
      const ptyId = unique();
      try {
        const ptyProcess = new PtyProcess({
          ...ptyOptions,
          args: ptyOptions.argsList,
          env: call.request.getEnv()?.toJavaScript() as Record<string, string>,
        });
        ptyProcesses.set(ptyId, ptyProcess);
      } catch (error) {
        callback(error);
      }
      callback(null, new PtyId().setId(ptyId));
    },
    getCwd: (call, callback) => {
      const id = call.request.getId();
      const ptyProcess = ptyProcesses.get(id);
      if (!ptyProcess) {
        return callback(new Error(`PTY process with id: ${id} does not exist`));
      }
      ptyProcess
        .getCwd()
        .then(cwd => {
          const response = new PtyCwd().setCwd(cwd);
          callback(null, response);
        })
        .catch(err => {
          callback(err);
        });
    },
    exchangeEvents: stream => new PtyEventsStreamHandler(stream, ptyProcesses),
  };
}
