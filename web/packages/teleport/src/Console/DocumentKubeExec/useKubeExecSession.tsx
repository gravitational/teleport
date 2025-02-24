/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { context, trace } from '@opentelemetry/api';
import { useEffect, useRef, useState } from 'react';

import cfg from 'teleport/config';
import ConsoleContext from 'teleport/Console/consoleContext';
import { useConsoleContext } from 'teleport/Console/consoleContextProvider';
import { DocumentKubeExec } from 'teleport/Console/stores';
import { TermEvent } from 'teleport/lib/term/enums';
import Tty from 'teleport/lib/term/tty';
import type {
  ParticipantMode,
  Session,
  SessionMetadata,
} from 'teleport/services/session';

const tracer = trace.getTracer('TTY');

export default function useKubeExecSession(doc: DocumentKubeExec) {
  const { clusterId, sid, kubeCluster, mode } = doc;
  const ctx = useConsoleContext();
  const ttyRef = useRef<Tty>(null);
  const tty = ttyRef.current as ReturnType<typeof ctx.createTty>;
  const [session, setSession] = useState<Session>(null);
  const [status, setStatus] = useState<Status>('loading');

  function closeDocument() {
    ctx.closeTab(doc);
  }

  function sendKubeExecData(
    namespace: string,
    pod: string,
    container: string,
    command: string,
    isInteractive: boolean
  ): void {
    tty.sendKubeExecData({
      kubeCluster: doc.kubeCluster,
      namespace,
      pod,
      container,
      command,
      isInteractive,
    });
    ctx.updateKubeExecDocument(doc.id, {
      title: `${namespace}/${pod}@${kubeCluster}`,
      kubeNamespace: namespace,
      pod,
      container,
      isInteractive,
      command,
    });
    setStatus('initialized');
  }

  useEffect(() => {
    function initTty(session, mode?: ParticipantMode) {
      tracer.startActiveSpan(
        'initTTY',
        undefined, // SpanOptions
        context.active(),
        span => {
          const tty = ctx.createTty(session, mode);

          // subscribe to tty events to handle connect/disconnects events
          tty.on(TermEvent.CLOSE, () => ctx.closeTab(doc));

          tty.on(TermEvent.CONN_CLOSE, () => {
            setStatus('disconnected');
            ctx.updateKubeExecDocument(doc.id, { status: 'disconnected' });
          });

          tty.on(TermEvent.SESSION, payload => {
            const data = JSON.parse(payload);
            data.session.kind = 'k8s';
            setSession(data.session);
            handleTtyConnect(ctx, data.session, doc.id);
          });

          // assign tty reference so it can be passed down to xterm
          ttyRef.current = tty;
          setSession(session);
          setStatus('waiting-for-exec-data');
          span.end();
        }
      );
    }
    initTty(
      {
        kind: 'k8s',
        resourceName: kubeCluster,
        login: 'root', // standard login value for kube sessions.
        clusterId,
        sid,
      },
      mode
    );

    function teardownTty() {
      ttyRef.current && ttyRef.current.removeAllListeners();
    }

    return teardownTty;
  }, []);

  return {
    tty,
    status,
    session,
    closeDocument,
    sendKubeExecData,
  };
}

function handleTtyConnect(
  ctx: ConsoleContext,
  session: SessionMetadata,
  docId: number
) {
  const { id: sid, cluster_name: clusterId, created } = session;

  const url = cfg.getKubeExecSessionRoute({ clusterId, sid });
  const createdDate = new Date(created);

  ctx.updateKubeExecDocument(docId, {
    status: 'connected',
    url,
    created: createdDate,
    sid,
    clusterId,
  });

  ctx.gotoTab({ url });
}

export type Status =
  | 'loading'
  | 'waiting-for-exec-data'
  | 'initialized'
  | 'disconnected';
