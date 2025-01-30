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

import { useRef } from 'react';

import { ClientScreenSpec } from 'teleport/lib/tdp/codec';

declare global {
  interface Navigator {
    userAgentData?: { platform: any };
  }
}

export default function useTdpClientCanvas() {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  /**
   * Synchronize the canvas resolution and display size with the
   * given ClientScreenSpec.
   */
  const syncCanvas = (spec: ClientScreenSpec) => {
    const canvas = canvasRef.current;
    if (!canvas) {
      return;
    }
    const { width, height } = spec;
    canvas.width = width;
    canvas.height = height;
    canvas.style.width = `${width}px`;
    canvas.style.height = `${height}px`;
  };

  return {
    syncCanvas,
    canvasRef,
  };
}
