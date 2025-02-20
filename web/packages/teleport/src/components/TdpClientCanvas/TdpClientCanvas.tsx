/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React, { memo, useEffect, useRef, type CSSProperties } from 'react';

import { debounce } from 'shared/utils/highbar';

import { TdpClient, TdpClientEvent } from 'teleport/lib/tdp';
import { BitmapFrame } from 'teleport/lib/tdp/client';
import type { ClientScreenSpec, PngFrame } from 'teleport/lib/tdp/codec';

function TdpClientCanvas(props: Props) {
  const {
    client,
    clientOnPngFrame,
    clientOnBmpFrame,
    clientOnClientScreenSpec,
    onKeyDown,
    onKeyUp,
    onBlur,
    onMouseMove,
    onMouseDown,
    onMouseUp,
    onMouseWheel,
    onContextMenu,
    onResize,
    style,
    updatePointer,
  } = props;
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    // Empty dependency array ensures this runs only once after initial render.
    // This code will run after the component has been mounted and the canvasRef has been assigned.
    const canvas = canvasRef.current;
    if (canvas) {
      // Make the canvas a focusable keyboard listener
      // https://stackoverflow.com/a/51267699/6277051
      // https://stackoverflow.com/a/16492878/6277051
      canvas.tabIndex = -1;
      canvas.style.outline = 'none';
      canvas.focus();
    }
  }, []);

  useEffect(() => {
    if (client && clientOnPngFrame) {
      const canvas = canvasRef.current;
      const ctx = canvas.getContext('2d');

      // Buffered rendering logic
      var pngBuffer: PngFrame[] = [];
      const renderBuffer = () => {
        if (pngBuffer.length) {
          for (let i = 0; i < pngBuffer.length; i++) {
            clientOnPngFrame(ctx, pngBuffer[i]);
          }
          pngBuffer = [];
        }
        requestAnimationFrame(renderBuffer);
      };
      requestAnimationFrame(renderBuffer);

      const pushToPngBuffer = (pngFrame: PngFrame) => {
        pngBuffer.push(pngFrame);
      };

      client.on(TdpClientEvent.TDP_PNG_FRAME, pushToPngBuffer);

      return () => {
        client.removeListener(TdpClientEvent.TDP_PNG_FRAME, pushToPngBuffer);
      };
    }
  }, [client, clientOnPngFrame]);

  useEffect(() => {
    if (client && updatePointer) {
      const canvas = canvasRef.current;
      const updatePointer = (pointer: {
        data: ImageData | boolean;
        hotspot_x?: number;
        hotspot_y?: number;
      }) => {
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
          let scale = Math.min(32 / cursor.width, 32 / cursor.height);
          resized.width = cursor.width * scale;
          resized.height = cursor.height * scale;

          let context = resized.getContext('2d', {
            colorSpace: pointer.data.colorSpace,
          });
          context.scale(scale, scale);
          context.drawImage(cursor, 0, 0);
          cursor = resized;
        }
        canvas.style.cursor = `url(${cursor.toDataURL()}) ${
          pointer.hotspot_x
        } ${pointer.hotspot_y}, auto`;
      };

      client.addListener(TdpClientEvent.POINTER, updatePointer);

      return () => {
        client.removeListener(TdpClientEvent.POINTER, updatePointer);
      };
    }
  }, [client, updatePointer]);

  useEffect(() => {
    if (client && clientOnBmpFrame) {
      const canvas = canvasRef.current;
      const ctx = canvas.getContext('2d');

      // Buffered rendering logic
      var bitmapBuffer: BitmapFrame[] = [];
      const renderBuffer = () => {
        if (bitmapBuffer.length) {
          for (let i = 0; i < bitmapBuffer.length; i++) {
            if (bitmapBuffer[i].image_data.data.length != 0) {
              clientOnBmpFrame(ctx, bitmapBuffer[i]);
            }
          }
          bitmapBuffer = [];
        }
        requestAnimationFrame(renderBuffer);
      };
      requestAnimationFrame(renderBuffer);

      const pushToBitmapBuffer = (bmpFrame: BitmapFrame) => {
        bitmapBuffer.push(bmpFrame);
      };

      client.on(TdpClientEvent.TDP_BMP_FRAME, pushToBitmapBuffer);

      return () => {
        client.removeListener(TdpClientEvent.TDP_BMP_FRAME, pushToBitmapBuffer);
      };
    }
  }, [client, clientOnBmpFrame]);

  useEffect(() => {
    if (client && clientOnClientScreenSpec) {
      const canvas = canvasRef.current;
      const _clientOnClientScreenSpec = (spec: ClientScreenSpec) => {
        clientOnClientScreenSpec(client, canvas, spec);
      };
      client.on(
        TdpClientEvent.TDP_CLIENT_SCREEN_SPEC,
        _clientOnClientScreenSpec
      );

      return () => {
        client.removeListener(
          TdpClientEvent.TDP_CLIENT_SCREEN_SPEC,
          _clientOnClientScreenSpec
        );
      };
    }
  }, [client, clientOnClientScreenSpec]);

  useEffect(() => {
    if (!onResize) {
      return;
    }

    const debouncedOnResize = debounce(onResize, 250, { trailing: true });
    const observer = new ResizeObserver(([entry]) => {
      if (entry) {
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

  useEffect(() => {
    if (client) {
      const canvas = canvasRef.current;
      const _clearCanvas = () => {
        const ctx = canvas.getContext('2d');
        ctx.clearRect(0, 0, canvas.width, canvas.height);
      };
      client.on(TdpClientEvent.RESET, _clearCanvas);

      return () => {
        client.removeListener(TdpClientEvent.RESET, _clearCanvas);
      };
    }
  }, [client]);

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
      onKeyDown={onKeyDown}
      onKeyUp={onKeyUp}
      onMouseDown={onMouseDown}
      onMouseUp={onMouseUp}
      onContextMenu={onContextMenu}
      onBlur={onBlur}
      onMouseMove={onMouseMove}
      style={{ ...style }}
      ref={canvasRef}
    />
  );
}

export type Props = {
  client: TdpClient;
  clientOnPngFrame?: (
    ctx: CanvasRenderingContext2D,
    pngFrame: PngFrame
  ) => void;
  clientOnBmpFrame?: (
    ctx: CanvasRenderingContext2D,
    pngFrame: BitmapFrame
  ) => void;
  clientOnClientScreenSpec?: (
    cli: TdpClient,
    canvas: HTMLCanvasElement,
    spec: ClientScreenSpec
  ) => void;
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
  style?: CSSProperties;
  updatePointer?: boolean;
};

export default memo(TdpClientCanvas);
