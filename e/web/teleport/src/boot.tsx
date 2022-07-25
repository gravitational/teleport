import ReactDOM from 'react-dom';
import React from 'react';
import history from 'teleport/services/history';

import cfg from 'e-teleport/config';

import TeleportE from './TeleportE';
import TeleportContextE from './teleportContextE';

// apply configuration received from the server
cfg.init(window['GRV_CONFIG']);

// use browser history
history.init();

const teleportContextE = new TeleportContextE();

ReactDOM.render(
  <TeleportE history={history.original()} ctx={teleportContextE} />,
  document.getElementById('app')
);
