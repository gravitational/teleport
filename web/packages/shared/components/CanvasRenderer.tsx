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

import React, {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useRef,
  type CSSProperties,
} from 'react';

import { Logger } from 'design/logger';
import { BitmapFrame, PngFrame } from 'shared/libs/tdp';
import { debounce } from 'shared/utils/highbar';

const logger = new Logger('CanvasRenderer');

export interface CanvasRendererRef {
  setPointer(pointer: Pointer): void;
  renderPngFrame(frame: PngFrame): void;
  renderBitmapFrame(frame: BitmapFrame): void;
  setResolution(resolution: { width: number; height: number }): void;
  clear(): void;
  focus(): void;
  getSize(): { width: number; height: number };
}

export const CanvasRenderer = forwardRef<
  CanvasRendererRef,
  {
    onKeyDown?(e: React.KeyboardEvent): void;
    onKeyUp?(e: React.KeyboardEvent): void;
    onBlur?(e: React.FocusEvent): void;
    onMouseMove?(e: React.MouseEvent): void;
    onMouseDown?(e: React.MouseEvent): void;
    onMouseUp?(e: React.MouseEvent): void;
    onMouseWheel?(e: WheelEvent): void;
    onContextMenu?(e: React.MouseEvent): void;
    /**
     * Handles canvas resize events.
     *
     * This function is called whenever the canvas is resized,
     * with a debounced delay of 250 ms to optimize performance.
     */
    onResize?(e: { width: number; height: number }): void;
    /** Hides the element without changing the layout of a document. */
    hidden?: boolean;
    style?: CSSProperties;
  }
>((props, ref) => {
  const {
    onKeyDown,
    onKeyUp,
    onBlur,
    onMouseMove,
    onMouseDown,
    onMouseUp,
    onMouseWheel,
    onContextMenu,
    onResize,
  } = props;
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useImperativeHandle(ref, () => {
    const renderPngFrame = makePngFrameRenderer(canvasRef.current);
    const renderBimapFrame = makeBitmapFrameRenderer(canvasRef.current);
    return {
      setPointer: pointer => setPointer(canvasRef.current, pointer),
      renderPngFrame: frame => renderPngFrame(frame),
      renderBitmapFrame: frame => renderBimapFrame(frame),
      setResolution: ({ width, height }) => {
        const canvas = canvasRef.current;
        canvas.width = width;
        canvas.height = height;
        logger.debug(`Canvas resolution set to ${width}x${height}.`);
      },
      clear: () => {
        const canvas = canvasRef.current;
        canvas.getContext('2d').clearRect(0, 0, canvas.width, canvas.height);
      },
      focus: () => canvasRef.current.focus(),
      getSize: () => canvasRef.current.getBoundingClientRect(),
    };
  }, []);

  useEffect(() => {
    if (!onResize) {
      return;
    }

    const debouncedOnResize = debounce(onResize, 250, { trailing: true });
    const observer = new ResizeObserver(([entry]) => {
      if (entry && entry.contentRect.height !== 0) {
        debouncedOnResize({
          height: entry.contentRect.height,
          width: entry.contentRect.width,
        });
      }
    });
    observer.observe(canvasRef.current);

    return () => {
      debouncedOnResize.cancel();
      observer.disconnect();
    };
  }, [onResize]);

  // Wheel events must be registered on a ref because React's onWheel
  // uses a passive listener, so handlers are not able to call of e.preventDefault() on it.
  useEffect(() => {
    if (!onMouseWheel) {
      return;
    }
    const canvas = canvasRef.current;
    canvas.addEventListener('wheel', onMouseWheel);
    return () => canvas.removeEventListener('wheel', onMouseWheel);
  }, [onMouseWheel]);

  return (
    <canvas
      // Make the canvas a focusable keyboard listener
      // https://stackoverflow.com/a/51267699/6277051
      tabIndex={-1}
      onKeyDown={onKeyDown}
      onKeyUp={onKeyUp}
      onMouseDown={onMouseDown}
      onMouseUp={onMouseUp}
      onContextMenu={onContextMenu}
      onBlur={onBlur}
      onMouseMove={onMouseMove}
      style={{
        visibility: props.hidden ? 'hidden' : 'visible',
        outline: 'none',
        height: '100%',
        width: '100%',
        minHeight: 0,
        minWidth: 0,
        // Maintains the original proportions of the canvas content.
        // Ensures the rendered content does not exceed its native resolution.
        // If the resolution is higher than the available window size,
        // the content is automatically scaled down to fit.
        objectFit: 'scale-down',
      }}
      ref={canvasRef}
    />
  );
});

interface Pointer {
  data: ImageData | boolean;
  hotspot_x?: number;
  hotspot_y?: number;
}

function setPointer(canvas: HTMLCanvasElement, pointer: Pointer): void {
  if (typeof pointer.data === 'boolean') {
    canvas.style.cursor = pointer.data ? 'default' : 'none';
    return;
  }
  let cursor = document.createElement('canvas');
  cursor.width = pointer.data.width;
  cursor.height = pointer.data.height;
  cursor
    .getContext('2d', { colorSpace: pointer.data.colorSpace })
    .putImageData(pointer.data, 0, 0);
  if (pointer.data.width > 32 || pointer.data.height > 32) {
    // scale the cursor down to at most 32px - max size fully supported by browsers
    const resized = document.createElement('canvas');
    const scale = Math.min(32 / cursor.width, 32 / cursor.height);
    resized.width = cursor.width * scale;
    resized.height = cursor.height * scale;

    const context = resized.getContext('2d', {
      colorSpace: pointer.data.colorSpace,
    });
    context.scale(scale, scale);
    context.drawImage(cursor, 0, 0);
    cursor = resized;
  }
  canvas.style.cursor = `url(${cursor.toDataURL()}) ${pointer.hotspot_x} ${pointer.hotspot_y}, auto`;
}

//TODO(gzdunek): renderBuffer is called  even when the buffer is empty.
// This causes the function to run in a loop, 60 times per second
// (actually x2 because we have two frame renderers).
// Fix it in the both renderers, check if it improves performance.
function makePngFrameRenderer(
  canvas: HTMLCanvasElement
): (frame: PngFrame) => void {
  const ctx = canvas.getContext('2d');

  // Buffered rendering logic
  let pngBuffer: PngFrame[] = [];

  const renderBuffer = () => {
    if (pngBuffer.length) {
      for (let i = 0; i < pngBuffer.length; i++) {
        const pngFrame = pngBuffer[i];
        ctx.drawImage(pngFrame.data, pngFrame.left, pngFrame.top);
      }
      pngBuffer = [];
    }
    requestAnimationFrame(renderBuffer);
  };
  requestAnimationFrame(renderBuffer);

  return frame => pngBuffer.push(frame);
}

function makeBitmapFrameRenderer(
  canvas: HTMLCanvasElement
): (frame: BitmapFrame) => void {
  const ctx = canvas.getContext('2d');

  // Buffered rendering logic
  let bitmapBuffer: BitmapFrame[] = [];
  const renderBuffer = () => {
    if (bitmapBuffer.length) {
      for (let i = 0; i < bitmapBuffer.length; i++) {
        if (bitmapBuffer[i].image_data.data.length != 0) {
          const bmpFrame = bitmapBuffer[i];
          ctx.putImageData(bmpFrame.image_data, bmpFrame.left, bmpFrame.top);
        }
      }
      bitmapBuffer = [];
    }
    requestAnimationFrame(renderBuffer);
  };
  requestAnimationFrame(renderBuffer);

  return frame => bitmapBuffer.push(frame);
}
