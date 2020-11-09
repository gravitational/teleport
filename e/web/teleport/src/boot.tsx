import ReactDOM from 'react-dom';
import React from 'react';
import history from 'teleport/services/history';
import cfg from 'teleport/config';
import TeleportE from './TeleportE';

// apply configuration received from the server
cfg.init(window['GRV_CONFIG']);
cfg.isEnterprise = true;

// use browser history
history.init();

ReactDOM.render(
  <TeleportE history={history.original()} />,
  document.getElementById('app')
);
