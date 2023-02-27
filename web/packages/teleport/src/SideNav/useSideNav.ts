/*
Copyright 2019-2022 Gravitational, Inc.

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

import { useMemo } from 'react';
import { useHistory } from 'react-router';

import * as Icons from 'design/Icon';

import useTeleport from 'teleport/useTeleport';
import useStickyClusterId from 'teleport/useStickyClusterId';
import * as Store from 'teleport/stores/storeNav';
import cfg from 'teleport/config';

export default function useSideNav() {
  const h = useHistory();
  const ctx = useTeleport();
  const { clusterId } = useStickyClusterId();
  const items = useMemo(
    () => makeItems(clusterId, ctx.storeNav.getSideItems()),
    [clusterId]
  );

  return {
    items,
    path: h.location.pathname,
  };
}

function makeItems(clusterId: string, storeItems: Store.NavItem[]) {
  const items = new Map<string, Item>();
  const itemGroups = getGroups();

  for (let i = 0; i < storeItems.length; i++) {
    const cur = storeItems[i];
    const groupName = cur.group;
    const item = {
      items: [] as Item[],
      route: cur.getLink(clusterId),
      exact: cur.exact,
      title: cur.title,
      Icon: cur.Icon,
      isExternalLink: cur.isExternalLink,
    };

    if (itemGroups[groupName]) {
      itemGroups[groupName].items.push(item);
      items.set(groupName, itemGroups[groupName]);
    } else {
      items.set(i + '', item);
    }
  }

  return Array.from(items.values());
}

function getGroups() {
  const groups = {
    team: {
      Icon: Icons.Users,
      title: 'Team',
      items: [] as Item[],
      route: '',
    },
    activity: {
      Icon: Icons.AlarmRing,
      title: 'Activity',
      items: [] as Item[],
      route: '',
    },
    clusters: {
      Icon: Icons.Clusters,
      title: 'Clusters',
      items: [] as Item[],
      route: '',
    },
  };

  if (cfg.isEnterprise) {
    groups['accessrequests'] = {
      Icon: Icons.EqualizerVertical,
      title: 'Access Requests',
      items: [] as Item[],
      route: '',
    };
  }

  return groups;
}

export interface Item {
  items: Item[];
  route: string;
  exact?: boolean;
  title: string;
  Icon: any;
  isExternalLink?: boolean;
}
