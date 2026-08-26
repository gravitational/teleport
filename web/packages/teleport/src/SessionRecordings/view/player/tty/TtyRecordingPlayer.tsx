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

import {
  resolveColorTokens,
  useDesignSystemContext,
} from '@gravitational/design-system';
import { useSuspenseQuery } from '@tanstack/react-query';
import { ITheme } from '@xterm/xterm';
import { useEffect, useMemo } from 'react';
import { useTheme } from 'styled-components';

import cfg from 'teleport/config';
import { AuthenticatedWebSocket } from 'teleport/lib/AuthenticatedWebSocket';
import { TtyPlayer } from 'teleport/SessionRecordings/view/player/tty/TtyPlayer';

import { RecordingPlayer, type RecordingPlayerProps } from '../RecordingPlayer';
import { decodeTtyEvent } from './decoding';
import { EventType, type TtyEvent } from './types';

interface TtyRecordingPlayerProps extends Omit<
  RecordingPlayerProps<TtyEvent>,
  'decodeEvent' | 'endEventType' | 'player' | 'ws'
> {
  clusterId: string;
  sessionId: string;
  initialCols: number;
  initialRows: number;
}

export function TtyRecordingPlayer({
  clusterId,
  sessionId,
  initialCols,
  initialRows,
  ...rest
}: TtyRecordingPlayerProps) {
  const theme = useTheme();
  const system = useDesignSystemContext();

  const xtermTheme = useMemo<ITheme>(
    () => resolveColorTokens(system, theme.colors.terminal, theme.type),
    [system, theme.colors.terminal, theme.type]
  );

  const {
    data: { player, ws },
  } = useSuspenseQuery({
    queryKey: ['ttyRecordingPlayer', clusterId, 'sessionId', sessionId],
    queryFn: async () => {
      const ws = await createWebSocket(clusterId, sessionId);

      const player = new TtyPlayer(xtermTheme, theme.fonts.mono, {
        cols: initialCols,
        rows: initialRows,
      });

      return {
        ws,
        player,
      };
    },
  });

  useEffect(() => {
    player.updateTheme(xtermTheme);
  }, [player, xtermTheme]);

  return (
    <RecordingPlayer
      {...rest}
      decodeEvent={decodeTtyEvent}
      endEventType={EventType.SessionEnd}
      player={player}
      ws={ws}
    />
  );
}

function createWebSocket(clusterId: string, sessionId: string) {
  return new Promise<WebSocket>((resolve, reject) => {
    const url = cfg.getSessionRecordingPlaybackUrl(clusterId, sessionId);

    const ws = new AuthenticatedWebSocket(url);

    ws.binaryType = 'arraybuffer';

    ws.onopen = () => {
      resolve(ws);
    };

    ws.onerror = () => {
      reject(new Error('Could not connect to the recording'));
    };
  });
}
