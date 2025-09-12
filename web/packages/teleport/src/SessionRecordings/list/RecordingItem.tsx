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

import { format } from 'date-fns';
import {
  Suspense,
  useMemo,
  type ComponentType,
  type PropsWithChildren,
  type ReactNode,
} from 'react';
import { ErrorBoundary } from 'react-error-boundary';
import { Link } from 'react-router-dom';
import styled, { css } from 'styled-components';

import Box from 'design/Box';
import Flex from 'design/Flex';
import {
  ArrowRight,
  Database,
  Desktop,
  Kubernetes,
  Server,
  Spinner,
  User,
} from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';
import { rotate360 } from 'design/keyframes';
import Text from 'design/Text';

import cfg from 'teleport/config';
import {
  type Recording,
  type RecordingType,
} from 'teleport/services/recordings';
import { RECORDING_TYPES_WITH_THUMBNAILS } from 'teleport/services/recordings/recordings';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { RecordingThumbnail } from './RecordingThumbnail';
import { Density, ViewMode } from './ViewSwitcher';

// ActionSlot is a function that takes a sessionId and the type of the recording,
// and returns a ReactNode, for placing a button on each session recording.
export type ActionSlot = (sessionId: string, type: RecordingType) => ReactNode;

export interface RecordingItemProps {
  actionSlot?: ActionSlot;
  density: Density;
  recording: Recording;
  thumbnailStyles: string;
  viewMode: ViewMode;
}

export function RecordingItem({
  actionSlot,
  density,
  recording,
  thumbnailStyles,
  viewMode,
}: RecordingItemProps) {
  const { clusterId } = useStickyClusterId();

  const { icon: Icon, label } = useMemo(
    () => getRecordingTypeInfo(recording.recordingType),
    [recording.recordingType]
  );

  const duration = useMemo(
    () => formatSessionRecordingDuration(recording.duration),
    [recording.duration]
  );

  const url = cfg.getPlayerRoute(
    { clusterId, sid: recording.sid },
    {
      recordingType: recording.recordingType,
      durationMs: recording.duration,
    }
  );

  const actions = useMemo(
    () => actionSlot?.(recording.sid, recording.recordingType),
    [actionSlot, recording.sid, recording.recordingType]
  );

  const hasThumbnail = RECORDING_TYPES_WITH_THUMBNAILS.includes(
    recording.recordingType
  );

  return (
    <RecordingItemContainer
      data-testid="recording-item"
      to={recording.playable ? url : '#'}
      target="_blank"
      playable={recording.playable}
      density={density}
      viewMode={viewMode}
    >
      <ThumbnailContainer density={density} viewMode={viewMode}>
        {recording.playable ? (
          hasThumbnail ? (
            <ErrorBoundary
              fallback={
                <ThumbnailError>Thumbnail not available</ThumbnailError>
              }
            >
              <Suspense fallback={<ThumbnailLoading />}>
                <RecordingThumbnail
                  clusterId={clusterId}
                  sessionId={recording.sid}
                  styles={thumbnailStyles}
                />
              </Suspense>
            </ErrorBoundary>
          ) : (
            <ThumbnailError>Thumbnail not available</ThumbnailError>
          )
        ) : (
          <ThumbnailError>
            Non-interactive session, no playback available
          </ThumbnailError>
        )}

        <Duration viewMode={viewMode}>{duration}</Duration>
      </ThumbnailContainer>

      <Flex width="100%">
        <RecordingDetails density={density} viewMode={viewMode}>
          <Flex gap={2} width="100%">
            <Icon size="small" />

            <Text fontWeight="500">{label}</Text>

            <Box flex={1} justifySelf="stretch" alignSelf="stretch" />

            <Text color="text.slightlyMuted" fontSize="small" pr={1}>
              {format(recording.createdDate, 'MMM dd, yyyy HH:mm')}
            </Text>
          </Flex>
          <Flex alignItems="center" gap={2}>
            <ItemSpan>
              <User size="small" color="sessionRecording.user" />

              <Text>{recording.user}</Text>
            </ItemSpan>

            <ArrowRight size="small" color="text.slightlyMuted" />

            <ItemSpan>
              <Server size="small" color="sessionRecording.resource" />

              <Text>{recording.hostname}</Text>
            </ItemSpan>
          </Flex>
          <Box flex={1} justifySelf="stretch" alignSelf="stretch" />
          <Flex alignItems="flex-end" justifyContent="space-between">
            <Text color="text.slightlyMuted" fontSize="12px" fontFamily="mono">
              {recording.sid}
            </Text>

            {actions}
          </Flex>
        </RecordingDetails>
      </Flex>
    </RecordingItemContainer>
  );
}

const RecordingItemContainer = styled(Link).withConfig({
  // We need to specify this when wrapping non-styled components
  shouldForwardProp: prop =>
    !['viewMode', 'density', 'playable'].includes(prop),
})<Pick<RecordingItemProps, 'viewMode' | 'density'> & { playable: boolean }>(
  p => css`
    border: ${p.theme.borders[2]} ${p.theme.colors.interactive.tonal.neutral[0]};
    border-radius: ${p.theme.radii[3]}px;
    overflow: hidden; // Needed to keep the rectangular contents from bleeding out of the round corners.
    display: flex;
    flex-grow: 0;
    cursor: pointer;
    text-decoration: none;
    color: ${p.theme.colors.text.main};
    pointer-events: ${p.playable ? 'all' : 'none'};

    &:hover {
      background: ${p.theme.colors.levels.surface};
      border-color: transparent;
      box-shadow: ${props => props.theme.boxShadow[3]};
    }

    ${p.viewMode === ViewMode.List
      ? css`
          padding: ${p.density === Density.Compact
            ? `${p.theme.space[2]}px`
            : `calc(${p.theme.space[2]}px + 2px) ${p.theme.space[2]}px`};
          gap: ${p.theme.space[3]}px;
        `
      : css`
          flex-direction: column;
        `}
    transition: background-color 150ms, border-color 150ms, box-shadow 150ms;
  `
);

const ThumbnailContainer = styled.div<
  Pick<RecordingItemProps, 'viewMode' | 'density'>
>(
  p => css`
    flex-shrink: 0;
    position: relative;
    overflow: hidden;

    ${p.viewMode === ViewMode.List
      ? css`
          border: 1px solid ${p.theme.colors.interactive.tonal.neutral[0]};
          border-radius: ${p.theme.radii[2]}px;
          height: 100%;
          width: ${p.density === Density.Compact ? '256px' : '320px'};
        `
      : css`
          border-bottom: 1px solid
            ${p.theme.colors.interactive.tonal.neutral[0]};
          flex: 1;
          height: ${p.density === Density.Compact ? '90px' : '120px'};
          width: 100%;
        `}

    ${RecordingItemContainer}:hover & {
      border-color: transparent;
      transition: border-color 150ms;
    }
  `
);

const RecordingDetails = styled.div<
  Pick<RecordingItemProps, 'viewMode' | 'density'>
>(
  p => css`
    display: flex;
    flex-direction: column;
    flex: 1;
    flex-shrink: 0;
    font-size: ${p.density === Density.Compact ? '13px' : '15px'};

    ${p.viewMode === ViewMode.List
      ? css`
          gap: ${p.density === Density.Compact
            ? p.theme.space[1]
            : p.theme.space[2]}px;
          padding-top: ${p.density === Density.Compact
            ? p.theme.space[0]
            : p.theme.space[2]}px;
          padding-right: ${p.theme.space[1]}px;
        `
      : css`
          padding: ${p.theme.space[3]}px ${p.theme.space[2]}px
            ${p.theme.space[2]}px ${p.theme.space[3]}px;
          gap: ${p.theme.space[1]}px;
        `}
  `
);

const Duration = styled.div<Pick<RecordingItemProps, 'viewMode'>>(
  p => css`
    background: rgba(0, 0, 0, 0.5);
    border-radius: ${p.theme.radii[3]}px;
    color: white;
    font-weight: bold;
    position: absolute;
    line-height: 1;
    padding: ${p.theme.space[1]}px ${p.theme.space[2]}px;
    right: ${p.theme.space[2]}px;

    ${p.viewMode === ViewMode.List
      ? css`
          bottom: ${p.theme.space[2]}px;
        `
      : css`
          top: ${p.theme.space[2]}px;
        `}
  `
);

const ItemSpan = styled.span`
  background: ${p => p.theme.colors.spotBackground[0]};
  line-height: 1;
  padding: ${p => p.theme.space[1]}px ${p => p.theme.space[1]}px;
  border-radius: ${p => p.theme.radii[3]}px;
  display: inline-flex;
  align-items: center;
  font-size: 13px;
  gap: ${p => p.theme.space[1]}px;
`;

const Spin = styled(Box)`
  line-height: 12px;
  font-size: 24px;
  animation: ${rotate360} 2s linear infinite;
`;

function ThumbnailLoading() {
  return (
    <Flex alignItems="center" justifyContent="center" height="100%">
      <Spin>
        <Spinner />
      </Spin>
    </Flex>
  );
}

function ThumbnailError({ children }: PropsWithChildren) {
  return (
    <Flex alignItems="center" justifyContent="center" height="100%">
      <Text color="text.slightlyMuted" fontSize="14px">
        {children}
      </Text>
    </Flex>
  );
}

function getRecordingTypeInfo(type: RecordingType): {
  icon: ComponentType<IconProps>;
  label: string;
} {
  switch (type) {
    case 'ssh':
      return {
        icon: Server,
        label: 'SSH Session',
      };

    case 'database':
      return {
        icon: Database,
        label: 'Database Session',
      };

    case 'k8s':
      return {
        icon: Kubernetes,
        label: 'Kubernetes Session',
      };

    case 'desktop':
      return {
        icon: Desktop,
        label: 'Desktop Session',
      };
  }
}

export function formatSessionRecordingDuration(ms: number): string {
  const roundedMs = Math.round(ms / 1000) * 1000;

  const units = [
    { label: 'd', value: Math.floor(roundedMs / 86400000) },
    { label: 'h', value: Math.floor(roundedMs / 3600000) % 24 },
    { label: 'm', value: Math.floor(roundedMs / 60000) % 60 },
    { label: 's', value: Math.floor(roundedMs / 1000) % 60 },
  ];

  const parts = units
    .filter(({ value }) => value > 0)
    .map(({ label, value }) => `${value}${label}`);

  return parts.length > 0 ? parts.join(' ') : '0s';
}
