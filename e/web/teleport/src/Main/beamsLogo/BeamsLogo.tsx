// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

import { css, useTheme } from 'styled-components';

import dark from 'design/assets/images/beams-dark.svg';
import light from 'design/assets/images/beams-light.svg';
import Image from 'design/Image/Image';
import { Theme } from 'design/theme';

const image: {
  [K in Theme['type']]: string;
} = {
  light,
  dark,
};

export function BeamsLogo() {
  const theme = useTheme();
  return (
    <Image
      src={image[theme.type]}
      width="fit-content"
      alt="beams logo"
      css={css`
        height: 36px;
        padding-left: ${props => props.theme.space[3]}px;
        padding-right: ${props => props.theme.space[3]}px;
        @media screen and (min-width: ${p => p.theme.breakpoints.small}) {
          height: 42px;
          padding-left: ${props => props.theme.space[4]}px;
          padding-right: ${props => props.theme.space[4]}px;
        }
      `}
    />
  );
}
