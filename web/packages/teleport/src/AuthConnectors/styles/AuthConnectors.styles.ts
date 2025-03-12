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

import styled from 'styled-components';

import { Box, Button, Subtitle1 } from 'design';

import { FeatureHeader } from 'teleport/components/Layout';

export const ResponsiveFeatureHeader = styled(FeatureHeader)`
  justify-content: space-between;

  @media screen and (max-width: ${p => p.theme.breakpoints.tablet}px) {
    flex-direction: column;
    height: auto;
    gap: 10px;
    margin: 0 0 10px 0;
    padding: 0 0 10px 0;
    align-items: start;
  }
`;

export const MobileDescription = styled(Subtitle1)`
  margin-bottom: ${p => p.theme.space[3]}px;
  @media screen and (min-width: ${p => p.theme.breakpoints.tablet}px) {
    display: none;
  }
`;

export const DesktopDescription = styled(Box)`
  margin-left: ${p => p.theme.space[4]}px;
  width: 240px;
  color: ${p => p.theme.colors.text.main};
  flex-shrink: 0;
  @media screen and (max-width: ${p => p.theme.breakpoints.tablet}px) {
    display: none;
  }
`;

export const ResponsiveAddButton = styled(Button)`
  width: 240px;
  @media screen and (max-width: ${p => p.theme.breakpoints.tablet}px) {
    width: 100%;
  }
`;
