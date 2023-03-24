import React from 'react';

import { render, screen } from 'design/utils/testing';

import { generatePath, Router } from 'react-router';

import { createMemoryHistory } from 'history';

import { TeleportFeature } from 'teleport/types';
import { NavigationCategory } from 'teleport/Navigation/categories';
import { NavigationItem } from 'teleport/Navigation/NavigationItem';
import { NavigationItemSize } from 'teleport/Navigation/common';

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

    render(
      <Router history={history}>
        <NavigationItem
          feature={new MockFeature()}
          size={NavigationItemSize.Large}
          transitionDelay={100}
          visible={true}
        />
      </Router>
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

    render(
      <Router history={history}>
        <NavigationItem
          feature={new MockFeature()}
          size={NavigationItemSize.Large}
          transitionDelay={100}
          visible={true}
        />
      </Router>
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
