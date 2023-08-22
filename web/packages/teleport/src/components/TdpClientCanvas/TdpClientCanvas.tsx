/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
import React, { useEffect, useRef } from 'react';

import { TdpClientEvent } from 'teleport/lib/tdp';
import { BitmapFrame } from 'teleport/lib/tdp/client';

import type { CSSProperties } from 'react';
import type {
  PngFrame,
  ClientScreenSpec,
  ClipboardData,
} from 'teleport/lib/tdp/codec';
import type { TdpClient } from 'teleport/lib/tdp';

export default function TdpClientCanvas(props: Props) {
  const {
    client,
    clientShouldConnect = false,
    clientScreenSpec,
    clientOnPngFrame,
    clientOnBmpFrame,
    clientOnClipboardData,
    clientOnTdpError,
    clientOnTdpWarning,
    clientOnWsClose,
    clientOnWsOpen,
    clientOnClientScreenSpec,
    canvasOnKeyDown,
    canvasOnKeyUp,
    canvasOnMouseMove,
    canvasOnMouseDown,
    canvasOnMouseUp,
    canvasOnMouseWheelScroll,
    canvasOnContextMenu,
    style,
  } = props;

  const canvasRef = useRef<HTMLCanvasElement>(null);

  if (canvasRef.current) {
    // Make the canvas a focusable keyboard listener
    // https://stackoverflow.com/a/51267699/6277051
    // https://stackoverflow.com/a/16492878/6277051
    canvasRef.current.tabIndex = -1;
    canvasRef.current.style.outline = 'none';
    canvasRef.current.focus();
  }

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
    if (client && clientOnBmpFrame) {
      const canvas = canvasRef.current;
      const ctx = canvas.getContext('2d');

      // Buffered rendering logic
      var bitmapBuffer: BitmapFrame[] = [];
      const renderBuffer = () => {
        if (bitmapBuffer.length) {
          for (let i = 0; i < bitmapBuffer.length; i++) {
            clientOnBmpFrame(ctx, bitmapBuffer[i]);
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
      if (canvasOnMouseMove)
        canvas.removeEventListener('mousemove', _onmousemove);
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

  // Call init after all listeners have been registered
  useEffect(() => {
    if (client && clientShouldConnect) {
      client.connect(clientScreenSpec);
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
  // clientScreenSpec will be passed to client.connect() if
  // clientShouldConnect is true.
  clientScreenSpec?: ClientScreenSpec;
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
  clientOnWsClose?: () => void;
  clientOnWsOpen?: () => void;
  clientOnClientScreenSpec?: (
    cli: TdpClient,
    canvas: HTMLCanvasElement,
    spec: ClientScreenSpec
  ) => void;
  canvasOnKeyDown?: (cli: TdpClient, e: KeyboardEvent) => void;
  canvasOnKeyUp?: (cli: TdpClient, e: KeyboardEvent) => void;
  canvasOnMouseMove?: (
    cli: TdpClient,
    canvas: HTMLCanvasElement,
    e: MouseEvent
  ) => void;
  canvasOnMouseDown?: (cli: TdpClient, e: MouseEvent) => void;
  canvasOnMouseUp?: (cli: TdpClient, e: MouseEvent) => void;
  canvasOnMouseWheelScroll?: (cli: TdpClient, e: WheelEvent) => void;
  canvasOnContextMenu?: () => boolean;
  style?: CSSProperties;
};
