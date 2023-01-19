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
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';
import * as Icons from 'design/Icon';
import { Box } from 'design';

import { SideNav } from './SideNav';
import { Item } from './useSideNav';

export default {
  title: 'Teleport/SideNav',
};

export const Story = () => {
  const props = {
    ...defaultProps,
  };

  const inMemoryHistory = createMemoryHistory({
    initialEntries: ['/web/roles'],
    initialIndex: 0,
  });
  return (
    <Box
      mt={-3}
      mx={-3}
      height="100%"
      style={{ position: 'absolute', overflow: 'hidden' }}
    >
      <Router history={inMemoryHistory}>
        <SideNav {...props} />
      </Router>
    </Box>
  );
};

const defaultProps = {
  path: '/web/roles',
  items: [
    {
      items: [],
      route: '/web/cluster/one/nodes',
      Icon: Icons.Server,
      exact: true,
      title: 'Servers',
    },
    {
      items: [],
      route: '/web/cluster/one/apps',
      Icon: Icons.NewTab,
      exact: true,
      title: 'Applications',
    },
    {
      items: [],
      route: '/web/cluster/one/kubes',
      Icon: Icons.Kubernetes,
      exact: true,
      title: 'Kubernetes',
    },
    {
      title: 'Team',
      Icon: Icons.Users,
      items: [
        {
          items: [],
          route: '/web/users',
          Icon: Icons.Users,
          exact: true,
          title: 'Users',
        },
        {
          items: [],
          route: '/web/roles',
          Icon: Icons.Profile,
          exact: true,
          title: 'Roles',
        },
        {
          items: [],
          route: '/web/sso',
          Icon: Icons.Lock,
          exact: false,
          title: 'Auth. Connectors',
        },
      ],
      route: '',
    },
    {
      title: 'Activity',
      Icon: Icons.AlarmRing,
      items: [
        {
          items: [],
          route: '/web/cluster/one/sessions',
          Icon: Icons.Terminal,
          exact: true,
          title: 'Active Sessions',
        },
        {
          items: [],
          route: '/web/cluster/one/recordings',
          Icon: Icons.CirclePlay,
          exact: true,
          title: 'Session Recordings',
        },
        {
          items: [],
          route: '/web/cluster/one/audit',
          Icon: Icons.ListThin,
          title: 'Audit Log',
        },
      ],
      route: '',
    },
    {
      title: 'Clusters',
      Icon: Icons.Clusters,

      items: [
        {
          items: [],
          route: '/web/clusters',
          Icon: Icons.EqualizerVertical,
          exact: false,
          title: 'Manage Clusters',
        },
        {
          items: [],
          route: '/web/trust',
          Icon: Icons.Cluster,
          title: 'Trust',
        },
      ],
      route: '',
    },
    {
      title: 'Support',
      Icon: Icons.Question,
      route: 'https://example.com',
      isExternalLink: true,
      items: [],
    },
  ] as Item[],
};
