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

// telebase imports
import history from 'telebase-app/services/history';
import { withAllRoutes } from 'telebase-app/routes';
import { AuditFeature, SshFeature } from 'telebase-app/features';
import FeatureActivator from 'telebase-app/featureActivator';
import { initApp } from 'telebase-app/flux/app/actions';
import 'telebase-app/flux';

// app imports
import SettignsFeature from './features/settings';
import reactor from 'app/reactor';
import cfg from 'app/config';
import './flux';

cfg.init(window.GRV_CONFIG);
history.init();

const nestedRoutes = [];
const featureActivator = new FeatureActivator();

featureActivator.register(new SshFeature(nestedRoutes));
featureActivator.register(new AuditFeature(nestedRoutes));
featureActivator.register(new SettignsFeature(nestedRoutes));

const onEnterApp = nextState => {  
  let { siteId } = nextState.params; 
  initApp(siteId, featureActivator)
}

const routes = [
  {       
    onEnter: onEnterApp,
    childRoutes: nestedRoutes        
  }
]

render((  
  <Provider reactor={reactor}>        
    <Router history={history.original()} routes={withAllRoutes(routes)}/>            
  </Provider>  
), document.getElementById("app"));
