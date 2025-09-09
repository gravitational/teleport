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

import type { ITerminalAddon, Terminal } from '@xterm/xterm';
import type { IRenderDimensions } from '@xterm/xterm/src/browser/renderer/shared/Types';

import type { TerminalSize } from 'teleport/SessionRecordings/view/player/tty/types';

/**
 * AspectFitAddon is a xterm.js addon that resizes the terminal to fit within its parent element
 * while maintaining the specified aspect ratio defined by cols and rows.
 * It uses the same approach as xterm's fit addon (accessing the _renderService and its dimensions).
 * It applies CSS transforms to scale and center the terminal within its parent element.
 */
export class AspectFitAddon implements ITerminalAddon {
  private terminal: Terminal | undefined;
  private currentScale: number = 1;
  private horizontalOffset: number = 0;
  private verticalOffset: number = 0;

  public activate(terminal: Terminal): void {
    this.terminal = terminal;
  }

  public dispose(): void {}

  public getScale(): number {
    return this.currentScale;
  }

  public getOffsets(): { x: number; y: number } {
    return { x: this.horizontalOffset, y: this.verticalOffset };
  }

  public fitWithAspectRatio({ cols, rows }: TerminalSize): void {
    if (!this.terminal?.element?.parentElement) {
      return;
    }

    // accessing the internals of xterm, this is how the fit addon does it
    const core = (this.terminal as any)._core;
    const dims: IRenderDimensions = core._renderService.dimensions;

    if (dims.css.cell.width === 0 || dims.css.cell.height === 0) {
      return;
    }

    const parentElementStyle = window.getComputedStyle(
      this.terminal.element.parentElement
    );
    const parentElementHeight = parseInt(
      parentElementStyle.getPropertyValue('height')
    );
    const parentElementWidth = Math.max(
      0,
      parseInt(parentElementStyle.getPropertyValue('width'))
    );

    const availableHeight = parentElementHeight;
    const availableWidth = parentElementWidth;

    if (this.terminal.rows !== rows || this.terminal.cols !== cols) {
      core._renderService.clear();
      this.terminal.resize(cols, rows);
    }

    const requiredWidth = cols * dims.css.cell.width;
    const requiredHeight = rows * dims.css.cell.height;

    const scaleX = availableWidth / requiredWidth;
    const scaleY = availableHeight / requiredHeight;
    const scale = Math.min(scaleX, scaleY);

    const scaledWidth = requiredWidth * scale;
    const scaledHeight = requiredHeight * scale;

    const horizontalOffset = (availableWidth - scaledWidth) / 2;
    const verticalOffset = (availableHeight - scaledHeight) / 2;

    // Store the current scale and offsets for mouse event transformation
    this.currentScale = scale;
    this.horizontalOffset = horizontalOffset;
    this.verticalOffset = verticalOffset;

    const terminalElement = this.terminal.element;

    terminalElement.style.width = `${requiredWidth}px`;
    terminalElement.style.height = `${requiredHeight}px`;
    terminalElement.style.position = 'absolute';
    terminalElement.style.left = '0';
    terminalElement.style.top = '0';
    terminalElement.style.transform = `translate(${horizontalOffset}px, ${verticalOffset}px) scale(${scale})`;
    terminalElement.style.transformOrigin = 'top left';
  }
}
