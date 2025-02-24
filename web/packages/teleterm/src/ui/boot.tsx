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

import React from 'react';
import { createRoot } from 'react-dom/client';

import Logger from 'teleterm/logger';
import { ElectronGlobals } from 'teleterm/types';
import { App } from 'teleterm/ui/App';
import AppContext from 'teleterm/ui/appContext';
import { FailedApp } from 'teleterm/ui/components/App';

async function boot(): Promise<void> {
  Logger.init(window['loggerService']);
  const logger = new Logger('UI');

  try {
    const globals = await getElectronGlobals();

    const appContext = new AppContext(globals);

    window.addEventListener('error', event => {
      // The event object is a `ErrorEvent` instance if it was generated from
      // a user interface element, or an `Event` instance otherwise.
      // https://developer.mozilla.org/en-US/docs/Web/API/Window/error_event#event_type
      const message = event.error ? event.error.stack : event.message;
      console.error(message);
      logger.error(message);
    });

    window.addEventListener('unhandledrejection', event => {
      logger.error(event.reason.stack);
    });

    renderApp(<App ctx={appContext} />);
  } catch (e) {
    logger.error('Failed to boot the React app', e);
    renderApp(
      <FailedApp message={`Could not start the application: ${e.toString()}`} />
    );
  }
}

/**
 * getElectronGlobals retrieves privileged APIs exposed through the contextBridge from preload.ts.
 *
 * It also immediately removes them from the window object so that if an attacker gets to execute
 * arbitrary JS on the page, they don't get easy access to those privileged APIs.
 */
async function getElectronGlobals(): Promise<ElectronGlobals> {
  const globals = await window['electron'];
  const globalsCopy = { ...globals };

  // Technically, each value exposed through the contextBridge gets frozen. [1] Since we expose a
  // promise returning an object however, we can delete properties from that object, effectively
  // removing the APIs from the window object.
  //
  // We suspect that the semantics of this might change between Electron or Chromium updates.
  // At the moment we're in the process of investigating how brittle this workaround is. [2]
  //
  // [1] https://www.electronjs.org/docs/latest/api/context-bridge#api
  // [2] https://github.com/electron/electron/issues/38243
  for (const property in globals) {
    delete globals[property];
  }

  return globalsCopy;
}

function renderApp(content: React.ReactElement): void {
  createRoot(document.getElementById('app')).render(content);
}

boot();
