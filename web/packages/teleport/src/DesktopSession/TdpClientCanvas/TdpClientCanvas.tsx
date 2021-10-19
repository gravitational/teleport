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
import { ImageData } from 'teleport/lib/tdp/client';
import useTdpClientCanvas from './useTdpClientCanvas';

export default function TdpClientCanvas(props: Props) {
  const {
    tdpClient,
    onInit,
    onConnect,
    onRender,
    onDisconnect,
    onError,
    onKeyDown,
    onKeyUp,
    onMouseMove,
    onMouseDown,
    onMouseUp,
    style,
  } = props;

  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    // Make the canvas a focusable keyboard listener
    // https://stackoverflow.com/a/51267699/6277051
    // https://stackoverflow.com/a/16492878/6277051
    canvas.tabIndex = -1;
    canvas.style.outline = 'none';
    canvas.focus();

    const ctx = canvas.getContext('2d');

    // Buffered rendering logic
    var buffer: ImageData[] = [];
    const renderBuffer = () => {
      if (buffer.length) {
        for (let i = 0; i < buffer.length; i++) {
          onRender(ctx, buffer[i]);
        }
        buffer = [];
      }
      requestAnimationFrame(renderBuffer);
    };
    requestAnimationFrame(renderBuffer);

    // Initialize canvas, document, and window event listeners.

    // Prevent native context menu to not obscure remote context menu.
    const oncontextmenu = () => false;
    canvas.oncontextmenu = oncontextmenu;

    // Mouse controls.
    const onmousemove = (e: MouseEvent) => {
      onMouseMove(tdpClient, canvas, e);
    };
    canvas.onmousemove = onmousemove;
    const onmousedown = (e: MouseEvent) => {
      onMouseDown(tdpClient, e);
    };
    canvas.onmousedown = onmousedown;
    const onmouseup = (e: MouseEvent) => {
      onMouseUp(tdpClient, e);
    };
    canvas.onmouseup = onmouseup;

    // Key controls.
    const onkeydown = (e: KeyboardEvent) => {
      onKeyDown(tdpClient, e);
    };
    canvas.onkeydown = onkeydown;
    const onkeyup = (e: KeyboardEvent) => {
      onKeyUp(tdpClient, e);
    };
    canvas.onkeyup = onkeyup;

    // Initialize tdpClient event listeners.
    tdpClient.on('init', () => {
      onInit(tdpClient, canvas);
    });

    tdpClient.on('connect', () => {
      onConnect();
    });

    tdpClient.on('render', (data: ImageData) => {
      buffer.push(data);
    });

    tdpClient.on('disconnect', () => {
      onDisconnect();
    });

    tdpClient.on('error', (err: Error) => {
      onError(err);
    });

    tdpClient.init();

    return () => {
      tdpClient.nuke();
      canvas.removeEventListener('contextmenu', oncontextmenu);
      canvas.removeEventListener('mousemove', onmousemove);
      canvas.removeEventListener('mousedown', onmousedown);
      canvas.removeEventListener('mouseup', onmouseup);
      canvas.removeEventListener('keydown', onkeydown);
      canvas.removeEventListener('keyup', onkeyup);
    };
  }, [tdpClient]);

  return <canvas style={{ ...style }} ref={canvasRef} />;
}

export type Props = ReturnType<typeof useTdpClientCanvas> & {
  style?: CSSProperties;
};
