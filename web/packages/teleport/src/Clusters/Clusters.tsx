/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useEffect, useState } from 'react';

import { Box, Indicator } from 'design';
import { Danger } from 'design/Alert';
import useAttempt from 'shared/hooks/useAttemptNext';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { Route, Switch } from 'teleport/components/Router';
import cfg from 'teleport/config';
import { useFeatures } from 'teleport/FeaturesContext';
import useTeleport from 'teleport/useTeleport';

import ClusterList from './ClusterList';
import { ManageCluster } from './ManageCluster';
import { buildACL } from './utils';

export function Clusters() {
  return (
    <Switch>
      <Route
        key="cluster-list"
        exact
        path={cfg.routes.clusters}
        component={ClusterListPage}
      />
      <Route
        key="cluster-management"
        path={cfg.routes.manageCluster}
        component={ManageCluster}
      />
    </Switch>
  );
}

export function ClusterListPage() {
  const ctx = useTeleport();

  const [clusters, setClusters] = useState([]);
  const { attempt, run } = useAttempt();

  const features = useFeatures();

  function init() {
    run(() => ctx.clusterService.fetchClusters().then(setClusters));
  }

  const [enabledFeatures] = useState(() => buildACL(features));

  useEffect(() => {
    init();
  }, []);

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle>Manage Clusters</FeatureHeaderTitle>
      </FeatureHeader>
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'failed' && <Danger>{attempt.statusText} </Danger>}
      {attempt.status === 'success' && (
        <ClusterList
          clusters={clusters}
          menuFlags={{
            showResources: enabledFeatures.resources,
            showAudit: enabledFeatures.audit,
            showRecordings: enabledFeatures.recordings,
          }}
        />
      )}
    </FeatureBox>
  );
}
