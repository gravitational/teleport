import { PtyProcess, PtyServiceClient } from './../types';

export class MockPtyProcess implements PtyProcess {
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

  getCwd = async () => '';
}

export class MockPtyServiceClient implements PtyServiceClient {
  createPtyProcess(): Promise<PtyProcess> {
    return Promise.resolve(new MockPtyProcess());
  }
}
