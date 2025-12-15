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

import { context, trace } from '@opentelemetry/api';
import { useEffect, useState } from 'react';

import { useAttempt } from 'shared/hooks';

import cfg from 'teleport/config';
import { Session } from 'teleport/services/session';
import Ctx from 'teleport/teleportContext';

const tracer = trace.getTracer('userSessions');

export default function useSessions(ctx: Ctx, clusterId: string) {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [attempt, attemptActions] = useAttempt({ isProcessing: true });

  function onRefresh() {
    return tracer.startActiveSpan(
      'onRefresh',
      undefined, // SpanOptions
      context.active(),
      span => {
        return ctx.sshService.fetchSessions(clusterId).then(resp => {
          setSessions(resp);
          span.end();
          return resp;
        });
      }
    );
  }

  useEffect(() => {
    attemptActions.do(() => onRefresh());
  }, [clusterId]);

  return {
    ctx,
    clusterId,
    attempt,
    sessions,
    // moderated is available with any enterprise editions
    showModeratedSessionsCTA: !ctx.isEnterprise,
    showActiveSessionsCTA: !cfg.entitlements.JoinActiveSessions.enabled,
  };
}
