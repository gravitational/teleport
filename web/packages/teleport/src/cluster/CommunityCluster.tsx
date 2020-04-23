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
import { useParams } from 'shared/components/Router';
import FeatureAccount from 'teleport/features/featureAccount';
import FeatureAudit from 'teleport/cluster/features/featureAudit';
import FeatureNodes from 'teleport/cluster/features/featureNodes';
import FeatureSessions from 'teleport/cluster/features/featureSessions';
import FeatureSupport from 'teleport/cluster/features/featureSupport';
import Cluster from 'teleport/cluster/components/Cluster';
import TeleportContext from 'teleport/teleportContext';
import TeleportContextProvider from 'teleport/teleportContextProvider';

export default function CommunityCluster() {
  const { clusterId } = useParams();
  const [ctx] = React.useState(() => {
    const features = [
      new FeatureAccount(),
      new FeatureNodes(),
      new FeatureSessions(),
      new FeatureAudit(),
      new FeatureSupport(),
    ];
    return new TeleportContext({ clusterId, features });
  });

  return (
    <TeleportContextProvider value={ctx}>
      <Cluster />
    </TeleportContextProvider>
  );
}
