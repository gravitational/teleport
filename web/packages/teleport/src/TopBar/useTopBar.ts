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

import { matchPath, useHistory } from 'react-router';

import session from 'teleport/services/websession';
import Ctx from 'teleport/teleportContext';
import cfg from 'teleport/config';
import { StickyCluster } from 'teleport/types';

export default function useTopBar(ctx: Ctx, stickyCluster: StickyCluster) {
  const history = useHistory();
  const { clusterId, hasClusterUrl } = stickyCluster;
  const popupItems = ctx.storeNav.getTopMenuItems();
  const { username } = ctx.storeUser.state;
  const loc = history.location;

  // find active feature
  const feature = ctx.features.find(f =>
    matchPath(loc.pathname, {
      path: f.route.path,
      exact: false,
    })
  );

  const title = feature?.topNavTitle || '';

  function loadClusters() {
    return ctx.clusterService.fetchClusters();
  }

  function logout() {
    session.logout();
  }

  function changeCluster(value: string) {
    const newPrefix = cfg.getClusterRoute(value);
    const oldPrefix = cfg.getClusterRoute(clusterId);
    const newPath = loc.pathname.replace(oldPrefix, newPrefix);
    history.push(newPath);
  }

  return {
    clusterId,
    hasClusterUrl,
    popupItems,
    username,
    changeCluster,
    loadClusters,
    logout,
    title,
  };
}
