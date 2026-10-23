/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import Flex from 'design/Flex';
import { ShimmerBox } from 'design/ShimmerBox';
import { LoadingSkeleton } from 'shared/components/UnifiedResources/shared/LoadingSkeleton';

import {
  recordingItemFrame,
  RecordingDetails,
  ThumbnailContainer,
} from './RecordingItem';
import type { Density, ViewMode } from './ViewSwitcher';

interface RecordingsListSkeletonProps {
  viewMode: ViewMode;
  density: Density;
}

export function RecordingsListSkeleton({
  viewMode,
  density,
}: RecordingsListSkeletonProps) {
  return (
    <LoadingSkeleton
      count={12}
      Element={<SkeletonItem viewMode={viewMode} density={density} />}
    />
  );
}

function SkeletonItem({ viewMode, density }: RecordingsListSkeletonProps) {
  return (
    <SkeletonContainer
      data-testid="recording-item-skeleton"
      viewMode={viewMode}
      density={density}
    >
      <ThumbnailContainer viewMode={viewMode} density={density}>
        <ShimmerBox height="100%" />
      </ThumbnailContainer>

      <RecordingDetails viewMode={viewMode} density={density}>
        <Flex justifyContent="space-between" gap={3}>
          <ShimmerBox height="14px" width="35%" />
          <ShimmerBox height="12px" width="25%" />
        </Flex>
        <ShimmerBox height="14px" width="55%" />
        <ShimmerBox height="12px" width="45%" mt="auto" />
      </RecordingDetails>
    </SkeletonContainer>
  );
}

const SkeletonContainer =
  styled.div<RecordingsListSkeletonProps>(recordingItemFrame);
