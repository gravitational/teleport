import ReactDOM from 'react-dom';
import React from 'react';

import { ElectronGlobals } from 'teleterm/types';
import App from 'teleterm/ui/App';
import AppContext from 'teleterm/ui/appContext';
import Logger from 'teleterm/logger';

const globals = window['electron'] as ElectronGlobals;
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

ReactDOM.render(<App ctx={appContext} />, document.getElementById('app'));
