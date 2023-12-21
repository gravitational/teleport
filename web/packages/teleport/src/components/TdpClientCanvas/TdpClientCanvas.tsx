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

import React, { useEffect, useRef } from 'react';

import { TdpClientEvent } from 'teleport/lib/tdp';

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
    tdpCliInit = false,
    tdpCliOnPngFrame,
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
      var buffer: PngFrame[] = [];
      const renderBuffer = () => {
        if (buffer.length) {
          for (let i = 0; i < buffer.length; i++) {
            tdpCliOnPngFrame(ctx, buffer[i]);
          }
          buffer = [];
        }
        requestAnimationFrame(renderBuffer);
      };
      requestAnimationFrame(renderBuffer);

      const pushToBuffer = (pngFrame: PngFrame) => {
        buffer.push(pngFrame);
      };

      tdpCli.on(TdpClientEvent.TDP_PNG_FRAME, pushToBuffer);

      return () => {
        tdpCli.removeListener(TdpClientEvent.TDP_PNG_FRAME, pushToBuffer);
      };
    }
  }, [tdpCli, tdpCliOnPngFrame]);

  useEffect(() => {
    if (tdpCli && tdpCliOnClientScreenSpec) {
      const canvas = canvasRef.current;
      const _tdpCliOnClientScreenSpec = (spec: ClientScreenSpec) => {
        tdpCliOnClientScreenSpec(canvas, spec);
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
    if (tdpCli && tdpCliInit) {
      tdpCli.init();
      return () => {
        tdpCli.shutdown();
      };
    }
  }, [tdpCli, tdpCliInit]);

  return <canvas style={{ ...style }} ref={canvasRef} />;
}

export type Props = {
  tdpCli?: TdpClient;
  tdpCliInit?: boolean;
  tdpCliOnPngFrame?: (
    ctx: CanvasRenderingContext2D,
    pngFrame: PngFrame
  ) => void;
  tdpCliOnClipboardData?: (clipboardData: ClipboardData) => void;
  tdpCliOnTdpError?: (error: Error) => void;
  tdpCliOnTdpWarning?: (warning: string) => void;
  tdpCliOnWsClose?: () => void;
  tdpCliOnWsOpen?: () => void;
  tdpCliOnClientScreenSpec?: (
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
