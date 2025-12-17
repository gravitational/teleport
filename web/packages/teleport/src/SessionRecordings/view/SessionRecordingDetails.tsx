import { format } from 'date-fns';
import { type PropsWithChildren } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';

import Flex from 'design/Flex';
import { ChevronLeft } from 'design/Icon';
import { H3 } from 'design/Text';

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
}

export function SessionRecordingDetails({
  children,
  metadata,
  recordingType,
}: PropsWithChildren<SessionRecordingDetailsProps>) {
  const { clusterId } = useStickyClusterId();

  const { icon: Icon, label } = getRecordingTypeInfo(recordingType);

  return (
    <>
      <Flex pl={3} pr={2} justifyContent="space-between">
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
    </>
  );
}

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
