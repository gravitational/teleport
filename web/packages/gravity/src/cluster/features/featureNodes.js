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

import cfg from 'gravity/config'
import withFeature, { FeatureBase } from 'gravity/components/withFeature';
import { addSideNavItem } from 'gravity/cluster/flux/nav/actions';
import { fetchNodes } from 'gravity/cluster/flux/nodes/actions';
import * as Icons from 'design/Icon';
import Nodes from '../components/Nodes';

class FeatureNodes extends FeatureBase {
  constructor(){
    super()
    this.Component = withFeature(this)(Nodes);
  }

  getRoute(){
    return {
      title: 'Nodes',
      path: cfg.routes.siteServers,
      exact: true,
      component: this.Component
    }
  }

  onload() {
    addSideNavItem({
      title: 'Nodes',
      Icon: Icons.Layers,
      exact: true,
      to: cfg.getSiteServersRoute()
    })

    this.setProcessing();
    fetchNodes()
      .done(() => this.setReady())
      .fail(this.setFailed.bind(this));
  }
}

export default FeatureNodes;
