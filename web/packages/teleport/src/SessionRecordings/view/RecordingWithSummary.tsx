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

import { type RefObject } from 'react';

import { ErrorSuspenseWrapper } from 'shared/components/ErrorSuspenseWrapper/ErrorSuspenseWrapper';

import { type RecordingType } from 'teleport/services/recordings';
import { VALID_RECORDING_TYPES } from 'teleport/services/recordings/recordings';
import { RecordingPlayer } from 'teleport/SessionRecordings/view/RecordingPlayer';
import type { PlayerHandle } from 'teleport/SessionRecordings/view/SshPlayer';
import {
  RecordingPlayerError,
  RecordingPlayerLoading,
  RecordingPlayerWithLoadDuration,
} from 'teleport/SessionRecordings/view/ViewSessionRecordingRoute';

export interface RecordingWithSummaryProps {
  clusterId: string;
  durationMs: number;
  sessionId: string;
  recordingType: RecordingType;
}

interface RecordingWithSummaryPlayerProps {
  clusterId: string;
  durationMs: number;
  sessionId: string;
  recordingType: RecordingType;
  toggleSidebar: () => void;
  playerRef: RefObject<PlayerHandle>;
}

export function RecordingWithSummaryPlayer({
  clusterId,
  durationMs,
  sessionId,
  recordingType,
  toggleSidebar,
  playerRef,
}: RecordingWithSummaryPlayerProps) {
  const validRecordingType = VALID_RECORDING_TYPES.includes(recordingType);
  const validDuration = Number.isInteger(durationMs) && durationMs > 0;

  const shouldFetchSessionDuration = !validRecordingType || !validDuration;

  let player = (
    <RecordingPlayer
      clusterId={clusterId}
      sessionId={sessionId}
      durationMs={durationMs}
      recordingType={recordingType}
      onToggleSidebar={toggleSidebar}
      ref={playerRef}
    />
  );

  if (shouldFetchSessionDuration) {
    return (
      <ErrorSuspenseWrapper
        errorComponent={RecordingPlayerError}
        loadingComponent={RecordingPlayerLoading}
      >
        <RecordingPlayerWithLoadDuration
          clusterId={clusterId}
          sessionId={sessionId}
          onToggleSidebar={toggleSidebar}
        />
      </ErrorSuspenseWrapper>
    );
  }

  return player;
}
