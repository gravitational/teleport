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

import { CanvasAddon } from '@xterm/addon-canvas';
import { ImageAddon } from '@xterm/addon-image';
import { WebLinksAddon } from '@xterm/addon-web-links';
import { WebglAddon } from '@xterm/addon-webgl';
import { ITerminalAddon, Terminal } from '@xterm/xterm';
import type { DefaultTheme } from 'styled-components';

import { Logger } from 'design/logger';
import { getPlatform, Platform } from 'design/platform';

import { Player } from '../Player';
import { AspectFitAddon } from './AspectFitRatio';
import { EventType, type TerminalSize, type TtyEvent } from './types';

/**
 * TtyPlayer is a player that connects a stream of TtyEvents to an xterm.js terminal.
 *
 * It handles rendering the terminal, applying events, and managing terminal addons.
 *
 * It supports resizing, clearing the terminal, and focusing the terminal on play/seek.
 * It also handles terminal themes and font settings.
 */
export class TtyPlayer extends Player<TtyEvent> {
  private addons: ITerminalAddon[] = [];
  private aspectFitAddon = new AspectFitAddon();
  private terminal: Terminal | null = null;
  private playing = false;
  private logger = new Logger('TtyPlayer');

  constructor(
    private theme: DefaultTheme,
    private size: TerminalSize
  ) {
    super();
  }

  override init(element: HTMLElement) {
    this.terminal = new Terminal({
      fontSize: getPlatform() === Platform.macOS ? 12 : 14,
      fontFamily: this.theme.fonts.mono,
      cols: this.size.cols,
      rows: this.size.rows,
      theme: this.theme.colors.terminal,
    });

    const linksAddon = new WebLinksAddon();
    const imageAddon = new ImageAddon();

    this.addons.push(this.aspectFitAddon, linksAddon, imageAddon);

    this.aspectFitAddon.activate(this.terminal);

    for (const addon of this.addons) {
      this.terminal.loadAddon(addon);
    }

    const createCanvasAddon = () => {
      const canvasAddon = new CanvasAddon();

      this.addons.push(canvasAddon);
      this.terminal.loadAddon(canvasAddon);
    };

    try {
      const webglAddon = new WebglAddon();

      webglAddon.onContextLoss(() => {
        createCanvasAddon();
      });

      this.terminal.loadAddon(webglAddon);
      this.addons.push(webglAddon);
    } catch {
      createCanvasAddon();
    }

    this.terminal.open(element);

    this.aspectFitAddon.fitWithAspectRatio(this.size);

    // Set up mouse event interceptor to scale coordinates
    this.setupMouseEventScaling();
  }

  override applyEvent(event: TtyEvent) {
    if (!this.terminal) {
      throw new Error('Terminal is not initialized');
    }

    switch (event.type) {
      case EventType.Resize:
        this.size = event.terminalSize;

        this.aspectFitAddon.fitWithAspectRatio(this.size);

        break;

      case EventType.SessionPrint:
        this.terminal.write(event.data);

        break;
    }
  }

  override clear() {
    if (!this.terminal) {
      throw new Error('Terminal is not initialized');
    }

    this.terminal.reset();
    this.fit();
  }

  fit() {
    this.aspectFitAddon.fitWithAspectRatio(this.size);

    if (this.playing) {
      this.terminal?.focus();
    }
  }

  override handleEvent(event: TtyEvent) {
    if (!this.terminal) {
      throw new Error('Terminal is not initialized');
    }

    if (event.type === EventType.Screen) {
      this.size.cols = event.screen.cols;
      this.size.rows = event.screen.rows;

      this.clear();

      this.terminal.write(event.screen.data);

      return true;
    }

    return false;
  }

  override destroy() {
    for (const addon of this.addons) {
      try {
        addon.dispose();
      } catch {
        this.logger.warn('Failed to dispose terminal addon', addon);
      }
    }

    this.addons = [];

    if (this.terminal) {
      this.terminal.dispose();
      this.terminal = null;
    }
  }

  onPlay() {
    if (!this.terminal) {
      throw new Error('Terminal is not initialized');
    }

    this.terminal.focus();

    this.playing = true;
  }

  onSeek() {
    if (!this.terminal) {
      throw new Error('Terminal is not initialized');
    }

    this.terminal.focus();
  }

  onPause() {
    this.playing = false;
  }

  onStop() {
    this.playing = false;
  }

  /**
   * The aspect fit addon scales the terminal using CSS transforms. This breaks the text
   * selection and mouse events as the mouse coordinates are not scaled. This method patches
   * xterm's mouse event handling to adjust the mouse coordinates based on the current scale
   * and offsets applied by the aspect fit addon.
   */
  private setupMouseEventScaling() {
    if (!this.terminal) {
      return;
    }

    const terminalElement = this.terminal.element;
    if (!terminalElement) {
      return;
    }

    const core = (this.terminal as any)._core;

    if (!core) {
      return;
    }

    if (core._selectionService) {
      const selectionService = core._selectionService;

      const mouseService = selectionService._mouseService;
      const originalGetCoords = mouseService.getCoords;

      mouseService.getCoords = (
        event: MouseEvent,
        element: HTMLElement,
        colCount: number,
        rowCount: number,
        isSelection?: boolean
      ): [number, number] | undefined => {
        const scale = this.aspectFitAddon.getScale();
        const offsets = this.aspectFitAddon.getOffsets();

        const parentRect =
          terminalElement.parentElement?.getBoundingClientRect();
        if (!parentRect) {
          return originalGetCoords.call(
            mouseService,
            event,
            element,
            colCount,
            rowCount,
            isSelection
          );
        }

        // Calculate mouse position relative to the parent container
        const mouseX = event.clientX - parentRect.left;
        const mouseY = event.clientY - parentRect.top;

        // Adjust for the terminal's offset and scale
        const adjustedX = (mouseX - offsets.x) / scale;
        const adjustedY = (mouseY - offsets.y) / scale;

        // Get cell dimensions
        const cellWidth = core._renderService?.dimensions?.css?.cell?.width;
        const cellHeight = core._renderService?.dimensions?.css?.cell?.height;

        if (!cellWidth || !cellHeight) {
          return originalGetCoords.call(
            mouseService,
            event,
            element,
            colCount,
            rowCount,
            isSelection
          );
        }

        // Calculate column and row (0-based indexing)
        const col = Math.floor(adjustedX / cellWidth) + 1;
        const row = Math.floor(adjustedY / cellHeight) + 1;

        // Clamp to terminal bounds
        const clampedCol = Math.max(0, Math.min(col, colCount - 1));
        const clampedRow = Math.max(0, Math.min(row, rowCount - 1));

        return [clampedCol, clampedRow];
      };
    }
  }
}
