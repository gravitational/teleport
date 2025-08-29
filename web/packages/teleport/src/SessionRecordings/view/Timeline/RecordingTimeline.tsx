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
  useCallback,
  useEffect,
  useImperativeHandle,
  useRef,
  type MouseEvent,
  type RefObject,
} from 'react';
import styled, { useTheme } from 'styled-components';
import { useDebounceCallback } from 'usehooks-ts';

import { useLocalStorage } from 'shared/hooks/useLocalStorage';

import type {
  SessionRecordingMetadata,
  SessionRecordingThumbnail,
} from 'teleport/services/recordings';
import { KeysEnum } from 'teleport/services/storageService';

import { TimelineRenderer } from './renderers/TimelineRenderer';
import { ResizeHandle } from './ResizeHandle';
import { useCursor } from './useCursor';
import {
  calculateAutoScrollOffset,
  calculateNextUserControlled,
  shouldAutoScroll,
} from './utils';

interface RecordingTimelineProps {
  frames: SessionRecordingThumbnail[];
  metadata: SessionRecordingMetadata;
  onTimeChange: (time: number) => void;
  showAbsoluteTime: boolean;
  ref: RefObject<RecordingTimelineHandle>;
}

const Container = styled.div`
  position: relative;
  width: 100%;
  overflow: hidden;
`;

const Canvas = styled.canvas`
  background: ${p => p.theme.colors.sessionRecordingTimeline.background};
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  width: 100%;
  will-change: transform;
  transform: translateZ(0); // force the browser to use the GPU for rendering
`;

const Cursor = styled.div`
  position: absolute;
  top: 0;
  bottom: 0;
  width: 1px;
  background: ${p => p.theme.colors.sessionRecordingTimeline.cursor};
  z-index: 1;
  pointer-events: none;
  will-change: transform;
  display: none;
`;

export interface RecordingTimelineHandle {
  moveToTime: (time: number, force?: boolean) => void;
}

const defaultHeight = 235;
const minHeight = 200;
const maxHeight = 500;

function scaleCanvas(
  canvas: HTMLCanvasElement,
  width: number,
  height: number,
  options?: CanvasRenderingContext2DSettings
) {
  const dpr = window.devicePixelRatio || 1;

  canvas.width = width * dpr;
  canvas.height = height * dpr;

  canvas.style.width = `${width}px`;
  canvas.style.height = `${height}px`;

  const ctx = canvas.getContext('2d', options);

  if (!ctx) {
    throw new Error('Failed to get canvas context');
  }

  ctx.scale(dpr, dpr);

  return ctx;
}

export function RecordingTimeline({
  frames,
  metadata,
  onTimeChange,
  showAbsoluteTime,
  ref,
}: RecordingTimelineProps) {
  const theme = useTheme();

  const [height, setHeight] = useLocalStorage(
    KeysEnum.SESSION_RECORDING_TIMELINE_HEIGHT,
    defaultHeight
  );

  const timeRef = useRef(0);

  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const cursorRef = useRef<HTMLDivElement | null>(null);
  const ctxRef = useRef<CanvasRenderingContext2D | null>(null);
  const rendererRef = useRef<TimelineRenderer | null>(null);

  useEffect(() => {
    if (!containerRef.current || !canvasRef.current) {
      return;
    }

    const { clientWidth: containerWidth, clientHeight: containerHeight } =
      containerRef.current;

    if (rendererRef.current) {
      rendererRef.current.destroy();
      rendererRef.current = null;
    }

    ctxRef.current = scaleCanvas(
      canvasRef.current,
      containerWidth,
      containerHeight,
      {
        alpha: false,
        desynchronized: true,
        willReadFrequently: true,
      }
    );

    ctxRef.current.imageSmoothingEnabled = false;
    ctxRef.current.textRendering = 'optimizeLegibility';

    ctxRef.current.fillStyle = theme.colors.sessionRecordingTimeline.background;
    ctxRef.current.fillRect(0, 0, containerWidth, containerHeight);

    rendererRef.current = new TimelineRenderer(
      ctxRef.current,
      metadata,
      frames,
      theme,
      containerWidth,
      containerHeight
    );
  }, [frames, metadata, theme]);

  useEffect(() => {
    if (!rendererRef.current) {
      return;
    }

    rendererRef.current.setShowAbsoluteTime(showAbsoluteTime);
  }, [showAbsoluteTime]);

  useEffect(() => {
    if (!canvasRef.current) {
      return;
    }

    function handleWheel(event: WheelEvent) {
      if (!rendererRef.current) {
        return;
      }

      event.preventDefault();

      const renderer = rendererRef.current;

      if (event.metaKey || event.ctrlKey) {
        const ZOOM_SENSITIVITY = 0.002;
        const deltaZoom = -event.deltaY * ZOOM_SENSITIVITY;

        renderer.accumulateZoom(event.clientX, deltaZoom);
      } else if (event.shiftKey && event.deltaY !== 0) {
        renderer.accumulatePan(event.deltaY);
      } else if (!event.shiftKey && event.deltaX !== 0) {
        renderer.accumulatePan(event.deltaX);
      }
    }

    const container = canvasRef.current;

    container.addEventListener('wheel', handleWheel, { passive: false });

    return () => {
      container.removeEventListener('wheel', handleWheel);
    };
  }, []);

  const updateImages = useCallback(() => {
    if (!rendererRef.current) {
      return;
    }

    rendererRef.current.recreateImages();
    rendererRef.current.render();
  }, []);

  const debouncedUpdateImages = useDebounceCallback(updateImages);

  const handleHeightChange = useCallback(
    (newHeight: number) => {
      if (!canvasRef.current || !ctxRef.current || !rendererRef.current) {
        return;
      }

      setHeight(newHeight);

      const dpr = window.devicePixelRatio || 1;

      canvasRef.current.height = newHeight * dpr;

      canvasRef.current.style.height = `${newHeight}px`;

      ctxRef.current.scale(dpr, dpr);

      rendererRef.current.setHeight(newHeight);

      // the thumbnails need to be recreated on height change (as they are already rendered at
      // the previous height on an offscreen canvas), so we need to call recreateVisibleImages so
      // that we don't recreate all of them at once and hang the UI
      rendererRef.current.recreateVisibleImages();

      // debounce updating the rest of the images to avoid hanging the UI
      // when resizing the timeline to a larger height
      debouncedUpdateImages();
    },
    [debouncedUpdateImages, setHeight]
  );

  const {
    handleMouseEnter,
    handleMouseLeave,
    handleMouseMove,
    isInteractingRef,
  } = useCursor({
    containerRef,
    cursorRef,
  });

  const handleClick = useCallback(
    (event: MouseEvent<HTMLCanvasElement>) => {
      if (!canvasRef.current || !rendererRef.current) {
        return;
      }

      const time = rendererRef.current.getTimeAtX(
        event.clientX - canvasRef.current.getBoundingClientRect().left
      );

      onTimeChange(time);
    },
    [onTimeChange]
  );

  const moveToTime = useCallback(
    (time: number, force?: boolean) => {
      if (!rendererRef.current || !containerRef.current) {
        return;
      }

      timeRef.current = time;

      const renderer = rendererRef.current;
      const containerWidth = containerRef.current.offsetWidth;

      // Calculate timeline width based on duration and zoom
      const pixelsPerMs = 0.1;
      const baseTimelineWidth = metadata.duration * pixelsPerMs;
      const timelineWidth = baseTimelineWidth * renderer.getZoom();

      // Calculate time position on the timeline
      const timePosition = (time / metadata.duration) * timelineWidth;
      const relativePosition = timePosition + renderer.getOffset();

      // Check if we should auto-scroll
      const isUserControlled = renderer.getIsUserControlled();
      const newUserControlled = calculateNextUserControlled(
        relativePosition,
        containerWidth,
        isInteractingRef.current,
        isUserControlled
      );

      if (newUserControlled !== isUserControlled) {
        renderer.setIsUserControlled(newUserControlled);
      }

      if (
        force ||
        shouldAutoScroll(
          relativePosition,
          containerWidth,
          isInteractingRef.current,
          newUserControlled
        )
      ) {
        const newOffset = calculateAutoScrollOffset(
          timePosition,
          relativePosition,
          containerWidth,
          timelineWidth,
          force
        );

        renderer.setOffset(newOffset);
      }

      renderer.setCurrentTime(time);
    },
    [metadata.duration, isInteractingRef]
  );

  useImperativeHandle(ref, () => ({
    moveToTime,
  }));

  const previousContainerWidth = useRef(0);

  useEffect(() => {
    if (!containerRef.current) {
      return;
    }

    // not using useResizeObserver here as it returns a RefCallback instead of a RefObject
    const observer = new ResizeObserver(([box]) => {
      if (!canvasRef.current || !rendererRef.current) {
        return;
      }

      const { blockSize: height, inlineSize: width } = box.contentBoxSize[0];

      const canvas = canvasRef.current;
      if (!ctxRef.current) {
        return;
      }

      const imageData = ctxRef.current.getImageData(
        0,
        0,
        canvas.width,
        canvas.height
      );

      const dpr = window.devicePixelRatio || 1;

      canvas.width = width * dpr;
      canvas.height = height * dpr;

      canvas.style.width = `${width}px`;
      canvas.style.height = `${height}px`;

      ctxRef.current.scale(dpr, dpr);
      ctxRef.current.putImageData(imageData, 0, 0);

      const sameWidth = width === previousContainerWidth.current;

      if (!sameWidth) {
        previousContainerWidth.current = width;

        rendererRef.current.setWidth(width);
      }
    });

    observer.observe(containerRef.current);

    return () => {
      observer.disconnect();
    };
  }, []);

  return (
    <Container style={{ height: `${height}px` }} ref={containerRef}>
      <ResizeHandle
        onChange={handleHeightChange}
        height={height}
        defaultHeight={defaultHeight}
        minHeight={minHeight}
        maxHeight={maxHeight}
      />

      <Cursor ref={cursorRef} />

      <Canvas
        onClick={handleClick}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
        onMouseMove={handleMouseMove}
        width="100%"
        ref={canvasRef}
      />
    </Container>
  );
}
