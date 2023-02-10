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
  try {
    const globals = await getElectronGlobals();
    Logger.init(globals.loggerService);

    const logger = new Logger('UI');
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
    renderApp(<FailedApp message={e.toString()} />);
  }
}

async function getElectronGlobals(): Promise<ElectronGlobals> {
  return await window['electron'];
}

function renderApp(content: React.ReactNode): void {
  ReactDOM.render(content, document.getElementById('app'));
}

boot();
