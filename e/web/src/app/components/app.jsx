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

import { App as TeleApp, Connector as TeleAppConnector}  from 'telebase-app/components/app';
import cfg from '../config';

const menuItems = [
  { icon: 'fa fa-share-alt', to: cfg.routes.nodes, title: 'Nodes' },
  { icon: 'fa fa-group', to: cfg.routes.sessions, title: 'Sessions' },
  { icon: 'fa fa-wrench', to: cfg.routes.settingsBase, title: 'Settings' }
];

class App extends TeleApp {  
  getMenuItems() {            
    return menuItems;
  }
}

export default TeleAppConnector(App);
