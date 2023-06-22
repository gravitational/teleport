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
    tdpCli,
    tdpCliConnect = false,
    tdpCliScreenSpec,
    tdpCliOnPngFrame,
    tdpCliOnBmpFrame,
    tdpCliOnClipboardData,
    tdpCliOnTdpError,
    tdpCliOnTdpWarning,
    tdpCliOnWsClose,
    tdpCliOnWsOpen,
    tdpCliOnClientScreenSpec,
    onKeyDown,
    onKeyUp,
    onMouseMove,
    onMouseDown,
    onMouseUp,
    onMouseWheelScroll,
    onContextMenu,
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
    if (tdpCli && tdpCliOnPngFrame) {
      const canvas = canvasRef.current;
      const ctx = canvas.getContext('2d');

      // Buffered rendering logic
      var pngBuffer: PngFrame[] = [];
      const renderBuffer = () => {
        if (pngBuffer.length) {
          for (let i = 0; i < pngBuffer.length; i++) {
            tdpCliOnPngFrame(ctx, pngBuffer[i]);
          }
          pngBuffer = [];
        }
        requestAnimationFrame(renderBuffer);
      };
      requestAnimationFrame(renderBuffer);

      const pushToPngBuffer = (pngFrame: PngFrame) => {
        pngBuffer.push(pngFrame);
      };

      tdpCli.on(TdpClientEvent.TDP_PNG_FRAME, pushToPngBuffer);

      return () => {
        tdpCli.removeListener(TdpClientEvent.TDP_PNG_FRAME, pushToPngBuffer);
      };
    }
  }, [tdpCli, tdpCliOnPngFrame]);

  useEffect(() => {
    if (tdpCli && tdpCliOnBmpFrame) {
      const canvas = canvasRef.current;
      const ctx = canvas.getContext('2d');

      // Buffered rendering logic
      var bitmapBuffer: BitmapFrame[] = [];
      const renderBuffer = () => {
        if (bitmapBuffer.length) {
          for (let i = 0; i < bitmapBuffer.length; i++) {
            tdpCliOnBmpFrame(ctx, bitmapBuffer[i]);
          }
          bitmapBuffer = [];
        }
        requestAnimationFrame(renderBuffer);
      };
      requestAnimationFrame(renderBuffer);

      const pushToBitmapBuffer = (bmpFrame: BitmapFrame) => {
        bitmapBuffer.push(bmpFrame);
      };

      tdpCli.on(TdpClientEvent.TDP_BMP_FRAME, pushToBitmapBuffer);

      return () => {
        tdpCli.removeListener(TdpClientEvent.TDP_BMP_FRAME, pushToBitmapBuffer);
      };
    }
  }, [tdpCli, tdpCliOnBmpFrame]);

  useEffect(() => {
    if (tdpCli && tdpCliOnClientScreenSpec) {
      const canvas = canvasRef.current;
      const _tdpCliOnClientScreenSpec = (spec: ClientScreenSpec) => {
        tdpCliOnClientScreenSpec(tdpCli, canvas, spec);
      };
      tdpCli.on(
        TdpClientEvent.TDP_CLIENT_SCREEN_SPEC,
        _tdpCliOnClientScreenSpec
      );

      return () => {
        tdpCli.removeListener(
          TdpClientEvent.TDP_CLIENT_SCREEN_SPEC,
          _tdpCliOnClientScreenSpec
        );
      };
    }
  }, [tdpCli, tdpCliOnClientScreenSpec]);

  useEffect(() => {
    if (tdpCli && tdpCliOnClipboardData) {
      tdpCli.on(TdpClientEvent.TDP_CLIPBOARD_DATA, tdpCliOnClipboardData);

      return () => {
        tdpCli.removeListener(
          TdpClientEvent.TDP_CLIPBOARD_DATA,
          tdpCliOnClipboardData
        );
      };
    }
  }, [tdpCli, tdpCliOnClipboardData]);

  useEffect(() => {
    if (tdpCli && tdpCliOnTdpError) {
      tdpCli.on(TdpClientEvent.TDP_ERROR, tdpCliOnTdpError);
      tdpCli.on(TdpClientEvent.CLIENT_ERROR, tdpCliOnTdpError);

      return () => {
        tdpCli.removeListener(TdpClientEvent.TDP_ERROR, tdpCliOnTdpError);
        tdpCli.removeListener(TdpClientEvent.CLIENT_ERROR, tdpCliOnTdpError);
      };
    }
  }, [tdpCli, tdpCliOnTdpError]);

  useEffect(() => {
    if (tdpCli && tdpCliOnTdpWarning) {
      tdpCli.on(TdpClientEvent.TDP_WARNING, tdpCliOnTdpWarning);
      tdpCli.on(TdpClientEvent.CLIENT_WARNING, tdpCliOnTdpWarning);

      return () => {
        tdpCli.removeListener(TdpClientEvent.TDP_WARNING, tdpCliOnTdpWarning);
        tdpCli.removeListener(
          TdpClientEvent.CLIENT_WARNING,
          tdpCliOnTdpWarning
        );
      };
    }
  }, [tdpCli, tdpCliOnTdpWarning]);

  useEffect(() => {
    if (tdpCli && tdpCliOnWsClose) {
      tdpCli.on(TdpClientEvent.WS_CLOSE, tdpCliOnWsClose);

      return () => {
        tdpCli.removeListener(TdpClientEvent.WS_CLOSE, tdpCliOnWsClose);
      };
    }
  }, [tdpCli, tdpCliOnWsClose]);

  useEffect(() => {
    if (tdpCli && tdpCliOnWsOpen) {
      tdpCli.on(TdpClientEvent.WS_OPEN, tdpCliOnWsOpen);

      return () => {
        tdpCli.removeListener(TdpClientEvent.WS_OPEN, tdpCliOnWsOpen);
      };
    }
  }, [tdpCli, tdpCliOnWsOpen]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const _oncontextmenu = onContextMenu;
    if (onContextMenu) {
      canvas.oncontextmenu = _oncontextmenu;
    }

    return () => {
      if (onContextMenu)
        canvas.removeEventListener('contextmenu', _oncontextmenu);
    };
  }, [onContextMenu]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const _onmousemove = (e: MouseEvent) => {
      onMouseMove(tdpCli, canvas, e);
    };
    if (onMouseMove) {
      canvas.onmousemove = _onmousemove;
    }

    return () => {
      if (onMouseMove) canvas.removeEventListener('mousemove', _onmousemove);
    };
  }, [tdpCli, onMouseMove]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const _onmousedown = (e: MouseEvent) => {
      onMouseDown(tdpCli, e);
    };
    if (onMouseDown) {
      canvas.onmousedown = _onmousedown;
    }

    return () => {
      if (onMouseDown) canvas.removeEventListener('mousedown', _onmousedown);
    };
  }, [tdpCli, onMouseDown]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const _onmouseup = (e: MouseEvent) => {
      onMouseUp(tdpCli, e);
    };
    if (onMouseUp) {
      canvas.onmouseup = _onmouseup;
    }

    return () => {
      if (onMouseUp) canvas.removeEventListener('mouseup', _onmouseup);
    };
  }, [tdpCli, onMouseUp]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const _onwheel = (e: WheelEvent) => {
      onMouseWheelScroll(tdpCli, e);
    };
    if (onMouseWheelScroll) {
      canvas.onwheel = _onwheel;
    }

    return () => {
      if (onMouseWheelScroll) canvas.removeEventListener('wheel', _onwheel);
    };
  }, [tdpCli, onMouseWheelScroll]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const _onkeydown = (e: KeyboardEvent) => {
      onKeyDown(tdpCli, e);
    };
    if (onKeyDown) {
      canvas.onkeydown = _onkeydown;
    }

    return () => {
      if (onKeyDown) canvas.removeEventListener('keydown', _onkeydown);
    };
  }, [tdpCli, onKeyDown]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const _onkeyup = (e: KeyboardEvent) => {
      onKeyUp(tdpCli, e);
    };
    if (onKeyUp) {
      canvas.onkeyup = _onkeyup;
    }

    return () => {
      if (onKeyUp) canvas.removeEventListener('keyup', _onkeyup);
    };
  }, [tdpCli, onKeyUp]);

  // Call init after all listeners have been registered
  useEffect(() => {
    if (tdpCli && tdpCliConnect) {
      tdpCli.connect(tdpCliScreenSpec);
      return () => {
        tdpCli.shutdown();
      };
    }
  }, [tdpCli, tdpCliConnect]);

  return <canvas style={{ ...style }} ref={canvasRef} />;
}

export type Props = {
  tdpCli: TdpClient;
  // tdpCliConnect determines whether the TdpClientCanvas
  // will try to connect to the server.
  tdpCliConnect?: boolean;
  // tdpCliScreenSpec will be passed to tdpCli.connect() if
  // tdpCliConnect is true.
  tdpCliScreenSpec?: ClientScreenSpec;
  tdpCliOnPngFrame?: (
    ctx: CanvasRenderingContext2D,
    pngFrame: PngFrame
  ) => void;
  tdpCliOnBmpFrame?: (
    ctx: CanvasRenderingContext2D,
    pngFrame: BitmapFrame
  ) => void;
  tdpCliOnClipboardData?: (clipboardData: ClipboardData) => void;
  tdpCliOnTdpError?: (error: Error) => void;
  tdpCliOnTdpWarning?: (warning: string) => void;
  tdpCliOnWsClose?: () => void;
  tdpCliOnWsOpen?: () => void;
  tdpCliOnClientScreenSpec?: (
    cli: TdpClient,
    canvas: HTMLCanvasElement,
    spec: ClientScreenSpec
  ) => void;
  onKeyDown?: (cli: TdpClient, e: KeyboardEvent) => void;
  onKeyUp?: (cli: TdpClient, e: KeyboardEvent) => void;
  onMouseMove?: (
    cli: TdpClient,
    canvas: HTMLCanvasElement,
    e: MouseEvent
  ) => void;
  onMouseDown?: (cli: TdpClient, e: MouseEvent) => void;
  onMouseUp?: (cli: TdpClient, e: MouseEvent) => void;
  onMouseWheelScroll?: (cli: TdpClient, e: WheelEvent) => void;
  onContextMenu?: () => boolean;
  style?: CSSProperties;
};
