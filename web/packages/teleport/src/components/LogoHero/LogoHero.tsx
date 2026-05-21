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

import BeamsLogoDark from 'design/assets/images/beams-dark.svg';
import BeamsLogoLight from 'design/assets/images/beams-light.svg';
import Image from 'design/Image';

import cfg from 'teleport/config';

// The edition logo SVG served at this path is selected at build time to match
// the edition of the binary. See the Makefile and the per-edition public/ dirs.
// TODO (avatus): replace the static `v=1` with the Teleport version so the
// URL changes when the binary is upgraded, or just update to v=2 if we ever
// update logos.
export function logoSrc(themeType: 'light' | 'dark'): string {
  const base = import.meta.env.MODE === 'development' ? '/app/' : '/web/app/';
  return `${base}logo-${themeType}.svg?v=1`;
}

// Beams branding is a per-cluster runtime feature flag (cfg.beamsUi), not a
// build-time binary identity, so it can't piggy-back on the build-time public
// path system the way the AGPL/Community/Enterprise logos do. Keep these as
// regular SVG imports and branch on the flag inside the component.
const beamsLogos = {
  light: BeamsLogoLight,
  dark: BeamsLogoDark,
};

export const LogoHero = ({
  my = '48px',
  customSrc,
}: {
  my?: string;
  customSrc?: string;
}) => {
  const theme = useTheme();
  const defaultSrc = cfg.beamsUi
    ? beamsLogos[theme.type]
    : logoSrc(theme.type);
  const src = customSrc || defaultSrc;
  return (
    <Image src={src} maxHeight="120px" maxWidth="200px" my={my} mx="auto" />
  );
};
