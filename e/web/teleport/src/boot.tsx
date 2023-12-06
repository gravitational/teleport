import React from 'react';

import { createRoot } from 'react-dom/client';

import history from 'teleport/services/history';

import 'teleport/lib/polyfillRandomUuid';

import cfg from 'e-teleport/config';

import TeleportE from './TeleportE';
import TeleportContextE from './teleportContextE';

// apply configuration received from the server
cfg.init(window['GRV_CONFIG']);

// use browser history
history.init();

if (localStorage.getItem('enable-telemetry') === 'true') {
  import('teleport/telemetry-boot').then(m => m.instantiateTelemetry());
}

const teleportContextE = new TeleportContextE();

createRoot(document.getElementById('app')).render(
  <TeleportE history={history.original()} ctx={teleportContextE} />
);
