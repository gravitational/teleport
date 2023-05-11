/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import ReactDOM from 'react-dom';
import React from 'react';

import { ElectronGlobals } from 'teleterm/types';
import { App, FailedApp } from 'teleterm/ui/App';
import AppContext from 'teleterm/ui/appContext';
import Logger from 'teleterm/logger';

async function boot(): Promise<void> {
  Logger.init(window['loggerService']);
  const logger = new Logger('UI');

  try {
    const globals = await getElectronGlobals();

    const appContext = new AppContext(globals);

    window.addEventListener('error', event => {
      console.error(event.error.stack);
      logger.error(event.error.stack);
    });

    window.addEventListener('unhandledrejection', event => {
      logger.error(event.reason.stack);
    });

    renderApp(<App ctx={appContext} />);
  } catch (e) {
    logger.error('Failed to boot the React app', e);
    renderApp(<FailedApp message={e.toString()} />);
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
  ReactDOM.render(content, document.getElementById('app'));
}

boot();
