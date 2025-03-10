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

import { MemoryRouter } from 'react-router';

import { ContextProvider } from 'teleport';
import { createTeleportContext } from 'teleport/mocks/contexts';
import useSessions from 'teleport/Sessions/useSessions';

import { sessions } from './fixtures';
import { Sessions } from './Sessions';

export default {
  title: 'Teleport/ActiveSessions',
};

export function Loaded() {
  const props = makeSessionProps({ attempt: { isSuccess: true } });

  return (
    <MemoryRouter>
      <ContextProvider ctx={createTeleportContextWithApiMock()}>
        <Sessions {...props} />
      </ContextProvider>
    </MemoryRouter>
  );
}

export function ActiveSessionsCTA() {
  const props = makeSessionProps({
    attempt: { isSuccess: true },
    showActiveSessionsCTA: true,
  });

  return (
    <MemoryRouter>
      <ContextProvider ctx={createTeleportContextWithApiMock()}>
        <Sessions {...props} />
      </ContextProvider>
    </MemoryRouter>
  );
}

export function ModeratedSessionsCTA() {
  const props = makeSessionProps({
    attempt: { isSuccess: true },
    showModeratedSessionsCTA: true,
  });

  return (
    <MemoryRouter>
      <ContextProvider ctx={createTeleportContextWithApiMock()}>
        <Sessions {...props} />
      </ContextProvider>
    </MemoryRouter>
  );
}

function createTeleportContextWithApiMock() {
  const ctx = createTeleportContext();
  ctx.clusterService.fetchClusters = () =>
    Promise.resolve([
      {
        clusterId: 'im-a-cluster-name',
        lastConnected: new Date('2022-02-02T14:03:00.355597-05:00'),
        connectedText: '2022-02-02 19:03:00',
        status: 'online',
        url: '/web/cluster/im-a-cluster-name/',
        authVersion: '8.0.0-alpha.1',
        publicURL: 'mockurl:3080',
        proxyVersion: '8.0.0-alpha.1',
      },
      {
        clusterId: 'im-a-cluster-name-2',
        lastConnected: new Date('2022-02-02T14:03:00.355597-05:00'),
        connectedText: '2022-02-02 19:03:00',
        status: 'online',
        url: '/web/cluster/im-a-cluster-name-2/',
        authVersion: '8.0.0-alpha.1',
        publicURL: 'mockurl:3081',
        proxyVersion: '8.0.0-alpha.1',
      },
    ]);
  return ctx;
}

const makeSessionProps = (
  overrides: Partial<typeof useSessions> = {}
): ReturnType<typeof useSessions> => {
  return Object.assign(
    {
      ctx: createTeleportContextWithApiMock(),
      clusterId: 'teleport.example.sh',
      sessions,
      attempt: {
        isSuccess: false,
        isProcessing: false,
        isFailed: false,
        message: '',
      },
      showActiveSessionsCTA: false,
      showModeratedSessionsCTA: false,
    },
    overrides
  );
};
