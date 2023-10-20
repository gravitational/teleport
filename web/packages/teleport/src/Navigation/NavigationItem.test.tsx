/*
Copyright 2023 Gravitational, Inc.

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

import { render, screen } from 'design/utils/testing';

import { generatePath, Router } from 'react-router';

import { createMemoryHistory, MemoryHistory } from 'history';

import TeleportContextProvider from 'teleport/TeleportContextProvider';
import TeleportContext from 'teleport/teleportContext';

import { TeleportFeature, NavTitle } from 'teleport/types';
import { NavigationCategory } from 'teleport/Navigation/categories';
import { NavigationItem } from 'teleport/Navigation/NavigationItem';
import { NavigationItemSize } from 'teleport/Navigation/common';
import { makeUserContext } from 'teleport/services/user';
import { NotificationKind } from 'teleport/stores/storeNotifications';

class MockUserFeature implements TeleportFeature {
  category = NavigationCategory.Resources;

  route = {
    title: 'Users',
    path: '/web/cluster/:clusterId/feature',
    exact: true,
    component: () => <div>Test!</div>,
  };

  hasAccess() {
    return true;
  }

  navigationItem = {
    title: NavTitle.Users,
    icon: <div />,
    exact: true,
    getLink(clusterId: string) {
      return generatePath('/web/cluster/:clusterId/feature', { clusterId });
    },
  };
}

class MockAccessListFeature implements TeleportFeature {
  category = NavigationCategory.Resources;

  route = {
    title: 'Users',
    path: '/web/cluster/:clusterId/feature',
    exact: true,
    component: () => <div>Test!</div>,
  };

  hasAccess() {
    return true;
  }

  navigationItem = {
    title: NavTitle.AccessLists,
    icon: <div />,
    exact: true,
    getLink(clusterId: string) {
      return generatePath('/web/cluster/:clusterId/feature', { clusterId });
    },
  };
}

describe('navigation items', () => {
  let ctx: TeleportContext;
  let history: MemoryHistory;

  beforeEach(() => {
    history = createMemoryHistory({
      initialEntries: ['/web/cluster/root/feature'],
    });

    ctx = new TeleportContext();
    ctx.storeUser.state = makeUserContext({
      cluster: {
        name: 'test-cluster',
        lastConnected: Date.now(),
      },
    });
  });

  it('should render the feature link correctly', () => {
    render(getNavigationItem({ ctx, history }));

    expect(screen.getByText('Users').closest('a')).toHaveAttribute(
      'href',
      '/web/cluster/root/feature'
    );
  });

  it('should change the feature link to the leaf cluster when navigating to a leaf cluster', () => {
    render(getNavigationItem({ ctx, history }));

    expect(screen.getByText('Users').closest('a')).toHaveAttribute(
      'href',
      '/web/cluster/root/feature'
    );

    history.push('/web/cluster/leaf/feature');

    expect(screen.getByText('Users').closest('a')).toHaveAttribute(
      'href',
      '/web/cluster/leaf/feature'
    );
  });

  it('rendeirng of attention dot for access list', () => {
    const { rerender } = render(
      getNavigationItem({ ctx, history, feature: new MockAccessListFeature() })
    );

    expect(
      screen.queryByTestId('nav-item-attention-dot')
    ).not.toBeInTheDocument();

    // Add in some notifications
    ctx.storeNotifications.setNotifications([
      {
        item: {
          kind: NotificationKind.AccessList,
          resourceName: 'banana',
          route: '',
        },
        id: 'abc',
        date: new Date(),
      },
    ]);

    rerender(
      getNavigationItem({ ctx, history, feature: new MockAccessListFeature() })
    );

    expect(screen.getByTestId('nav-item-attention-dot')).toBeInTheDocument();
  });
});

function getNavigationItem({
  ctx,
  history,
  feature = new MockUserFeature(),
}: {
  ctx: TeleportContext;
  history: MemoryHistory;
  feature?: TeleportFeature;
}) {
  return (
    <TeleportContextProvider ctx={ctx}>
      <Router history={history}>
        <NavigationItem
          feature={feature}
          size={NavigationItemSize.Large}
          transitionDelay={100}
          visible={true}
        />
      </Router>
    </TeleportContextProvider>
  );
}
