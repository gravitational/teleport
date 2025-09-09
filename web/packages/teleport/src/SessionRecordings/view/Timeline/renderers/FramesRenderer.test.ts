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

import { SessionRecordingThumbnail } from 'teleport/services/recordings';
import type { TimelineRenderContext } from 'teleport/SessionRecordings/view/Timeline/renderers/TimelineCanvasRenderer';

import { FramesRenderer, type LoadedImageResult } from './FramesRenderer';

// Mock the SVG utilities
jest.mock('teleport/SessionRecordings/svg', () => ({
  generateTerminalSVGStyleTag: jest.fn(() => '<style></style>'),
  injectSVGStyles: jest.fn(svg => svg),
  svgToDataURIBase64: jest.fn(svg => `data:image/svg+xml;base64,${svg}`),
}));

beforeEach(() => {
  Object.defineProperty(window, 'devicePixelRatio', {
    writable: true,
    configurable: true,
    value: 1,
  });

  global.OffscreenCanvas = jest.fn().mockImplementation((width, height) => {
    const canvas = document.createElement('canvas');

    canvas.width = width;
    canvas.height = height;

    return canvas;
  });
});

afterEach(() => {
  global.OffscreenCanvas = undefined;
});

function createThumbnail(
  svg: string,
  cols: number,
  rows: number,
  startOffset: number,
  cursorX = 0,
  cursorY = 0
): SessionRecordingThumbnail {
  return {
    svg,
    startOffset,
    endOffset: startOffset + 1000,
    cols,
    rows,
    cursorX,
    cursorY,
    cursorVisible: true,
  };
}

const mockFrames: SessionRecordingThumbnail[] = [
  createThumbnail('<svg>frame1</svg>', 80, 24, 0),
  createThumbnail('<svg>frame2</svg>', 100, 30, 1000),
  createThumbnail('<svg>frame3</svg>', 120, 40, 2000),
  createThumbnail('<svg>frame4</svg>', 80, 24, 3000),
];

async function createRenderer(
  frames: SessionRecordingThumbnail[] = mockFrames,
  timelineWidth = 1000,
  duration = 5000,
  initialHeight = 200,
  eventsHeight = 50,
  frameWidth = 200,
  frameHeight = 50
) {
  const canvas = document.createElement('canvas');
  const ctx = canvas.getContext('2d')!;

  function imageLoader(): Promise<LoadedImageResult> {
    const canvas = new OffscreenCanvas(frameWidth, frameHeight);
    const img = new Image();

    return Promise.resolve({ canvas, img });
  }

  const renderer = new FramesRenderer(
    ctx,
    darkTheme,
    duration,
    frames,
    initialHeight,
    eventsHeight,
    imageLoader
  );

  renderer.setTimelineWidth(timelineWidth);
  renderer.calculate();

  await renderer.loadVisibleFrames(0, timelineWidth);

  const renderContext: TimelineRenderContext = {
    containerWidth: timelineWidth,
    offset: 0,
    containerHeight: initialHeight,
    eventsHeight: eventsHeight,
  };

  renderer._render(renderContext);

  return { ctx, renderer };
}

function getDrawImageCalls(ctx: CanvasRenderingContext2D) {
  return ctx.__getDrawCalls().filter(call => call.type === 'drawImage');
}

test('should calculate frame positions correctly', async () => {
  const { ctx } = await createRenderer();

  const drawImageCalls = getDrawImageCalls(ctx);

  expect(drawImageCalls).toHaveLength(4);

  expect(drawImageCalls[0].props.dx).toBe(24); // left padding
  expect(drawImageCalls[1].props.dx).toBe(224); // 24 + 200
  expect(drawImageCalls[2].props.dx).toBe(424); // 224 + 200
  expect(drawImageCalls[3].props.dx).toBe(624); // last frame positioned next to previous
});

test('should skip frames that would overlap at current zoom', async () => {
  const closeFrames: SessionRecordingThumbnail[] = [
    createThumbnail('<svg>1</svg>', 80, 24, 0),
    createThumbnail('<svg>2</svg>', 80, 24, 500),
    createThumbnail('<svg>3</svg>', 80, 24, 1000),
    createThumbnail('<svg>4</svg>', 80, 24, 1500),
    createThumbnail('<svg>5</svg>', 80, 24, 2000),
  ];

  const { ctx } = await createRenderer(closeFrames, 1000);

  const drawImageCalls = getDrawImageCalls(ctx);

  expect(drawImageCalls).toHaveLength(3);

  expect(drawImageCalls[0].props.dx).toBe(24); // first frame, should be drawn at left padding
  expect(drawImageCalls[1].props.dx).toBe(224); // fourth frame
});

test('should only render visible frames', async () => {
  const { ctx } = await createRenderer(mockFrames, 500);

  const drawImageCalls = getDrawImageCalls(ctx);

  expect(drawImageCalls).toHaveLength(2);

  expect(drawImageCalls[0].props.dx).toBe(24); // left padding
  expect(drawImageCalls[1].props.dx).toBe(224); // 24 + 200
});

test('should calculate the frames at the current zoom level', async () => {
  const { renderer } = await createRenderer();

  const framesAtCurrentZoom = renderer.getFramesAtCurrentZoom();

  expect(framesAtCurrentZoom).toHaveLength(4);

  renderer.setTimelineWidth(400);

  const framesAtNewZoom = renderer.getFramesAtCurrentZoom();

  expect(framesAtNewZoom).toHaveLength(2);
});
