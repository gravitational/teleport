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
import React, { useEffect, useRef, CSSProperties } from 'react';
import { TdpClient, TdpClientEvent } from 'teleport/lib/tdp';
import {
  PngFrame,
  ClientScreenSpec,
  ClipboardData,
} from 'teleport/lib/tdp/codec';

export default function TdpClientCanvas(props: Props) {
  const {
    tdpCli,
    tdpCliOnPngFrame,
    tdpCliOnClipboardData,
    tdpCliOnTdpError,
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
    onMouseEnter,
    windowOnFocus,
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
    if (tdpCli) {
      tdpCli.init();
      return () => {
        tdpCli.nuke();
      };
    }
  }, [tdpCli]);

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

      return () => {
        tdpCli.removeListener(TdpClientEvent.TDP_ERROR, tdpCliOnTdpError);
      };
    }
  }, [tdpCli, tdpCliOnTdpError]);

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
  }, [onMouseMove]);

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
  }, [onMouseDown]);

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
  }, [onMouseUp]);

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
  }, [onMouseWheelScroll]);

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
  }, [onKeyDown]);

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
  }, [onKeyUp]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const _onmouseenter = (e: MouseEvent) => {
      onMouseEnter(tdpCli, e);
    };
    if (onMouseEnter) {
      canvas.onmouseenter = _onmouseenter;
    }

    return () => {
      if (onMouseEnter) canvas.removeEventListener('mouseenter', _onmouseenter);
    };
  }, [onMouseEnter]);

  useEffect(() => {
    const _windowonfocus = (e: FocusEvent) => {
      // Checking for (canvasRef.current.style.display !== 'none') ensures windowOnFocus behaves
      // like the other passed event listeners, namely it isn't called if the TdpClientCanvas isn't displayed.
      if (canvasRef.current.style.display !== 'none') windowOnFocus(tdpCli, e);
    };
    if (windowOnFocus) {
      window.onfocus = _windowonfocus;
    }

    return () => {
      if (windowOnFocus) window.removeEventListener('focus', _windowonfocus);
    };
  }, [windowOnFocus]);

  return <canvas style={{ ...style }} ref={canvasRef} />;
}

export type Props = {
  tdpCli?: TdpClient;
  tdpCliOnPngFrame?: (
    ctx: CanvasRenderingContext2D,
    pngFrame: PngFrame
  ) => void;
  tdpCliOnClipboardData?: (clipboardData: ClipboardData) => void;
  tdpCliOnTdpError?: (err: Error) => void;
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
  onMouseEnter?: (cli: TdpClient, e: MouseEvent) => void;
  windowOnFocus?: (cli: TdpClient, e: FocusEvent) => void;
  style?: CSSProperties;
};
