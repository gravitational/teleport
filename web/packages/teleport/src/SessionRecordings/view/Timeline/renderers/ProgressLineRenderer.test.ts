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

import 'jest-canvas-mock';

import { darkTheme } from 'design/theme';

import { SessionRecordingMetadata } from 'teleport/services/recordings';
import { ProgressLineRenderer } from 'teleport/SessionRecordings/view/Timeline/renderers/ProgressLineRenderer';
import type { TimelineRenderContext } from 'teleport/SessionRecordings/view/Timeline/renderers/TimelineCanvasRenderer';

import { LEFT_PADDING } from '../constants';

const mockMetadata: SessionRecordingMetadata = {
  startTime: 1609459200, // Jan 1, 2021
  endTime: 1609462800, // Jan 1, 2021
  duration: 3600000, // 1 hour in milliseconds
  user: 'testuser',
  resourceName: 'test-server',
  clusterName: 'test-cluster',
  events: [],
  startCols: 80,
  startRows: 24,
  type: 'ssh',
};

function createRenderer(duration: number) {
  const canvas = document.createElement('canvas');
  const ctx = canvas.getContext('2d')!;

  const renderer = new ProgressLineRenderer(ctx, darkTheme, duration);

  return { ctx, renderer };
}

test('should calculate position correctly', () => {
  const { ctx, renderer } = createRenderer(mockMetadata.duration);

  renderer.setTimelineWidth(1000);
  renderer.setCurrentTime(mockMetadata.duration / 2);

  const renderContext: TimelineRenderContext = {
    containerHeight: 100,
    containerWidth: 1000,
    eventsHeight: 50,
    offset: 0,
  };

  renderer.render(renderContext);

  const expectedPosition = 500 + LEFT_PADDING; // Half of 1000 + LEFT_PADDING

  const path = ctx.__getPath();

  expect(path[0].type).toBe('beginPath');
  expect(path[1].type).toBe('moveTo');
  expect(path[1].props.x).toBe(expectedPosition);
  expect(path[1].props.y).toBe(0);
  expect(path[2].type).toBe('lineTo');
  expect(path[2].props.x).toBe(expectedPosition);
  expect(path[2].props.y).toBe(100);

  ctx.__clearPath();

  renderer.setCurrentTime(mockMetadata.duration); // Move to end
  renderer.render(renderContext);

  const endPosition = 1000 + LEFT_PADDING; // End of timeline + LEFT_PADDING
  const newPath = ctx.__getPath();

  expect(newPath[0].type).toBe('beginPath');
  expect(newPath[1].type).toBe('moveTo');
  expect(newPath[1].props.x).toBe(endPosition);
  expect(newPath[1].props.y).toBe(0);
  expect(newPath[2].type).toBe('lineTo');
  expect(newPath[2].props.x).toBe(endPosition);
  expect(newPath[2].props.y).toBe(100);

  ctx.__clearPath();

  renderer.setCurrentTime(0); // Move to start
  renderer.render(renderContext);

  const startPosition = LEFT_PADDING; // Start of timeline (LEFT_PADDING)
  const startPath = ctx.__getPath();

  expect(startPath[0].type).toBe('beginPath');
  expect(startPath[1].type).toBe('moveTo');
  expect(startPath[1].props.x).toBe(startPosition);
  expect(startPath[1].props.y).toBe(0);
  expect(startPath[2].type).toBe('lineTo');
  expect(startPath[2].props.x).toBe(startPosition);
  expect(startPath[2].props.y).toBe(100);
});
