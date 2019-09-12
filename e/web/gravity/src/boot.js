import 'es6-shim';
import React from 'react';
import history from 'gravity/services/history';
import ReactDOM from 'react-dom';
import Root from  './index';
import cfg from './config';

// apply configuration provided by the backend
cfg.init(window.GRV_CONFIG);

// use browser history
history.init();

ReactDOM.render( ( <Root history={history.original()}/> ), document.getElementById('app'));