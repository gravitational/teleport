/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { format } from 'date-fns';
import { type PropsWithChildren } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';

import Flex from 'design/Flex';
import { ChevronLeft } from 'design/Icon';
import { H3 } from 'design/Text';
import { CopyButton } from 'shared/components/CopyButton/CopyButton';

import cfg from 'teleport/config';
import type {
  RecordingType,
  SessionRecordingMetadata,
} from 'teleport/services/recordings';
import {
  formatSessionRecordingDuration,
  getRecordingTypeInfo,
} from 'teleport/SessionRecordings/list/RecordingItem';
import useStickyClusterId from 'teleport/useStickyClusterId';

interface SessionRecordingDetailsProps {
  metadata: SessionRecordingMetadata | null;
  recordingType: RecordingType;
  sessionId: string;
}

export function SessionRecordingDetails({
  children,
  metadata,
  recordingType,
  sessionId,
}: PropsWithChildren<SessionRecordingDetailsProps>) {
  const { clusterId } = useStickyClusterId();

  const { icon: Icon, label } = getRecordingTypeInfo(recordingType);

  return (
    <>
      <Flex mt={3} pl={3} pr={2} justifyContent="space-between">
        <BackLink to={cfg.getRecordingsRoute(clusterId)}>
          <ChevronLeft size="small" />
          Back to Session Recordings
        </BackLink>
      </Flex>

      <Flex alignItems="center" gap={3} px={3}>
        <Icon size="small" />

        <H3>{label}</H3>
      </Flex>

      <InfoGrid>
        {children}

        {metadata && (
          <>
            <InfoGridLabel>User</InfoGridLabel>

            <div>{metadata.user}</div>

            <InfoGridLabel>Resource</InfoGridLabel>

            <div>{metadata.resourceName}</div>

            <InfoGridLabel>Duration</InfoGridLabel>

            <div>{formatSessionRecordingDuration(metadata.duration)}</div>

            <InfoGridLabel>Cluster</InfoGridLabel>

            <div>{metadata.clusterName}</div>

            <RecordingTimes metadata={metadata} />
          </>
        )}
      </InfoGrid>

      <Flex alignItems="center" px={3} mt={-2}>
        <InfoGridLabel>ID</InfoGridLabel>

        <SessionId>{sessionId}</SessionId>

        <CopyButton value={sessionId} ml={2} />
      </Flex>
    </>
  );
}

const SessionId = styled.div`
  font-family: ${p => p.theme.fonts.mono};
  color: ${p => p.theme.colors.text.slightlyMuted};
  font-size: ${p => p.theme.fontSizes[1]}px;
  padding-top: 1px;
  margin-left: ${p => p.theme.space[2]}px;
`;

interface RecordingTimesProps {
  metadata: SessionRecordingMetadata;
}

function RecordingTimes({ metadata }: RecordingTimesProps) {
  const startTime = new Date(metadata.startTime * 1000);
  const endTime = new Date(metadata.endTime * 1000);

  return (
    <>
      <InfoGridLabel>Start Time</InfoGridLabel>

      <div>{format(startTime, 'MMM dd, yyyy HH:mm')}</div>

      <InfoGridLabel>End Time</InfoGridLabel>

      <div>{format(endTime, 'MMM dd, yyyy HH:mm')}</div>
    </>
  );
}

const InfoGrid = styled.div`
  display: grid;
  column-gap: ${p => p.theme.space[3]}px;
  row-gap: ${p => p.theme.space[2]}px;
  grid-template-columns: 80px 1fr;
  padding: 0 ${p => p.theme.space[3]}px;
`;

export const InfoGridLabel = styled.div`
  font-weight: bold;
  color: ${p => p.theme.colors.text.slightlyMuted};
`;

const BackLink = styled(Link)`
  color: ${p => p.theme.colors.text.slightlyMuted};
  text-decoration: none;
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: ${p => p.theme.space[2]}px;
`;
