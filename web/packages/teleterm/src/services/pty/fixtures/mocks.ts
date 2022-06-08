import { IPtyProcess } from 'teleterm/sharedProcess/ptyHost';
import {
  PtyProcessCreationStatus,
  PtyServiceClient,
} from 'teleterm/services/pty';

export class MockPtyProcess implements IPtyProcess {
  start() {}

  write() {}

  resize() {}

  dispose() {}

  onData() {}

  onExit() {}

  onOpen() {}

  getPid() {
    return 0;
  }

  async getCwd() {
    return '';
  }
}

export class MockPtyServiceClient implements PtyServiceClient {
  createPtyProcess(): Promise<{
    process: IPtyProcess;
    creationStatus: PtyProcessCreationStatus;
  }> {
    return Promise.resolve({
      process: new MockPtyProcess(),
      creationStatus: PtyProcessCreationStatus.Ok,
    });
  }
}
