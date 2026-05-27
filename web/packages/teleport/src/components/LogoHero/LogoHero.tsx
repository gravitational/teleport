/*
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

import { useTheme } from 'styled-components';

import Image from 'design/Image';

// The logo SVG served at this path is selected at build time to match the
// edition of the binary. See the Makefile and the per-edition public/ dirs.
// TODO (avatus): replace the static `v=1` with the Teleport version so the
// URL changes when the binary is upgraded, or just update to v=2 if we ever
// update logos.
export function logoSrc(themeType: 'light' | 'dark'): string {
  const base = import.meta.env.MODE === 'development' ? '/app/' : '/web/app/';
  return `${base}logo-${themeType}.svg?v=1`;
}

export const LogoHero = ({
  my = '48px',
  maxWidth = '200px',
  height = '',
  customSrc,
}: {
  my?: string;
  height?: string;
  maxWidth?: string;
  customSrc?: string;
}) => {
  const theme = useTheme();
  const src = customSrc || logoSrc(theme.type);
  return (
    <Image
      src={src}
      maxHeight="120px"
      height={height}
      maxWidth={maxWidth}
      my={my}
      mx="auto"
    />
  );
};
