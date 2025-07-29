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

import { render, screen, userEvent } from 'design/utils/testing';

import { DetailsView } from './DetailsView';
import {
  makeUpdateAvailableEvent,
  makeUpdateInfo,
  makeUpdateNotAvailableEvent,
} from './testHelpers';

test('download button is available when autoDownload is false', async () => {
  render(
    <DetailsView
      updateEvent={makeUpdateAvailableEvent(makeUpdateInfo(false, '18.0.0'), {
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
      })}
      clusterGetter={{
        findCluster: () => undefined,
      }}
      platform="darwin"
      onCheckForUpdates={() => {}}
      onDownload={() => {}}
      onCancelDownload={() => {}}
      onInstall={() => {}}
      changeManagingCluster={() => {}}
    />
  );
  expect(
    await screen.findByRole('button', { name: 'Download' })
  ).toBeInTheDocument();
});

test('when there is no compatible client version, user needs to select cluster', async () => {
  const changeManagingClusterSpy = jest.fn();
  render(
    <DetailsView
      updateEvent={makeUpdateNotAvailableEvent({
        enabled: false,
        reason: 'no-compatible-version',
        options: {
          highestCompatibleVersion: undefined,
          managingClusterUri: undefined,
          clusters: [
            {
              clusterUri: '/clusters/foo',
              toolsAutoUpdate: true,
              toolsVersion: '16.0.0',
              minToolsVersion: '15.0.0-aa',
              otherCompatibleClusters: [],
            },
            {
              clusterUri: '/clusters/bar',
              toolsAutoUpdate: false,
              toolsVersion: '18.0.0',
              minToolsVersion: '17.0.0-aa',
              otherCompatibleClusters: [],
            },
          ],
          unreachableClusters: [
            { clusterUri: '/clusters/baz', errorMessage: 'NET_ERR' },
          ],
        },
      })}
      clusterGetter={{
        findCluster: () => undefined,
      }}
      platform={'darwin'}
      onCheckForUpdates={() => {}}
      onDownload={() => {}}
      onCancelDownload={() => {}}
      onInstall={() => {}}
      changeManagingCluster={changeManagingClusterSpy}
    />
  );
  expect(
    await screen.findByText(
      'Your clusters require incompatible client versions. To enable app updates, select which cluster should manage them.'
    )
  ).toBeInTheDocument();
  expect(
    await screen.findByText(
      'Unable to retrieve accepted client versions from the cluster baz.'
    )
  ).toBeInTheDocument();
  expect(
    await screen.findByRole('checkbox', {
      name: 'Use the highest compatible version from your clusters',
    })
  ).not.toBeChecked();
  const radioOptions = await screen.findAllByRole('radio');
  expect(radioOptions).toHaveLength(3);

  expect(radioOptions.at(0).closest('label')).toHaveTextContent(
    // The cluster name and the helper text are normally in separate lines.
    'foo16.0.0 client, only compatible with this cluster.'
  );
  expect(radioOptions.at(1).closest('label')).toHaveTextContent(
    'bar18.0.0 client.⚠︎ Cannot provide updates, automatic client tools updates are disabled on this cluster.'
  );
  expect(radioOptions.at(2).closest('label')).toHaveTextContent(
    'baz⚠︎ Cannot provide updates, cluster is unreachable.NET_ERR'
  );

  await userEvent.click(radioOptions.at(0));
  expect(changeManagingClusterSpy).toHaveBeenCalledWith('/clusters/foo');
});
