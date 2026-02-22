/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
import { useConsoleContext } from 'teleport/Console/consoleContextProvider';
import ConsoleContext from 'teleport/Console/consoleContext';
import { DocumentKubeTUI } from 'teleport/Console/stores/types';
import Tty from 'teleport/lib/term/tty';
import { TermEvent } from 'teleport/lib/term/enums';
import { Session as SessionMetadata } from 'teleport/services/session';

const tracer = trace.getTracer('kube-tui-session');

export type Status = 'loading' | 'initialized' | 'disconnected';

export default function useKubeTUISession(doc: DocumentKubeTUI) {
  const { clusterId, sid, kubeCluster } = doc;
  const ctx = useConsoleContext();
  const ttyRef = useRef<Tty>(null);
  const tty = ttyRef.current as ReturnType<typeof ctx.createTty>;
  const [session, setSession] = useState<SessionMetadata>(null);
  const [status, setStatus] = useState<Status>('loading');

  function closeDocument() {
    ctx.closeTab(doc);
  }

  useEffect(() => {
    function initTty(session: { kind: string; resourceName: string; login: string; clusterId: string; sid?: string }) {
      tracer.startActiveSpan(
        'initTTY',
        undefined,
        context.active(),
        span => {
          const tty = ctx.createTty(session);

          tty.on(TermEvent.CLOSE, () => ctx.closeTab(doc));

          tty.on(TermEvent.CONN_CLOSE, () => {
            setStatus('disconnected');
            ctx.updateKubeTUIDocument(doc.id, { status: 'disconnected' });
          });

          tty.on(TermEvent.SESSION, payload => {
            const data = JSON.parse(payload);
            data.session.kind = 'kubeTUI';
            setSession(data.session);

            handleTtyConnect(ctx, data.session, doc.id);
          });

          ttyRef.current = tty;
          setStatus('initialized');
          span.end();
        }
      );
    }

    initTty({
      kind: 'kubeTUI',
      resourceName: kubeCluster,
      login: 'root',
      clusterId,
      sid,
    });

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
  };
}

function handleTtyConnect(
  ctx: ConsoleContext,
  session: SessionMetadata,
  docId: number
) {
  const { id: sid, cluster_name: clusterId, created } = session;

  const url = cfg.getKubeTUIConnectRoute({ clusterId, kubeId: sid });
  const createdDate = new Date(created);

  ctx.updateKubeTUIDocument(docId, {
    status: 'connected',
    url,
    created: createdDate,
    sid,
    clusterId,
  });

  ctx.gotoTab({ url });
}
