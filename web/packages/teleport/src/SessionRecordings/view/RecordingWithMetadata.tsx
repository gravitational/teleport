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

import styled from 'styled-components';

import { type RecordingType } from 'teleport/services/recordings';
import { DEFAULT_SIDEBAR_WIDTH } from 'teleport/SessionRecordings/view/SidebarResizeHandle';

export interface RecordingWithMetadataProps {
  clusterId: string;
  recordingType: RecordingType;
  sessionId: string;
}

function getGridTemplateAreas(hasSummary: boolean, sidebarHidden: boolean) {
  if (hasSummary) {
    return sidebarHidden
      ? `'recording recording' 'sidebar timeline'`
      : `'sidebar recording' 'sidebar timeline'`;
  }

  return sidebarHidden
    ? `'recording recording' 'timeline timeline'`
    : `'sidebar recording' 'timeline timeline'`;
}

export const SessionRecordingGrid = styled.div<{
  hasSummary?: boolean;
  sidebarHidden: boolean;
  sidebarWidth?: number;
}>`
  background: ${p => p.theme.colors.levels.sunken};
  display: grid;
  grid-template-areas: ${p =>
    getGridTemplateAreas(!!p.hasSummary, p.sidebarHidden)};
  grid-template-columns: ${p =>
    p.sidebarHidden
      ? '1fr'
      : `${p.sidebarWidth ?? DEFAULT_SIDEBAR_WIDTH}px 1fr`};
  grid-template-rows: 1fr auto;
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
`;

export const PlayerContainer = styled.div`
  grid-area: recording;
  display: flex;
  justify-content: center;
  align-items: center;
  position: relative;
`;

export const SidebarContainer = styled.div`
  grid-area: sidebar;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  gap: ${p => p.theme.space[3]}px;
  min-height: 0;
  height: 100%;
  position: relative;
`;

export const TimelineContainer = styled.div`
  grid-area: timeline;
`;
