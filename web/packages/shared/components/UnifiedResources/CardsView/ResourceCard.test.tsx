/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { mockResizeObserver } from 'jsdom-testing-mocks';

import { render, screen } from 'design/utils/testing';

import { makeUnifiedResourceViewItemKube } from '../shared/viewItemsFactory';
import { PinningSupport, UnifiedResourceKube } from '../types';
import { ResourceCard } from './ResourceCard';

mockResizeObserver();

test('renders a Kubernetes cluster description as a subtitle', () => {
  const viewItem = setupKubeCard('Production cluster for the payments team');

  expect(
    screen.getByText('Production cluster for the payments team')
  ).toBeInTheDocument();
  expect(viewItem.listViewProps.description).toBe(
    'Production cluster for the payments team'
  );
});

test('preserves the existing subtitle when a Kubernetes cluster has no description', () => {
  setupKubeCard();

  expect(screen.getByText('Kubernetes')).toBeInTheDocument();
  expect(
    screen.queryByText('Production cluster for the payments team')
  ).not.toBeInTheDocument();
});

function setupKubeCard(description?: string) {
  const resource: UnifiedResourceKube = {
    kind: 'kube_cluster',
    name: 'production-us-east',
    description,
    labels: [],
  };

  const viewItem = makeUnifiedResourceViewItemKube(resource, {
    ActionButton: <button>Connect</button>,
  });

  render(
    <ResourceCard
      pinned={false}
      pinResource={() => {}}
      selectResource={() => {}}
      selected={false}
      pinningSupport={PinningSupport.Supported}
      onShowStatusInfo={() => {}}
      showingStatusInfo={false}
      viewItem={viewItem}
      visibleInputFields={{
        checkbox: false,
        pin: false,
        copy: false,
        hoverState: false,
      }}
    />
  );

  return viewItem;
}
