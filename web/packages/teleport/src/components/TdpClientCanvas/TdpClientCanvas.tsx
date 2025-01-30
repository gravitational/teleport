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

import React, {
  memo,
  useEffect,
  type CSSProperties,
  type MutableRefObject,
} from 'react';

import { DebouncedFunc } from 'shared/utils/highbar';

function TdpClientCanvas(props: Props) {
  const {
    canvasRef,
    onKeyDown,
    onKeyUp,
    onFocusOut,
    onMouseMove,
    onMouseDownDS: onMouseDown,
    onMouseUp,
    onMouseWheelScroll,
    windowOnResize,
    style,
  } = props;

  useEffect(() => {
    const canvas = canvasRef.current;
    if (canvas) {
      // Make the canvas a focusable keyboard listener
      // https://stackoverflow.com/a/51267699/6277051
      // https://stackoverflow.com/a/16492878/6277051
      canvas.tabIndex = -1;
      canvas.style.outline = 'none';
      canvas.focus();
    }
  }, [canvasRef]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) {
      return;
    }

    const _resize = () => windowOnResize();

    window.addEventListener('resize', _resize);
    canvas.addEventListener('mousemove', onMouseMove);
    canvas.oncontextmenu = _contextMenu;
    canvas.addEventListener('mousedown', onMouseDown);
    canvas.addEventListener('focusout', onFocusOut);
    canvas.addEventListener('mouseup', onMouseUp);
    canvas.addEventListener('wheel', onMouseWheelScroll);
    canvas.addEventListener('keydown', onKeyDown);
    canvas.addEventListener('keyup', onKeyUp);
    canvas.addEventListener('focusout', onFocusOut);

    return () => {
      window.removeEventListener('resize', _resize);
      canvas.removeEventListener('mousemove', onMouseMove);
      canvas.removeEventListener('contextmenu', _contextMenu);
      canvas.removeEventListener('focusout', onFocusOut);
      canvas.removeEventListener('mousedown', onMouseDown);
      canvas.removeEventListener('mouseup', onMouseUp);
      canvas.removeEventListener('wheel', onMouseWheelScroll);
      canvas.removeEventListener('keydown', onKeyDown);
      canvas.removeEventListener('keyup', onKeyUp);
      canvas.removeEventListener('focusout', onFocusOut);
    };
  }, [
    canvasRef,
    onMouseMove,
    onKeyDown,
    onMouseDown,
    onMouseUp,
    onMouseWheelScroll,
    onKeyUp,
    onFocusOut,
    windowOnResize,
  ]);

  // useEffect(() => {
  //   if (client) {
  //     const canvas = canvasRef.current;
  //     const _clearCanvas = () => {
  //       const ctx = canvas.getContext('2d');
  //       ctx.clearRect(0, 0, canvas.width, canvas.height);
  //     };
  //     client.on(TdpClientEvent.RESET, _clearCanvas);

  //     return () => {
  //       client.removeListener(TdpClientEvent.RESET, _clearCanvas);
  //     };
  //   }
  // }, [client]);

  return <canvas style={{ ...style }} ref={canvasRef} />;
}

export type Props = {
  canvasRef: MutableRefObject<HTMLCanvasElement>;
  onKeyDown?: (e: KeyboardEvent) => any;
  onKeyUp?: (e: KeyboardEvent) => any;
  onFocusOut?: () => any;
  onMouseMove?: (e: MouseEvent) => any;
  onMouseDownDS?: (e: MouseEvent) => any;
  onMouseUp?: (e: MouseEvent) => any;
  onMouseWheelScroll?: (e: WheelEvent) => any;
  onContextMenu?: () => boolean;
  windowOnResize?: DebouncedFunc<() => void>;
  style?: CSSProperties;
  updatePointer?: boolean;
};

export default memo(TdpClientCanvas);

function _contextMenu() {
  return false;
}
