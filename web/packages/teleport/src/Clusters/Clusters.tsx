/*
Copyright 2020 Gravitational, Inc.

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
import { Box, Indicator } from 'design';
import { Danger } from 'design/Alert';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import useTeleport from 'teleport/useTeleport';

import ClusterList from './ClusterList';
import useClusters from './useClusters';

export default function Container() {
  const ctx = useTeleport();
  const state = useClusters(ctx);
  return <Clusters {...state} />;
}

export function Clusters(props: ReturnType<typeof useClusters>) {
  const { clusters, enabledFeatures, initAttempt } = props;
  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle>Manage Clusters</FeatureHeaderTitle>
      </FeatureHeader>
      {initAttempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {initAttempt.status === 'failed' && (
        <Danger>{initAttempt.statusText} </Danger>
      )}
      {initAttempt.status === 'success' && (
        <ClusterList
          clusters={clusters}
          menuFlags={{
            showNodes: enabledFeatures.nodes,
            showAudit: enabledFeatures.audit,
            showRecordings: enabledFeatures.recordings,
            showApps: enabledFeatures.apps,
            showDatabases: enabledFeatures.databases,
            showKubes: enabledFeatures.kubes,
            showDesktops: enabledFeatures.desktops,
          }}
        />
      )}
    </FeatureBox>
  );
}
