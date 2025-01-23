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

import { Image } from 'design';

import pam from './pam.svg';

export function PamIcon() {
  return (
    <PamCircle>
      <Image src={pam} width="14px" />
    </PamCircle>
  );
}

const PamCircle = styled.div`
  height: 30px;
  width: 30px;
  display: flex;
  align-content: center;
  justify-content: center;
  border-radius: 50%;
  background: ${props => props.theme.colors.spotBackground[0]};
`;
