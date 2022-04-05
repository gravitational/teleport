import ReactDOM from 'react-dom';
import React from 'react';
import { ElectronGlobals } from 'teleterm/types';
import App from 'teleterm/ui/App';
import AppContext from 'teleterm/ui/appContext';
import Logger, { initLogger } from 'teleterm/ui/logger';

const globals = window['electron'] as ElectronGlobals;
initLogger(globals);

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
