/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import ReactDOM from 'react-dom';
import React from 'react';

import history from 'teleport/services/history';

import 'teleport/lib/polyfillRandomUuid';

import Teleport from './Teleport';
import TeleportContext from './teleportContext';
import cfg from './config';

// apply configuration received from the server
cfg.init(window['GRV_CONFIG']);

// use browser history
history.init();

if (localStorage.getItem('enable-telemetry') === 'true') {
  import(/* webpackChunkName: "telemetry" */ './telemetry-boot').then(m =>
    m.instantiateTelemetry()
  );
}

const teleportContext = new TeleportContext();

ReactDOM.render(
  <Teleport history={history.original()} ctx={teleportContext} />,
  document.getElementById('app')
);
