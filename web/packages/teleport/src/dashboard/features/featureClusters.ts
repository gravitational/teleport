/*
Copyright 2019-2020 Gravitational, Inc.

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

import Clusters from 'teleport/dashboard/components/Clusters';
import { withFeature, FeatureBase } from 'teleport/components/withFeature';
import Ctx from 'teleport/teleportContext';
import cfg from 'teleport/config';

class FeatureClusters extends FeatureBase {
  Component = withFeature(this)(Clusters);

  getRoute() {
    return {
      title: 'Clusters',
      path: cfg.routes.app,
      exact: true,
      component: this.Component,
    };
  }

  onload(context: Ctx) {
    this.setProcessing();
    return context.clusterService
      .fetchClusters()
      .then(clusters => context.storeClusters.setState(clusters))
      .then(this.setReady.bind(this))
      .catch(this.setFailed.bind(this));
  }
}

export default FeatureClusters;
