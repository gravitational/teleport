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

import React from 'react';
import { useTheme } from 'styled-components';
import Image from 'design/Image';

import LogoHeroLight from './LogoHeroLight.svg';
import LogoHeroDark from './LogoHeroDark.svg';

const LogoHero = ({ ...rest }) => {
  const theme = useTheme();
  const src = theme.type === 'light' ? LogoHeroLight : LogoHeroDark;
  return <Image {...rest} src={src} />;
};

LogoHero.defaultProps = {
  src: LogoHeroDark,
  maxHeight: '120px',
  maxWidth: '200px',
  my: '48px',
  mx: 'auto',
};

export default LogoHero;
