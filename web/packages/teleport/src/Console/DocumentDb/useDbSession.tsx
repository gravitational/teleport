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
import { DocumentDb } from 'teleport/Console/stores';
import { TermEvent } from 'teleport/lib/term/enums';
import Tty, { DbConnectData } from 'teleport/lib/term/tty';
import type { Session, SessionMetadata } from 'teleport/services/session';

const tracer = trace.getTracer('TTY');

export function useDbSession(doc: DocumentDb) {
  const { clusterId, sid } = doc;
  const ctx = useConsoleContext();
  const ttyRef = useRef<Tty>(null);
  const tty = ttyRef.current;
  const [session, setSession] = useState<Session>(null);
  const [status, setStatus] = useState<Status>('loading');

  function closeDocument() {
    ctx.closeTab(doc);
  }

  function sendDbConnectData(data: DbConnectData): void {
    tty.sendDbConnectData(data);
    ctx.updateDbDocument(doc.id, {
      title: `${data.dbUser}@${data.serviceName}`,
    });
    setStatus('initialized');
  }

  useEffect(() => {
    const session: Session = {
      kind: 'db',
      clusterId,
      sid,
      resourceName: doc.name,
    };

    tracer.startActiveSpan(
      'initTTY',
      undefined, // SpanOptions
      context.active(),
      span => {
        const tty = ctx.createTty(session);

        // subscribe to tty events to handle connect/disconnects events
        tty.on(TermEvent.CLOSE, () => ctx.closeTab(doc));

        tty.on(TermEvent.CONN_CLOSE, () => {
          setStatus('disconnected');
        });

        tty.on(TermEvent.SESSION, payload => {
          const data = JSON.parse(payload);
          data.session.kind = 'db';
          setSession(data.session);
          handleTtyConnect(ctx, data.session, doc.id);
        });

        // assign tty reference so it can be passed down to xterm
        ttyRef.current = tty;
        setSession(session);
        setStatus('waiting');
        span.end();
      }
    );

    function teardownTty() {
      ttyRef.current?.removeAllListeners();
    }

    return teardownTty;
  }, []);

  return {
    tty,
    status,
    session,
    closeDocument,
    sendDbConnectData,
  };
}

function handleTtyConnect(
  ctx: ConsoleContext,
  session: SessionMetadata,
  docId: number
) {
  let { id: sid, cluster_name: clusterId, created } = session;

  // The server response could not include the session ID. To avoid breaking the
  // redirect, set a value to the session ID.
  if (sid === '') {
    sid = 'new';
  }

  const url = cfg.getDbSessionRoute({ clusterId, sid });
  const createdDate = new Date(created);

  ctx.updateDbDocument(docId, {
    url,
    created: createdDate,
    sid,
    clusterId,
  });

  ctx.gotoTab({ url });
}

export type Status = 'loading' | 'waiting' | 'initialized' | 'disconnected';
