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
import { Router } from 'react-router';
import { Provider } from 'nuclear-js-react-addons';

// telebase imports
import history from 'telebase-app/services/history';
import FeatureActivator from 'telebase-app/featureActivator';
import { addRoutes } from 'telebase-app/routes';
import * as Features from 'telebase-app/features';
import userActions from 'telebase-app/flux/user/actions';
import 'telebase-app/flux';
import 'telebase-app/vendor';

// app imports
import { initApp } from './flux/actions';
import { createSettings } from './features';
import reactor from './reactor';
import cfg from './config';
import TeleportE from './components/app';
import './flux';
import './../styles/grv.scss';

cfg.init(window.GRV_CONFIG);
history.init();

const childRoutes = [];
const featureActivator = new FeatureActivator();

featureActivator.register(new Features.Ssh(childRoutes));
featureActivator.register(new Features.Audit(childRoutes));
featureActivator.register(createSettings(childRoutes));

const onEnterApp = nextState => {
  let { siteId } = nextState.params;
  initApp(siteId, featureActivator)
}

const appRoutes = [{
  path: cfg.routes.app,
  onEnter: userActions.ensureUser,
  component: TeleportE,
  childRoutes:  [{
    onEnter: onEnterApp,
    childRoutes
   }]
}];

const Root = () => (
  <Provider reactor={reactor}>
    <Router history={history.original()} routes={addRoutes(appRoutes)} />
  </Provider>
)

export default Root;

