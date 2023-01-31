import { PtyCommand, PtyServiceClient } from 'teleterm/services/pty';

export class TerminalsService {
  ptyServiceClient: PtyServiceClient;

  constructor(ptyProvider: PtyServiceClient) {
    this.ptyServiceClient = ptyProvider;
  }

  createPtyProcess(cmd: PtyCommand) {
    return this.ptyServiceClient.createPtyProcess(cmd);
  }
}
