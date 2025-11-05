/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { ContextProvider } from 'teleport';
import { TestLayout } from 'teleport/Console/Console.story';
import ConsoleCtx from 'teleport/Console/consoleContext';
import * as stores from 'teleport/Console/stores/types';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { ResourcesResponse, UnifiedResource } from 'teleport/services/agents';
import TeleportContext from 'teleport/teleportContext';

import { DocumentDb } from './DocumentDb';

export default {
  title: 'Teleport/Console/DocumentDb',
};

export const Connect = () => {
  const { ctx, consoleCtx } = getContexts(
    Promise.resolve({
      agents: [
        {
          kind: 'db',
          name: 'mydb',
          description: '',
          type: '',
          protocol: 'postgres',
          labels: [],
          names: ['users', 'orders'],
          users: ['alice', 'bob'],
          roles: ['reader', 'all'],
          hostname: '',
          supportsInteractive: true,
        },
      ],
    })
  );

  return <DocumentDbWrapper ctx={ctx} consoleCtx={consoleCtx} doc={baseDoc} />;
};

export const ConnectWithEmptyDatabaseName = () => {
  const { ctx, consoleCtx } = getContexts(
    Promise.resolve({
      agents: [
        {
          kind: 'db',
          name: 'mydb',
          description: '',
          type: '',
          protocol: 'mysql',
          labels: [],
          names: ['users', 'orders'],
          users: ['alice', 'bob'],
          roles: ['reader', 'all'],
          hostname: '',
          supportsInteractive: true,
        },
      ],
    })
  );

  return <DocumentDbWrapper ctx={ctx} consoleCtx={consoleCtx} doc={baseDoc} />;
};

export const ConnectWithoutAllowedDatabaseNames = () => {
  const { ctx, consoleCtx } = getContexts(
    Promise.resolve({
      agents: [
        {
          kind: 'db',
          name: 'mydb',
          description: '',
          type: '',
          protocol: 'mysql',
          labels: [],
          users: ['alice', 'bob'],
          roles: ['reader', 'all'],
          hostname: '',
          supportsInteractive: true,
        },
      ],
    })
  );

  return <DocumentDbWrapper ctx={ctx} consoleCtx={consoleCtx} doc={baseDoc} />;
};

export const ConnectWithDatabaseNamesUnsupported = () => {
  const { ctx, consoleCtx } = getContexts(
    Promise.resolve({
      agents: [
        {
          kind: 'db',
          name: 'mydb',
          description: '',
          type: '',
          // as of writing, we don't even have a Cassandra web client, but we
          // should test that protocols without database name support render
          // without an input for database name.
          protocol: 'cassandra',
          labels: [],
          users: ['alice', 'bob'],
          roles: ['reader', 'all'],
          hostname: '',
          supportsInteractive: true,
        },
      ],
    })
  );

  return <DocumentDbWrapper ctx={ctx} consoleCtx={consoleCtx} doc={baseDoc} />;
};

export const ConnectWithoutRoles = () => {
  const { ctx, consoleCtx } = getContexts(
    Promise.resolve({
      agents: [
        {
          kind: 'db',
          name: 'mydb',
          description: '',
          type: '',
          protocol: 'postgres',
          labels: [],
          names: ['users', 'orders'],
          users: ['alice', 'bob'],
          hostname: '',
          supportsInteractive: true,
        },
      ],
    })
  );

  return <DocumentDbWrapper ctx={ctx} consoleCtx={consoleCtx} doc={baseDoc} />;
};

export const ConnectWithoutValues = () => {
  const { ctx, consoleCtx } = getContexts(
    Promise.resolve({
      agents: [
        {
          kind: 'db',
          name: 'mydb',
          description: '',
          type: '',
          protocol: 'postgres',
          labels: [],
          hostname: '',
          supportsInteractive: true,
        },
      ],
    })
  );

  return <DocumentDbWrapper ctx={ctx} consoleCtx={consoleCtx} doc={baseDoc} />;
};

export const ConnectWithWildcards = () => {
  const { ctx, consoleCtx } = getContexts(
    Promise.resolve({
      agents: [
        {
          kind: 'db',
          name: 'mydb',
          description: '',
          type: '',
          protocol: 'postgres',
          labels: [],
          names: ['postgres', '*'],
          users: ['*'],
          roles: ['*'],
          hostname: '',
          supportsInteractive: true,
        },
      ],
    })
  );

  return <DocumentDbWrapper ctx={ctx} consoleCtx={consoleCtx} doc={baseDoc} />;
};

export const ConnectWithAutoUserProvisioning = () => {
  const { ctx, consoleCtx } = getContexts(
    Promise.resolve({
      agents: [
        {
          kind: 'db',
          name: 'mydb',
          description: '',
          type: '',
          protocol: 'postgres',
          labels: [],
          names: ['postgres', '*'],
          users: ['alice'],
          roles: ['readonly', 'read-write'],
          autoUsersEnabled: true,
          hostname: '',
          supportsInteractive: true,
        },
      ],
    })
  );

  return <DocumentDbWrapper ctx={ctx} consoleCtx={consoleCtx} doc={baseDoc} />;
};

export const NotFound = () => {
  const { ctx, consoleCtx } = getContexts(Promise.resolve({ agents: [] }));

  return <DocumentDbWrapper ctx={ctx} consoleCtx={consoleCtx} doc={baseDoc} />;
};

export const Loading = () => {
  // Resources list that never resolves.
  const { ctx, consoleCtx } = getContexts(new Promise(() => {}));

  return <DocumentDbWrapper ctx={ctx} consoleCtx={consoleCtx} doc={baseDoc} />;
};

export const LoadError = () => {
  const { ctx, consoleCtx } = getContexts(
    Promise.reject(new Error('failed to fetch'))
  );

  return <DocumentDbWrapper ctx={ctx} consoleCtx={consoleCtx} doc={baseDoc} />;
};

type Props = {
  ctx: TeleportContext;
  consoleCtx: ConsoleCtx;
  doc: stores.DocumentDb;
};

const DocumentDbWrapper = ({ ctx, consoleCtx, doc }: Props) => {
  return (
    <ContextProvider ctx={ctx}>
      <TestLayout ctx={consoleCtx}>
        <DocumentDb doc={doc} visible={true} />
      </TestLayout>
    </ContextProvider>
  );
};

function getContexts(resources: Promise<ResourcesResponse<UnifiedResource>>) {
  const ctx = createTeleportContext();
  ctx.resourceService.fetchUnifiedResources = () => resources;

  const consoleCtx = new ConsoleCtx();
  const tty = consoleCtx.createTty({
    kind: 'db',
    login: '123',
    sid: '456',
    namespace: '',
    created: new Date(),
    durationText: '',
    serverId: '',
    resourceName: '',
    clusterId: '',
    parties: [],
    addr: '',
    participantModes: [],
    moderated: false,
    command: '/bin/bash',
  });
  tty.connect = () => null;
  consoleCtx.createTty = () => tty;
  consoleCtx.storeUser = ctx.storeUser;

  return { ctx, consoleCtx };
}

const baseDoc = {
  kind: 'db',
  status: 'connected',
  sid: 'sid-value',
  clusterId: 'clusterId-value',
  serverId: 'serverId-value',
  name: 'mydb',
  url: 'fd',
  created: new Date(),
} as const;
