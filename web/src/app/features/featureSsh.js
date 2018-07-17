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

import cfg from 'app/config'
import Nodes from 'app/components/nodes/main';
import FeatureBase from './../featureBase';
import { addNavItem } from './../flux/app/actions';
import Index from '../components/terminal';

const sshRoutes = [
  {
    path: cfg.routes.nodes,
    title: "Nodes",
    component: Nodes
  }, {
    path: cfg.routes.terminal,
    title: "Terminal",
    components: {
      CurrentSessionHost: Index
    }
  }
]

const sshNavItem = {
  icon: 'fa fa-share-alt',
  to: cfg.routes.nodes,
  title: 'Nodes'
}

class SshFeature extends FeatureBase {

  constructor(routes) {
    super();
    routes.push(...sshRoutes);
  }

  onload() {
    addNavItem(sshNavItem);
  }
}

export default SshFeature;