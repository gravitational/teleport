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

import React from 'react';

import { render, screen } from 'design/utils/testing';

import { Server } from 'design/Icon';

import { generatePath, Router } from 'react-router';

import { createMemoryHistory, MemoryHistory } from 'history';

import { act } from '@testing-library/react';

import TeleportContextProvider from 'teleport/TeleportContextProvider';
import TeleportContext from 'teleport/teleportContext';

import { TeleportFeature, NavTitle } from 'teleport/types';
import { NavigationCategory } from 'teleport/Navigation/categories';
import { NavigationItem } from 'teleport/Navigation/NavigationItem';
import { NavigationItemSize } from 'teleport/Navigation/common';
import { makeUserContext } from 'teleport/services/user';
import { LocalNotificationKind } from 'teleport/services/notifications';

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
    icon: Server,
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
    icon: Server,
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

    expect(screen.getByRole('link', { name: 'Users' })).toHaveAttribute(
      'href',
      '/web/cluster/root/feature'
    );
  });

  it('should change the feature link to the leaf cluster when navigating to a leaf cluster', () => {
    render(getNavigationItem({ ctx, history }));

    expect(screen.getByRole('link', { name: 'Users' })).toHaveAttribute(
      'href',
      '/web/cluster/root/feature'
    );

    act(() => history.push('/web/cluster/leaf/feature'));

    expect(screen.getByRole('link', { name: 'Users' })).toHaveAttribute(
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
          kind: LocalNotificationKind.AccessList,
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
