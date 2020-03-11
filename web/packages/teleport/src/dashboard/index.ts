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

import React from 'react';
import { withState } from 'shared/hooks';
import FeatureAccount from 'teleport/features/featureAccount';
import FeatureClusters from 'teleport/dashboard/features/featureClusters';
import Dashboard from 'teleport/dashboard/components';
import { useTeleport } from 'teleport/teleportContextProvider';
import cfg from 'teleport/config';

function mapState() {
  const teleport = useTeleport();
  const [features] = React.useState(() => {
    return [new FeatureAccount(), new FeatureClusters()];
  });

  function onInit() {
    return teleport.init({ clusterId: cfg.proxyCluster, features });
  }

  return {
    features,
    onInit,
  };
}

export default withState(mapState)(Dashboard);
