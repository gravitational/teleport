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

import React, { memo, useEffect, useRef } from 'react';
import { DebouncedFunc } from 'shared/utils/highbar';

import { TdpClientEvent, TdpClient } from 'teleport/lib/tdp';
import { BitmapFrame } from 'teleport/lib/tdp/client';

import type { CSSProperties } from 'react';
import type {
  PngFrame,
  ClientScreenSpec,
  ClipboardData,
} from 'teleport/lib/tdp/codec';

function TdpClientCanvas(props: Props) {
  const {
    client,
    clientShouldConnect = false,
    clientScreenSpecToRequest,
    clientOnPngFrame,
    clientOnBmpFrame,
    clientOnClipboardData,
    clientOnTdpError,
    clientOnTdpWarning,
    clientOnTdpInfo,
    clientOnWsClose,
    clientOnWsOpen,
    clientOnClientScreenSpec,
    canvasOnKeyDown,
    canvasOnKeyUp,
    canvasOnFocusOut,
    canvasOnMouseMove,
    canvasOnMouseDown,
    canvasOnMouseUp,
    canvasOnMouseWheelScroll,
    canvasOnContextMenu,
    windowOnResize,
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
    if (client && clientOnClipboardData) {
      client.on(TdpClientEvent.TDP_CLIPBOARD_DATA, clientOnClipboardData);

      return () => {
        client.removeListener(
          TdpClientEvent.TDP_CLIPBOARD_DATA,
          clientOnClipboardData
        );
      };
    }
  }, [client, clientOnClipboardData]);

  useEffect(() => {
    if (client && clientOnTdpError) {
      client.on(TdpClientEvent.TDP_ERROR, clientOnTdpError);
      client.on(TdpClientEvent.CLIENT_ERROR, clientOnTdpError);

      return () => {
        client.removeListener(TdpClientEvent.TDP_ERROR, clientOnTdpError);
        client.removeListener(TdpClientEvent.CLIENT_ERROR, clientOnTdpError);
      };
    }
  }, [client, clientOnTdpError]);

  useEffect(() => {
    if (client && clientOnTdpWarning) {
      client.on(TdpClientEvent.TDP_WARNING, clientOnTdpWarning);
      client.on(TdpClientEvent.CLIENT_WARNING, clientOnTdpWarning);

      return () => {
        client.removeListener(TdpClientEvent.TDP_WARNING, clientOnTdpWarning);
        client.removeListener(
          TdpClientEvent.CLIENT_WARNING,
          clientOnTdpWarning
        );
      };
    }
  }, [client, clientOnTdpWarning]);

  useEffect(() => {
    if (client && clientOnTdpInfo) {
      client.on(TdpClientEvent.TDP_INFO, clientOnTdpInfo);

      return () => {
        client.removeListener(TdpClientEvent.TDP_INFO, clientOnTdpInfo);
      };
    }
  }, [client, clientOnTdpInfo]);

  useEffect(() => {
    if (client && clientOnWsClose) {
      client.on(TdpClientEvent.WS_CLOSE, clientOnWsClose);

      return () => {
        client.removeListener(TdpClientEvent.WS_CLOSE, clientOnWsClose);
      };
    }
  }, [client, clientOnWsClose]);

  useEffect(() => {
    if (client && clientOnWsOpen) {
      client.on(TdpClientEvent.WS_OPEN, clientOnWsOpen);

      return () => {
        client.removeListener(TdpClientEvent.WS_OPEN, clientOnWsOpen);
      };
    }
  }, [client, clientOnWsOpen]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const _oncontextmenu = canvasOnContextMenu;
    if (canvasOnContextMenu) {
      canvas.oncontextmenu = _oncontextmenu;
    }

    return () => {
      if (canvasOnContextMenu)
        canvas.removeEventListener('contextmenu', _oncontextmenu);
    };
  }, [canvasOnContextMenu]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const _onmousemove = (e: MouseEvent) => {
      canvasOnMouseMove(client, canvas, e);
    };
    if (canvasOnMouseMove) {
      canvas.onmousemove = _onmousemove;
    }

    return () => {
      if (canvasOnMouseMove) {
        canvas.removeEventListener('mousemove', _onmousemove);
      }
    };
  }, [client, canvasOnMouseMove]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const _onmousedown = (e: MouseEvent) => {
      canvasOnMouseDown(client, e);
    };
    if (canvasOnMouseDown) {
      canvas.onmousedown = _onmousedown;
    }

    return () => {
      if (canvasOnMouseDown)
        canvas.removeEventListener('mousedown', _onmousedown);
    };
  }, [client, canvasOnMouseDown]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const _onmouseup = (e: MouseEvent) => {
      canvasOnMouseUp(client, e);
    };
    if (canvasOnMouseUp) {
      canvas.onmouseup = _onmouseup;
    }

    return () => {
      if (canvasOnMouseUp) canvas.removeEventListener('mouseup', _onmouseup);
    };
  }, [client, canvasOnMouseUp]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const _onwheel = (e: WheelEvent) => {
      canvasOnMouseWheelScroll(client, e);
    };
    if (canvasOnMouseWheelScroll) {
      canvas.onwheel = _onwheel;
    }

    return () => {
      if (canvasOnMouseWheelScroll)
        canvas.removeEventListener('wheel', _onwheel);
    };
  }, [client, canvasOnMouseWheelScroll]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const _onkeydown = (e: KeyboardEvent) => {
      canvasOnKeyDown(client, e);
    };
    if (canvasOnKeyDown) {
      canvas.onkeydown = _onkeydown;
    }

    return () => {
      if (canvasOnKeyDown) canvas.removeEventListener('keydown', _onkeydown);
    };
  }, [client, canvasOnKeyDown]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const _onkeyup = (e: KeyboardEvent) => {
      canvasOnKeyUp(client, e);
    };
    if (canvasOnKeyUp) {
      canvas.onkeyup = _onkeyup;
    }

    return () => {
      if (canvasOnKeyUp) canvas.removeEventListener('keyup', _onkeyup);
    };
  }, [client, canvasOnKeyUp]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const _onfocusout = () => {
      canvasOnFocusOut(client);
    };
    if (canvasOnFocusOut) {
      canvas.addEventListener('focusout', _onfocusout);
    }

    return () => {
      if (canvasOnFocusOut) canvas.removeEventListener('focusout', _onfocusout);
    };
  }, [client, canvasOnFocusOut]);

  useEffect(() => {
    if (client && windowOnResize) {
      const _onresize = () => windowOnResize(client);
      window.addEventListener('resize', _onresize);
      return () => {
        windowOnResize.cancel();
        window.removeEventListener('resize', _onresize);
      };
    }
  }, [client, windowOnResize]);

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

  // Call connect after all listeners have been registered
  useEffect(() => {
    if (client && clientShouldConnect) {
      client.connect(clientScreenSpecToRequest);
      return () => {
        client.shutdown();
      };
    }
  }, [client, clientShouldConnect]);

  return <canvas style={{ ...style }} ref={canvasRef} />;
}

export type Props = {
  client: TdpClient;
  // clientShouldConnect determines whether the TdpClientCanvas
  // will try to connect to the server.
  clientShouldConnect?: boolean;
  // clientScreenSpecToRequest will be passed to client.connect() if
  // clientShouldConnect is true.
  clientScreenSpecToRequest?: ClientScreenSpec;
  clientOnPngFrame?: (
    ctx: CanvasRenderingContext2D,
    pngFrame: PngFrame
  ) => void;
  clientOnBmpFrame?: (
    ctx: CanvasRenderingContext2D,
    pngFrame: BitmapFrame
  ) => void;
  clientOnClipboardData?: (clipboardData: ClipboardData) => void;
  clientOnTdpError?: (error: Error) => void;
  clientOnTdpWarning?: (warning: string) => void;
  clientOnTdpInfo?: (info: string) => void;
  clientOnWsClose?: (message: string) => void;
  clientOnWsOpen?: () => void;
  clientOnClientScreenSpec?: (
    cli: TdpClient,
    canvas: HTMLCanvasElement,
    spec: ClientScreenSpec
  ) => void;
  canvasOnKeyDown?: (cli: TdpClient, e: KeyboardEvent) => void;
  canvasOnKeyUp?: (cli: TdpClient, e: KeyboardEvent) => void;
  canvasOnFocusOut?: (cli: TdpClient) => void;
  canvasOnMouseMove?: (
    cli: TdpClient,
    canvas: HTMLCanvasElement,
    e: MouseEvent
  ) => void;
  canvasOnMouseDown?: (cli: TdpClient, e: MouseEvent) => void;
  canvasOnMouseUp?: (cli: TdpClient, e: MouseEvent) => void;
  canvasOnMouseWheelScroll?: (cli: TdpClient, e: WheelEvent) => void;
  canvasOnContextMenu?: () => boolean;
  windowOnResize?: DebouncedFunc<(cli: TdpClient) => void>;
  style?: CSSProperties;
  updatePointer?: boolean;
};

export default memo(TdpClientCanvas);
