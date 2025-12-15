/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { createRoot } from 'react-dom/client';

import history from 'teleport/services/history';

import 'teleport/lib/polyfillRandomUuid';

import cfg from './config';
import Teleport from './Teleport';
import TeleportContext from './teleportContext';

// apply configuration received from the server
cfg.init(window['GRV_CONFIG']);

// use browser history
history.init();

if (localStorage.getItem('enable-telemetry') === 'true') {
  import('./telemetry-boot').then(m => m.instantiateTelemetry());
}

const teleportContext = new TeleportContext();

createRoot(document.getElementById('app')).render(
  <Teleport history={history.original()} ctx={teleportContext} />
);
