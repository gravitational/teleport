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

import * as Icons from 'design/Icon';
import Nodes from 'teleport/cluster/components/Nodes';
import { withFeature, FeatureBase } from 'teleport/components/withFeature';
import Ctx from 'teleport/teleportContext';
import cfg from 'teleport/config';

class FeatureNodes extends FeatureBase {
  Component = withFeature(this)(Nodes);

  getRoute() {
    return {
      title: 'Nodes',
      path: cfg.routes.clusterNodes,
      exact: true,
      component: this.Component,
    };
  }

  onload(context: Ctx) {
    context.storeNav.addSideItem({
      title: 'Nodes',
      Icon: Icons.Layers,
      exact: true,
      to: cfg.getNodesRoute(),
    });

    this.setProcessing();
    return context.storeNodes
      .fetchNodes()
      .then(this.setReady.bind(this))
      .catch(this.setFailed.bind(this));
  }
}

export default FeatureNodes;
