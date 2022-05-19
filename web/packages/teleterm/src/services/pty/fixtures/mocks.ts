import { IPtyProcess } from 'teleterm/sharedProcess/ptyHost';
import { PtyServiceClient } from 'teleterm/services/pty';

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
  createPtyProcess(): Promise<IPtyProcess> {
    return Promise.resolve(new MockPtyProcess());
  }
}
