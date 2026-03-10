import { createRoot } from 'react-dom/client';

import cfg from 'e-teleport/config';
import 'teleport/lib/polyfillRandomUuid';
import history from 'teleport/services/history';
import { KeysEnum } from 'teleport/services/storageService';

import TeleportContextE from './teleportContextE';
import TeleportE from './TeleportE';

// apply configuration received from the server
cfg.init(window['GRV_CONFIG']);

// use browser history
history.init();

if (localStorage.getItem(KeysEnum.ENABLE_TELEMETRY) === 'true') {
  import('teleport/telemetry-boot').then(m => m.instantiateTelemetry());
}

const teleportContextE = new TeleportContextE();

createRoot(document.getElementById('app')).render(
  <TeleportE history={history.original()} ctx={teleportContextE} />
);
