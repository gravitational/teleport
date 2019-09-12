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
import Sessions from 'teleport/cluster/components/Sessions';
import withFeature, { FeatureBase } from 'teleport/components/withFeature';
import cfg from 'teleport/config';

class FeatureNodes extends FeatureBase {
  constructor() {
    super();
    this.Component = withFeature(this)(Sessions);
  }

  getRoute() {
    return {
      title: 'Active Sessions',
      path: cfg.routes.clusterSessions,
      exact: true,
      component: this.Component,
    };
  }

  onload({ context }) {
    context.storeNav.addSideItem({
      title: 'Active Sessions',
      Icon: Icons.Layers,
      exact: true,
      to: cfg.getSessionsRoute(),
    });

    this.setProcessing();
    return context.storeSessions
      .fetchSessions()
      .then(this.setReady.bind(this))
      .catch(this.setFailed.bind(this));
  }
}

export default FeatureNodes;
