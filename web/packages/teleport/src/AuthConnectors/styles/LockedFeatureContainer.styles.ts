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

import { Flex } from 'design';

import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';

export const LockedFeatureContainer = styled(Flex)`
  flex-wrap: wrap;
  position: relative;
  justify-content: center;
  min-width: 224px;
`;

export const LockedFeatureButton = styled(ButtonLockedFeature)`
  position: absolute;
  width: 80%;
  right: 10%;
  bottom: -10px;

  @media screen and (max-width: ${props => props.theme.breakpoints.tablet}px) {
    width: 100%;
    right: 1px;
  }
`;
