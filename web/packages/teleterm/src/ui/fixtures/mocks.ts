import { MockMainProcessClient } from 'teleterm/mainProcess/fixtures/mocks';
import { MockTshClient } from 'teleterm/services/tshd/fixtures/mocks';
import { MockPtyServiceClient } from 'teleterm/services/pty/fixtures/mocks';
import AppContext from 'teleterm/ui/appContext';
import { RuntimeSettings } from 'teleterm/types';

export class MockAppContext extends AppContext {
  constructor(runtimeSettings?: Partial<RuntimeSettings>) {
    const mainProcessClient = new MockMainProcessClient(runtimeSettings);
    const tshdClient = new MockTshClient();
    const ptyServiceClient = new MockPtyServiceClient();

    super({
      mainProcessClient,
      tshClient: tshdClient,
      ptyServiceClient,
      subscribeToTshdEvent: () => {},
    });
  }
}
