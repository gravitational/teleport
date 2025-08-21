import { format } from 'date-fns';
import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';

import Flex from 'design/Flex';
import {
  ArrowLineLeft,
  ChevronLeft,
  DotsThreeMoreVertical,
  Terminal,
} from 'design/Icon';
import { H3 } from 'design/Text';
import { HoverTooltip } from 'design/Tooltip';

import cfg from 'teleport/config';
import { useSuspenseGetRecordingMetadata } from 'teleport/services/recordings/hooks';
import { formatSessionRecordingDuration } from 'teleport/SessionRecordings/list/RecordingItem';
import { RecordingPlayer } from 'teleport/SessionRecordings/view/RecordingPlayer';

export type SummarySlot = (sessionId: string) => ReactNode;

interface RecordingWithMetadataProps {
  clusterId: string;
  sessionId: string;
  summarySlot?: SummarySlot;
}

const Grid = styled.div<{ sidebarVisible: boolean }>`
  display: grid;
  grid-template-areas: ${p =>
    p.sidebarVisible
      ? `'sidebar recording' 'timeline timeline'`
      : `'recording recording' 'timeline timeline'`};
  grid-template-columns: 1fr 4fr;
  grid-template-rows: 1fr auto;
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
`;

const InfoGrid = styled.div`
  display: grid;
  column-gap: ${p => p.theme.space[3]}px;
  row-gap: ${p => p.theme.space[2]}px;
  grid-template-columns: 80px 1fr;
  padding: 0 ${p => p.theme.space[3]}px;
`;

const Player = styled.div`
  grid-area: recording;
  display: flex;
  justify-content: center;
  align-items: center;
  position: relative;
`;

const TerminalContainer = styled.div`
  overflow: hidden;
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
`;

const Sidebar = styled.div`
  grid-area: sidebar;
  border-right: 1px solid ${p => p.theme.colors.spotBackground[1]};
`;

const Summary = styled.div`
  border-top: 1px solid ${p => p.theme.colors.spotBackground[1]};
`;

const InfoGridLabel = styled.div`
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

const HideSidebarButton = styled.button`
  background: none;
  border: none;
  color: ${p => p.theme.colors.text.slightlyMuted};
  cursor: pointer;
  padding: ${p => p.theme.space[2]}px;
  border-radius: ${p => p.theme.radii[2]}px;
  display: flex;
  align-items: center;

  &:hover {
    background: ${p => p.theme.colors.spotBackground[1]};
  }
`;

const ShowSidebarButton = styled(HideSidebarButton)`
  background: ${p => p.theme.colors.levels.surface};
  border: 1px solid ${p => p.theme.colors.spotBackground[2]};
  border-left: none;
  opacity: 0.6;
  position: absolute;
  top: 50%;
  left: 0;
  transform: translateY(-50%);
  z-index: 10;
  border-radius: 0 ${p => p.theme.radii[3]}px ${p => p.theme.radii[3]}px 0;
  padding: ${p => p.theme.space[2]}px 0;

  &:hover {
    opacity: 1;
  }
`;

export function RecordingWithMetadata({
  clusterId,
  sessionId,
  summarySlot,
}: RecordingWithMetadataProps) {
  const { data } = useSuspenseGetRecordingMetadata({
    clusterId,
    sessionId,
  });

  const [sidebarVisible, setSidebarVisible] = useState(true);

  const toggleSidebar = useCallback(() => {
    setSidebarVisible(prev => !prev);
  }, []);

  const summary = useMemo(
    () => (summarySlot ? summarySlot(sessionId) : null),
    [summarySlot, sessionId]
  );

  const startTime = new Date(data.metadata.startTime * 1000);
  const endTime = new Date(data.metadata.endTime * 1000);

  return (
    <Grid sidebarVisible={sidebarVisible}>
      <Player>
        <TerminalContainer>
          <RecordingPlayer
            clusterId={clusterId}
            sessionId={sessionId}
            durationMs={data.metadata.duration}
            recordingType="ssh"
          />
        </TerminalContainer>
      </Player>

      {sidebarVisible ? (
        <Sidebar>
          <Flex flexDirection="column" gap={4} pt={3}>
            <Flex pl={3} pr={2} justifyContent="space-between">
              <BackLink to={cfg.getRecordingsRoute(clusterId)}>
                <ChevronLeft size="small" />
                Back to Session Recordings
              </BackLink>

              <HoverTooltip tipContent="Hide Sidebar">
                <HideSidebarButton onClick={toggleSidebar}>
                  <ArrowLineLeft size="small" />
                </HideSidebarButton>
              </HoverTooltip>
            </Flex>

            <Flex alignItems="center" gap={3} px={3}>
              <Terminal />

              <H3>SSH Session</H3>
            </Flex>

            <InfoGrid>
              <InfoGridLabel>User</InfoGridLabel>

              <div>{data.metadata.user}</div>

              <InfoGridLabel>Resource</InfoGridLabel>

              <div>{data.metadata.resource}</div>

              <InfoGridLabel>Duration</InfoGridLabel>

              <div>
                {formatSessionRecordingDuration(data.metadata.duration)}
              </div>

              <InfoGridLabel>Cluster</InfoGridLabel>

              <div>{data.metadata.clusterName}</div>

              <InfoGridLabel>Start Time</InfoGridLabel>

              <div>{format(startTime, 'MMM dd, yyyy HH:mm')}</div>

              <InfoGridLabel>End Time</InfoGridLabel>

              <div>{format(endTime, 'MMM dd, yyyy HH:mm')}</div>
            </InfoGrid>

            {summary && <Summary>{summary}</Summary>}
          </Flex>
        </Sidebar>
      ) : (
        <HoverTooltip tipContent="Show Sidebar" placement="right">
          <ShowSidebarButton onClick={toggleSidebar}>
            <DotsThreeMoreVertical size="large" />
          </ShowSidebarButton>
        </HoverTooltip>
      )}
    </Grid>
  );
}
