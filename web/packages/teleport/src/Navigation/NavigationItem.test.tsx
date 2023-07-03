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

import { createMemoryHistory } from 'history';

import TeleportContextProvider from 'teleport/TeleportContextProvider';
import TeleportContext from 'teleport/teleportContext';

import { TeleportFeature } from 'teleport/types';
import { NavigationCategory } from 'teleport/Navigation/categories';
import { NavigationItem } from 'teleport/Navigation/NavigationItem';
import { NavigationItemSize } from 'teleport/Navigation/common';
import { makeUserContext } from 'teleport/services/user';

class MockFeature implements TeleportFeature {
  category = NavigationCategory.Resources;

  route = {
    title: 'Some Feature',
    path: '/web/cluster/:clusterId/feature',
    exact: true,
    component: () => <div>Test!</div>,
  };

  hasAccess() {
    return true;
  }

  navigationItem = {
    title: 'Some Feature',
    icon: <div />,
    exact: true,
    getLink(clusterId: string) {
      return generatePath('/web/cluster/:clusterId/feature', { clusterId });
    },
  };
}

describe('navigation items', () => {
  it('should render the feature link correctly', () => {
    const history = createMemoryHistory({
      initialEntries: ['/web/cluster/root/feature'],
    });

    const ctx = new TeleportContext();
    ctx.storeUser.state = makeUserContext({
      cluster: {
        name: 'test-cluster',
        lastConnected: Date.now(),
      },
    });

    render(
      <TeleportContextProvider ctx={ctx}>
        <Router history={history}>
          <NavigationItem
            feature={new MockFeature()}
            size={NavigationItemSize.Large}
            transitionDelay={100}
            visible={true}
          />
        </Router>
      </TeleportContextProvider>
    );

    expect(screen.getByText('Some Feature').closest('a')).toHaveAttribute(
      'href',
      '/web/cluster/root/feature'
    );
  });

  it('should change the feature link to the leaf cluster when navigating to a leaf cluster', () => {
    const history = createMemoryHistory({
      initialEntries: ['/web/cluster/root/feature'],
    });

    const ctx = new TeleportContext();
    ctx.storeUser.state = makeUserContext({
      cluster: {
        name: 'test-cluster',
        lastConnected: Date.now(),
      },
    });

    render(
      <TeleportContextProvider ctx={ctx}>
        <Router history={history}>
          <NavigationItem
            feature={new MockFeature()}
            size={NavigationItemSize.Large}
            transitionDelay={100}
            visible={true}
          />
        </Router>
      </TeleportContextProvider>
    );

    expect(screen.getByText('Some Feature').closest('a')).toHaveAttribute(
      'href',
      '/web/cluster/root/feature'
    );

    history.push('/web/cluster/leaf/feature');

    expect(screen.getByText('Some Feature').closest('a')).toHaveAttribute(
      'href',
      '/web/cluster/leaf/feature'
    );
  });
});
