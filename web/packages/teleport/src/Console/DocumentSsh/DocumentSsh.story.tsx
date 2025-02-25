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

import { ContextProvider } from 'teleport';
import * as stores from 'teleport/Console/stores/types';
import { createTeleportContext } from 'teleport/mocks/contexts';
import type { Session } from 'teleport/services/session';
import TeleportContext from 'teleport/teleportContext';

import { TestLayout } from './../Console.story';
import ConsoleCtx from './../consoleContext';
import DocumentSsh from './DocumentSsh';

export const Connected = () => {
  const { ctx, consoleCtx } = getContexts();

  return <DocumentSshWrapper ctx={ctx} consoleCtx={consoleCtx} doc={doc} />;
};

export const NotFound = () => {
  const disconnectedDoc = {
    ...doc,
    status: 'disconnected' as const,
  };
  const { ctx, consoleCtx } = getContexts();

  return (
    <DocumentSshWrapper
      ctx={ctx}
      consoleCtx={consoleCtx}
      doc={disconnectedDoc}
    />
  );
};

export const ServerError = () => {
  const noSidDoc = {
    ...doc,
    sid: '',
  };
  const { ctx, consoleCtx } = getContexts();

  return (
    <DocumentSshWrapper ctx={ctx} consoleCtx={consoleCtx} doc={noSidDoc} />
  );
};

type Props = {
  ctx: TeleportContext;
  consoleCtx: ConsoleCtx;
  doc: stores.DocumentSsh;
};

const DocumentSshWrapper = ({ ctx, consoleCtx, doc }: Props) => {
  return (
    <ContextProvider ctx={ctx}>
      <TestLayout ctx={consoleCtx}>
        <DocumentSsh doc={doc} visible={true} />
      </TestLayout>
    </ContextProvider>
  );
};

function getContexts() {
  const ctx = createTeleportContext();
  const consoleCtx = new ConsoleCtx();
  const tty = consoleCtx.createTty(session);
  tty.connect = () => null;
  consoleCtx.createTty = () => tty;
  consoleCtx.storeUser = ctx.storeUser;

  return { ctx, consoleCtx };
}

export default {
  title: 'Teleport/Console/DocumentSsh',
};

const doc = {
  kind: 'terminal',
  status: 'connected',
  sid: 'sid-value',
  clusterId: 'clusterId-value',
  serverId: 'serverId-value',
  login: 'login-value',
  id: 3,
  url: 'fd',
  created: new Date(),
  latency: {
    client: 123,
    server: 456,
  },
} as const;

const session: Session = {
  kind: 'ssh',
  login: '123',
  sid: '',
  namespace: '',
  created: new Date(),
  durationText: '',
  serverId: '',
  resourceName: '',
  clusterId: '',
  parties: [],
  addr: '1.1.1.1:1111',
  participantModes: ['observer', 'moderator', 'peer'],
  moderated: false,
  command:
    'top -o command -o cpu -o boosts -o cycles -o cow -o user -o vsize -o csw -o threads -o ports -o ppid',
};
