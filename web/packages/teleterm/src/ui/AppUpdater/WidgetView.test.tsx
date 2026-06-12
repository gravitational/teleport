/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { render, screen } from 'design/utils/testing';

import {
  makeUpdateAvailableEvent,
  makeUpdateInfo,
  makeUpdateNotAvailableEvent,
} from './testHelpers';
import { WidgetView } from './WidgetView';

test('download button is available when autoDownload is false', async () => {
  render(
    <WidgetView
      updateEvent={makeUpdateAvailableEvent(
        makeUpdateInfo(false, '18.0.0', 'upgrade'),
        {
          enabled: true,
          version: '18.0.0',
          source: 'highest-compatible',
          options: {
            highestCompatibleVersion: '18.0.0',
            managingClusterUri: undefined,
            clusters: [
              {
                clusterUri: '/cluster/bar',
                toolsAutoUpdate: true,
                toolsVersion: '18.0.0',
                minToolsVersion: '17.0.0-aa',
                otherCompatibleClusters: [],
              },
            ],
            unreachableClusters: [
              { clusterUri: '/clusters/foo', errorMessage: 'NET_ERR' },
            ],
          },
        }
      )}
      clusterGetter={{
        findCluster: () => undefined,
      }}
      platform="darwin"
      onDownload={() => {}}
      onMore={() => {}}
      onInstall={() => {}}
    />
  );

  expect(
    await screen.findByRole('button', { name: 'Download' })
  ).toBeInTheDocument();
});

test('error is displayed when cluster specify incompatible versions', async () => {
  render(
    <WidgetView
      updateEvent={makeUpdateNotAvailableEvent({
        enabled: false,
        reason: 'no-compatible-version',
        options: {
          highestCompatibleVersion: undefined,
          managingClusterUri: undefined,
          clusters: [
            {
              clusterUri: '/cluster/foo',
              toolsAutoUpdate: true,
              toolsVersion: '16.0.0',
              minToolsVersion: '15.0.0-aa',
              otherCompatibleClusters: [],
            },
            {
              clusterUri: '/cluster/bar',
              toolsAutoUpdate: true,
              toolsVersion: '18.0.0',
              minToolsVersion: '17.0.0-aa',
              otherCompatibleClusters: [],
            },
          ],
          unreachableClusters: [],
        },
      })}
      clusterGetter={{
        findCluster: () => undefined,
      }}
      platform="darwin"
      onDownload={() => {}}
      onMore={() => {}}
      onInstall={() => {}}
    />
  );
  expect(
    await screen.findByText('App updates are disabled')
  ).toBeInTheDocument();
  expect(
    await screen.findByText(
      'Your clusters require incompatible client versions. Choose one to enable app updates.'
    )
  ).toBeInTheDocument();
  expect(
    await screen.findByRole('button', { name: 'Resolve' })
  ).toBeInTheDocument();
});
