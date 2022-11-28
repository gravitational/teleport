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

async function getElectronGlobals(): Promise<ElectronGlobals> {
  return await window['electron'];
}

function renderApp(content: React.ReactElement): void {
  ReactDOM.render(content, document.getElementById('app'));
}

boot();
