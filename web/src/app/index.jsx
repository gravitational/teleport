/*
Copyright 2015 Gravitational, Inc.

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

import React from 'react';
import { render } from 'react-dom';
import { Router } from 'react-router';
import { Provider } from 'nuclear-js-react-addons';
import history from './services/history';
import cfg from './config';
import reactor from './reactor';
import { addRoutes } from './routes';
import * as Features from './features';
import { createSettings } from './features/settings';
import FeatureActivator from './featureActivator';
import { initApp } from './flux/app/actions';
import App from './components/app.jsx';
import { ensureUser } from './flux/user/actions';
import './flux';

cfg.init(window.GRV_CONFIG);
history.init();

const featureRoutes = [];
const featureActivator = new FeatureActivator();

featureActivator.register(new Features.Ssh(featureRoutes));
featureActivator.register(new Features.Audit(featureRoutes));
featureActivator.register(createSettings(featureRoutes))

const onEnterApp = nextState => {  
  const { siteId } = nextState.params; 
  initApp(siteId, featureActivator)
}

const routes = [{
  path: cfg.routes.app,
  onEnter: ensureUser,
  component: App,
  childRoutes: [{
    onEnter: onEnterApp,
    childRoutes: featureRoutes
  }]
}];

render((  
  <Provider reactor={reactor}>        
    <Router history={history.original()} routes={addRoutes(routes)}/>            
  </Provider>  
), document.getElementById("app"));
