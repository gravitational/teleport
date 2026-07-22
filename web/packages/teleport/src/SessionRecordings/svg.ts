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

import type { LegacyThemeColors } from '@gravitational/design-system';
import { useMemo } from 'react';

type ResolvedTerminalColors = Record<
  keyof LegacyThemeColors['terminal'],
  string | undefined
>;

export function useThumbnailSvg(svg: string, styles: string) {
  return useMemo(
    () => svgToDataURIBase64(injectSVGStyles(svg, styles)),
    [svg, styles]
  );
}

export function injectSVGStyles(svg: string, styles: string) {
  if (svg.includes('<style>')) {
    return svg.replace(/<style>/, `<style>${styles}\n`);
  }

  const styleTag = `<style>${styles}</style>`;

  return svg.replace(/<svg[^>]*>/, match => `${match}${styleTag}`);
}

export function generateTerminalSVGStyleTag(
  terminal: ResolvedTerminalColors,
  monoFont: string
): string {
  const colorMap = [
    terminal.black,
    terminal.red,
    terminal.green,
    terminal.yellow,
    terminal.blue,
    terminal.magenta,
    terminal.cyan,
    terminal.white,
    terminal.brightBlack,
    terminal.brightRed,
    terminal.brightGreen,
    terminal.brightYellow,
    terminal.brightBlue,
    terminal.brightMagenta,
    terminal.brightCyan,
    terminal.brightWhite,
  ];

  const rules: string[] = [
    '.i { font-style: italic; }',
    '.b { font-weight: bold; }',
    '.u { text-decoration: underline; }',
    `* { font-family: ${monoFont} }`,
    `.terminal { fill: ${terminal.foreground}; }`,
    `.bg-default { fill: ${terminal.background}; }`,
  ];

  for (let i = 0; i < colorMap.length; i++) {
    rules.push(`.fg-${i} { fill: ${colorMap[i]}; }`);
    rules.push(`.bg-${i} { fill: ${colorMap[i]}; }`);
  }

  return rules.join('\n');
}

// svgToDataURIBase64 converts an SVG string to a Base64-encoded data URI, providing
// a way to load SVG images in the code.
export function svgToDataURIBase64(svg: string) {
  const encoded = encodeURIComponent(svg.replace('<?xml version="1.0"?>', ''));

  return `data:image/svg+xml;charset=utf-8,${encoded}`;
}

export function pngToDataURIBase64(png: string) {
  return `data:image/png;base64,${png}`;
}
