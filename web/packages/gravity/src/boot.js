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