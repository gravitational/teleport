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
import type { Session } from 'teleport/services/session';
import TeleportContext from 'teleport/teleportContext';

import DocumentKubeExec from './DocumentKubeExec';

export default {
  title: 'Teleport/Console/DocumentKubeExec',
};

export const Connected = () => {
  const { ctx, consoleCtx } = getContexts();

  return (
    <DocumentKubeExecWrapper ctx={ctx} consoleCtx={consoleCtx} doc={baseDoc} />
  );
};

export const NotFound = () => {
  const { ctx, consoleCtx } = getContexts();

  const disconnectedDoc = {
    ...baseDoc,
    status: 'disconnected' as const,
  };

  return (
    <DocumentKubeExecWrapper
      ctx={ctx}
      consoleCtx={consoleCtx}
      doc={disconnectedDoc}
    />
  );
};

export const ServerError = () => {
  const { ctx, consoleCtx } = getContexts();

  const noSidDoc = {
    ...baseDoc,
    sid: '',
  };

  return (
    <DocumentKubeExecWrapper ctx={ctx} consoleCtx={consoleCtx} doc={noSidDoc} />
  );
};

type Props = {
  ctx: TeleportContext;
  consoleCtx: ConsoleCtx;
  doc: stores.DocumentKubeExec;
};

const DocumentKubeExecWrapper = ({ ctx, consoleCtx, doc }: Props) => {
  return (
    <ContextProvider ctx={ctx}>
      <TestLayout ctx={consoleCtx}>
        <DocumentKubeExec doc={doc} visible={true} />
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

const baseDoc = {
  kind: 'kubeExec',
  status: 'connected',
  sid: 'sid-value',
  clusterId: 'clusterId-value',
  serverId: 'serverId-value',
  login: 'login-value',
  kubeCluster: 'kubeCluster1',
  kubeNamespace: 'namespace1',
  pod: 'pod1',
  container: '',
  id: 3,
  url: 'fd',
  created: new Date(),
  command: '/bin/bash',
  isInteractive: true,
} as const;

const session: Session = {
  kind: 'k8s',
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
};
