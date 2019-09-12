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
import FeatureAudit from 'teleport/cluster/features/featureAudit';
import FeatureNodes from 'teleport/cluster/features/featureNodes';
import FeatureSessions from 'teleport/cluster/features/featureSessions';
import Cluster from 'teleport/cluster/components/Cluster';
import { useTeleport } from 'teleport/teleport';

function mapState(props) {
  const { clusterId } = props.match.params;
  const teleport = useTeleport();
  const [features] = React.useState(() => {
    return [
      new FeatureAccount(),
      new FeatureNodes(),
      new FeatureSessions(),
      new FeatureAudit(),
    ];
  });

  function onInit() {
    return teleport.init({ clusterId, features });
  }

  return {
    features,
    onInit,
  };
}

export default withState(mapState)(Cluster);
